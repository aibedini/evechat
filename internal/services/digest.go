package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/i18n/email"
	"github.com/evechat/evechat/internal/integrations/smtp"
	"github.com/evechat/evechat/internal/repo"
)

// DigestService sends the two scheduled emails that keep a support team honest:
// a nudge for conversations nobody answered and a daily summary. It is invoked
// by `evechat job missed|digest` from systemd timers, never by a request.
type DigestService struct {
	repo    *repo.Repo
	client  *smtp.Client
	baseURL string
	log     zerolog.Logger
}

// NewDigestService wires the service. A nil SMTP client disables delivery while
// still letting the command report counts (useful in cron dry runs).
func NewDigestService(r *repo.Repo, client *smtp.Client, baseURL string, log zerolog.Logger) *DigestService {
	return &DigestService{
		repo:    r,
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
		log:     log,
	}
}

type missedRow struct {
	ConversationID string
	ProjectID      string
	VisitorName    string
	CreatedAt      time.Time
}

// RunMissed emails one reminder per conversation that has waited more than
// `olderThan` without any agent or bot reply. It returns the number of emails
// handed to SMTP.
func (s *DigestService) RunMissed(ctx context.Context, olderThan time.Duration) (int, error) {
	if s.client == nil || !s.client.Enabled() {
		s.log.Info().Msg("digest: smtp disabled, skipping missed-chat emails")
		return 0, nil
	}
	if olderThan <= 0 {
		olderThan = 10 * time.Minute
	}
	cutoff := time.Now().UTC().Add(-olderThan).Format("2006-01-02 15:04:05")
	rows, err := s.repo.DB().R.QueryContext(ctx, `
		SELECT c.id, c.project_id, COALESCE(NULLIF(v.name,''), 'کاربر مهمان'), c.created_at
		FROM conversations c
		JOIN visitors v ON v.id = c.visitor_id
		WHERE c.status = 'pending'
		  AND c.assigned_agent_id IS NULL
		  AND c.created_at < ?
		  AND NOT EXISTS (
		    SELECT 1 FROM messages m
		    WHERE m.conversation_id = c.id AND m.sender_type IN ('agent', 'bot')
		  )
		ORDER BY c.created_at ASC
		LIMIT 100`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("querying missed conversations: %w", err)
	}
	defer rows.Close()

	var pending []missedRow
	for rows.Next() {
		var r missedRow
		var created string
		if err := rows.Scan(&r.ConversationID, &r.ProjectID, &r.VisitorName, &created); err != nil {
			return 0, err
		}
		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	sent := 0
	for _, r := range pending {
		waiting := humanWaiting(time.Since(r.CreatedAt))
		msg := email.MissedConversation(r.VisitorName, waiting, s.baseURL, r.ConversationID)
		if err := s.sendToProject(ctx, r.ProjectID, msg.Subject, msg.HTML, msg.Text); err != nil {
			s.log.Warn().Err(err).Str("conversation", r.ConversationID).Msg("digest: missed email failed")
			continue
		}
		sent++
	}
	return sent, nil
}

type dailyRow struct {
	ProjectID     string
	Conversations int
	Resolved      int
	Messages      int
	CSAT          float64
	Busiest       string
}

// RunDaily emails yesterday's numbers per project to its admins.
func (s *DigestService) RunDaily(ctx context.Context) (int, error) {
	if s.client == nil || !s.client.Enabled() {
		s.log.Info().Msg("digest: smtp disabled, skipping daily digest")
		return 0, nil
	}
	rows, err := s.repo.DB().R.QueryContext(ctx, `
		SELECT c.project_id,
		       COUNT(*) AS conversations,
		       SUM(CASE WHEN c.status = 'resolved' THEN 1 ELSE 0 END) AS resolved,
		       (SELECT COUNT(*) FROM messages m JOIN conversations c2 ON c2.id = m.conversation_id
		         WHERE c2.project_id = c.project_id AND m.created_at >= datetime('now','-1 day')) AS messages,
		       COALESCE((SELECT AVG(cs.score) FROM csat_responses cs JOIN conversations c3 ON c3.id = cs.conversation_id
		         WHERE c3.project_id = c.project_id AND cs.created_at >= datetime('now','-1 day')), 0) AS csat,
		       COALESCE((SELECT h.name FROM (
		           SELECT c4.assigned_agent_id AS id, COUNT(*) AS n FROM conversations c4
		           WHERE c4.project_id = c.project_id AND c4.created_at >= datetime('now','-1 day')
		             AND c4.assigned_agent_id IS NOT NULL
		           GROUP BY c4.assigned_agent_id ORDER BY n DESC LIMIT 1) t
		         JOIN agents h ON h.id = t.id), '') AS busiest
		FROM conversations c
		WHERE c.created_at >= datetime('now','-1 day')
		GROUP BY c.project_id`)
	if err != nil {
		return 0, fmt.Errorf("aggregating daily digest: %w", err)
	}
	defer rows.Close()

	date := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	sent := 0
	for rows.Next() {
		var d dailyRow
		if err := rows.Scan(&d.ProjectID, &d.Conversations, &d.Resolved, &d.Messages, &d.CSAT, &d.Busiest); err != nil {
			return 0, err
		}
		msg := email.DailyDigest(date, d.Conversations, d.Resolved, d.Messages, d.CSAT, d.Busiest)
		if err := s.sendToProject(ctx, d.ProjectID, msg.Subject, msg.HTML, msg.Text); err != nil {
			s.log.Warn().Err(err).Str("project", d.ProjectID).Msg("digest: daily email failed")
			continue
		}
		sent++
	}
	return sent, rows.Err()
}

// sendToProject resolves the notification inbox: the project setting
// `notification_email`, then `alert_email`, then active admins/superadmins.
func (s *DigestService) sendToProject(ctx context.Context, projectID, subject, htmlBody, textBody string) error {
	recipients := s.recipients(ctx, projectID)
	if len(recipients) == 0 {
		return fmt.Errorf("no recipient configured for project %s", projectID)
	}
	var firstErr error
	for _, to := range recipients {
		if err := s.client.Send(ctx, to, subject, htmlBody, textBody); err != nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *DigestService) recipients(ctx context.Context, projectID string) []string {
	for _, key := range []string{"notification_email", "alert_email", "support_email"} {
		if v, ok, err := s.repo.Settings.Get(ctx, projectID+":"+key); err == nil && ok && strings.Contains(v, "@") {
			return splitEmails(v)
		}
		if v, ok, err := s.repo.Settings.Get(ctx, key); err == nil && ok && strings.Contains(v, "@") {
			return splitEmails(v)
		}
	}
	rows, err := s.repo.DB().R.QueryContext(ctx, `
		SELECT email FROM agents
		WHERE project_id = ? AND role IN ('admin','superadmin') AND is_active = 1
		ORDER BY created_at ASC LIMIT 5`, projectID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var e string
		if rows.Scan(&e) == nil && e != "" {
			out = append(out, e)
		}
	}
	return out
}

func splitEmails(raw string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		if part = strings.TrimSpace(part); strings.Contains(part, "@") {
			out = append(out, part)
		}
	}
	return out
}

func humanWaiting(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "چند لحظه"
	case d < time.Hour:
		return fmt.Sprintf("%d دقیقه", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d ساعت", int(d.Hours()))
	default:
		return fmt.Sprintf("%d روز", int(d.Hours()/24))
	}
}

// ensure the sql import is used even if queries change to prepared statements.
var _ = sql.ErrNoRows
