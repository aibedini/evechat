package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// StatsRepo computes aggregates for the dashboard.
type StatsRepo struct{ db *db.DB }

// OverviewStats is the KPI block of the dashboard.
type OverviewStats struct {
	TotalConversations      int     `json:"total_conversations"`
	Pending                 int     `json:"pending"`
	Active                  int     `json:"active"`
	Resolved                int     `json:"resolved"`
	Closed                  int     `json:"closed"`
	AvgFirstResponseSeconds float64 `json:"avg_first_response_seconds"`
	AvgResolutionMinutes    float64 `json:"avg_resolution_minutes"`
	CSATAvg                 float64 `json:"csat_avg"`
	CSATCount               int     `json:"csat_count"`
	Messages                int     `json:"messages"`
	AgentMessages           int     `json:"agent_messages"`
	UniqueVisitors          int     `json:"unique_visitors"`
	TicketsOpen             int     `json:"tickets_open"`
}

// SeriesPoint is one bucket of a time series.
type SeriesPoint struct {
	Bucket        string `json:"bucket"`
	Conversations int    `json:"conversations"`
	Messages      int    `json:"messages"`
	Resolved      int    `json:"resolved"`
}

// AgentStat is one row of the agent leaderboard.
type AgentStat struct {
	AgentID                 string  `json:"agent_id"`
	Name                    string  `json:"name"`
	Conversations           int     `json:"conversations"`
	Resolved                int     `json:"resolved"`
	AvgFirstResponseSeconds float64 `json:"avg_first_response_seconds"`
	CSATAvg                 float64 `json:"csat_avg"`
}

// Overview computes KPIs in the [from, to] window.
func (r *StatsRepo) Overview(ctx context.Context, projectID string, from, to time.Time) (*OverviewStats, error) {
	s := &OverviewStats{}
	err := r.db.R.QueryRowContext(ctx, `SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status = 'closed' THEN 1 ELSE 0 END), 0),
		COALESCE(AVG(CASE WHEN first_response_at IS NOT NULL THEN (julianday(first_response_at) - julianday(created_at)) * 86400 END), 0),
		COALESCE(AVG(CASE WHEN resolved_at IS NOT NULL THEN (julianday(resolved_at) - julianday(created_at)) * 1440 END), 0),
		COALESCE(AVG(csat_score), 0),
		COALESCE(COUNT(csat_score), 0),
		COALESCE(COUNT(DISTINCT visitor_id), 0)
		FROM conversations WHERE project_id = ? AND created_at >= ? AND created_at <= ?`,
		projectID, models.FormatTime(from), models.FormatTime(to)).
		Scan(&s.TotalConversations, &s.Pending, &s.Active, &s.Resolved, &s.Closed,
			&s.AvgFirstResponseSeconds, &s.AvgResolutionMinutes, &s.CSATAvg, &s.CSATCount, &s.UniqueVisitors)
	if err != nil {
		return nil, fmt.Errorf("computing overview: %w", err)
	}
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.project_id = ? AND m.created_at >= ? AND m.created_at <= ?`,
		projectID, models.FormatTime(from), models.FormatTime(to)).Scan(&s.Messages); err != nil {
		return nil, fmt.Errorf("counting messages: %w", err)
	}
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.project_id = ? AND m.created_at >= ? AND m.created_at <= ?
		  AND m.sender_type = 'agent'`,
		projectID, models.FormatTime(from), models.FormatTime(to)).Scan(&s.AgentMessages); err != nil {
		return nil, fmt.Errorf("counting agent messages: %w", err)
	}
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets
		WHERE project_id = ? AND status IN ('open','in_progress','waiting')`, projectID).Scan(&s.TicketsOpen); err != nil {
		return nil, fmt.Errorf("counting tickets: %w", err)
	}
	return s, nil
}

// Series returns hourly or daily buckets, zero-filled across the window.
func (r *StatsRepo) Series(ctx context.Context, projectID string, from, to time.Time, bucket string) ([]SeriesPoint, error) {
	if bucket != "hour" {
		bucket = "day"
	}
	layout := "%Y-%m-%d"
	step := 24 * time.Hour
	key := func(t time.Time) string { return t.UTC().Format("2006-01-02") }
	if bucket == "hour" {
		layout = "%Y-%m-%d %H:00"
		step = time.Hour
		key = func(t time.Time) string { return t.UTC().Format("2006-01-02 15") + ":00" }
	}

	conv := map[string]int{}
	msg := map[string]int{}
	res := map[string]int{}

	if err := r.collect(ctx, &conv, `SELECT strftime('`+layout+`', created_at) AS b, COUNT(*)
		FROM conversations WHERE project_id = ? AND created_at >= ? AND created_at <= ?
		GROUP BY b`, projectID, from, to); err != nil {
		return nil, err
	}
	if err := r.collect(ctx, &msg, `SELECT strftime('`+layout+`', m.created_at) AS b, COUNT(*)
		FROM messages m JOIN conversations c ON c.id = m.conversation_id
		WHERE c.project_id = ? AND m.created_at >= ? AND m.created_at <= ?
		GROUP BY b`, projectID, from, to); err != nil {
		return nil, err
	}
	if err := r.collect(ctx, &res, `SELECT strftime('`+layout+`', resolved_at) AS b, COUNT(*)
		FROM conversations WHERE project_id = ? AND resolved_at IS NOT NULL
		  AND resolved_at >= ? AND resolved_at <= ?
		GROUP BY b`, projectID, from, to); err != nil {
		return nil, err
	}

	start := from.UTC().Truncate(step)
	if bucket == "day" {
		start = time.Date(from.UTC().Year(), from.UTC().Month(), from.UTC().Day(), 0, 0, 0, 0, time.UTC)
	}
	out := make([]SeriesPoint, 0, 32)
	for t := start; !t.After(to.UTC()); t = t.Add(step) {
		k := key(t)
		out = append(out, SeriesPoint{
			Bucket:        k,
			Conversations: conv[k],
			Messages:      msg[k],
			Resolved:      res[k],
		})
	}
	return out, nil
}

func (r *StatsRepo) collect(ctx context.Context, dst *map[string]int, query, projectID string, from, to time.Time) error {
	rows, err := r.db.R.QueryContext(ctx, query, projectID, models.FormatTime(from), models.FormatTime(to))
	if err != nil {
		return fmt.Errorf("collecting series: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			b string
			n int
		)
		if err := rows.Scan(&b, &n); err != nil {
			return wrapErr("scanning series bucket", err)
		}
		(*dst)[b] = n
	}
	return rows.Err()
}

// AgentStats returns the leaderboard for a window.
func (r *StatsRepo) AgentStats(ctx context.Context, projectID string, from, to time.Time) ([]*AgentStat, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT a.id, a.name,
		COUNT(c.id),
		COALESCE(SUM(CASE WHEN c.status IN ('resolved','closed') THEN 1 ELSE 0 END), 0),
		COALESCE(AVG(CASE WHEN c.first_response_at IS NOT NULL THEN (julianday(c.first_response_at) - julianday(c.created_at)) * 86400 END), 0),
		COALESCE(AVG(c.csat_score), 0)
		FROM agents a
		LEFT JOIN conversations c ON c.assigned_agent_id = a.id AND c.created_at >= ? AND c.created_at <= ?
		WHERE a.project_id = ?
		GROUP BY a.id, a.name
		ORDER BY COUNT(c.id) DESC, a.name ASC`,
		models.FormatTime(from), models.FormatTime(to), projectID)
	if err != nil {
		return nil, fmt.Errorf("computing agent stats: %w", err)
	}
	defer rows.Close()
	out := make([]*AgentStat, 0, 8)
	for rows.Next() {
		var s AgentStat
		if err := rows.Scan(&s.AgentID, &s.Name, &s.Conversations, &s.Resolved,
			&s.AvgFirstResponseSeconds, &s.CSATAvg); err != nil {
			return nil, wrapErr("scanning agent stat", err)
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// ---- SaaS metering ----

// SaaSRepo persists plans, tenant plans, usage counters and custom domains.
type SaaSRepo struct{ db *db.DB }

const planCols = `id, code, name, price_monthly, max_agents, max_conversations_month, max_ai_tokens_month, widget_branding, created_at`

func scanPlan(s scanner) (*models.Plan, error) {
	var (
		p         models.Plan
		createdAt models.NullTime
	)
	if err := s.Scan(&p.ID, &p.Code, &p.Name, &p.PriceMonthly, &p.MaxAgents, &p.MaxConversationsMonth,
		&p.MaxAITokensMonth, &p.WidgetBranding, &createdAt); err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Time
	return &p, nil
}

// UpsertPlan inserts or updates a plan definition.
func (r *SaaSRepo) UpsertPlan(ctx context.Context, p *models.Plan) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO plans
		(id, code, name, price_monthly, max_agents, max_conversations_month, max_ai_tokens_month, widget_branding, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(code) DO UPDATE SET name = excluded.name, price_monthly = excluded.price_monthly,
			max_agents = excluded.max_agents, max_conversations_month = excluded.max_conversations_month,
			max_ai_tokens_month = excluded.max_ai_tokens_month, widget_branding = excluded.widget_branding`,
		p.ID, p.Code, p.Name, p.PriceMonthly, p.MaxAgents, p.MaxConversationsMonth, p.MaxAITokensMonth,
		boolInt(p.WidgetBranding), models.NowStr())
	if err != nil {
		return fmt.Errorf("upserting plan: %w", err)
	}
	return nil
}

// ListPlans returns every plan.
func (r *SaaSRepo) ListPlans(ctx context.Context) ([]*models.Plan, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+planCols+` FROM plans ORDER BY price_monthly ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing plans: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Plan, 0, 4)
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, wrapErr("scanning plan", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetTenantPlan returns the plan code assigned to a project.
func (r *SaaSRepo) GetTenantPlan(ctx context.Context, projectID string) (*models.TenantPlan, error) {
	var (
		tp      models.TenantPlan
		period  models.NullTime
		updated models.NullTime
	)
	err := r.db.R.QueryRowContext(ctx, `SELECT project_id, plan_code, period_start, is_active, updated_at
		FROM tenant_plans WHERE project_id = ?`, projectID).Scan(&tp.ProjectID, &tp.PlanCode, &period, &tp.IsActive, &updated)
	if err != nil {
		return nil, wrapErr("getting tenant plan", err)
	}
	tp.PeriodStart = period.Time
	tp.UpdatedAt = updated.Time
	return &tp, nil
}

// SetTenantPlan assigns a plan to a project.
func (r *SaaSRepo) SetTenantPlan(ctx context.Context, projectID, planCode string) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO tenant_plans (project_id, plan_code, period_start, is_active, updated_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(project_id) DO UPDATE SET plan_code = excluded.plan_code, updated_at = excluded.updated_at`,
		projectID, planCode, models.NowStr(), models.NowStr())
	if err != nil {
		return fmt.Errorf("setting tenant plan: %w", err)
	}
	return nil
}

// IncrementUsage adds delta to a per-period counter.
func (r *SaaSRepo) IncrementUsage(ctx context.Context, projectID, metric, period string, delta int64) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO usage_counters (project_id, metric, period, value, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(project_id, metric, period) DO UPDATE SET value = usage_counters.value + excluded.value,
			updated_at = excluded.updated_at`,
		projectID, metric, period, delta, models.NowStr())
	if err != nil {
		return fmt.Errorf("incrementing usage: %w", err)
	}
	return nil
}

// Usage returns the counter value for one metric/period.
func (r *SaaSRepo) Usage(ctx context.Context, projectID, metric, period string) (int64, error) {
	var v int64
	err := r.db.R.QueryRowContext(ctx, `SELECT value FROM usage_counters
		WHERE project_id = ? AND metric = ? AND period = ?`, projectID, metric, period).Scan(&v)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("reading usage: %w", err)
	}
	return v, nil
}

// UsageList returns every counter of a period.
func (r *SaaSRepo) UsageList(ctx context.Context, projectID, period string) ([]*models.UsageStat, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT project_id, metric, period, value, updated_at
		FROM usage_counters WHERE project_id = ? AND period = ? ORDER BY metric ASC`, projectID, period)
	if err != nil {
		return nil, fmt.Errorf("listing usage: %w", err)
	}
	defer rows.Close()
	out := make([]*models.UsageStat, 0, 8)
	for rows.Next() {
		var (
			u      models.UsageStat
			update models.NullTime
		)
		if err := rows.Scan(&u.ProjectID, &u.Metric, &u.Period, &u.Value, &update); err != nil {
			return nil, wrapErr("scanning usage", err)
		}
		u.UpdatedAt = update.Time
		out = append(out, &u)
	}
	return out, rows.Err()
}

// AddDomain registers a custom widget domain.
func (r *SaaSRepo) AddDomain(ctx context.Context, d *models.CustomDomain) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO custom_domains (id, project_id, domain, verified, created_at)
		VALUES (?, ?, ?, 0, ?)`, d.ID, d.ProjectID, d.Domain, models.NowStr())
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("adding custom domain: %w", err)
	}
	return nil
}

// VerifyDomain marks a domain as verified.
func (r *SaaSRepo) VerifyDomain(ctx context.Context, id string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE custom_domains SET verified = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("verifying domain: %w", err)
	}
	return nil
}

// ListDomains returns the custom domains of a project.
func (r *SaaSRepo) ListDomains(ctx context.Context, projectID string) ([]*models.CustomDomain, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT id, project_id, domain, verified, created_at
		FROM custom_domains WHERE project_id = ? ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing domains: %w", err)
	}
	defer rows.Close()
	out := make([]*models.CustomDomain, 0, 4)
	for rows.Next() {
		var (
			d         models.CustomDomain
			createdAt models.NullTime
		)
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Domain, &d.Verified, &createdAt); err != nil {
			return nil, wrapErr("scanning domain", err)
		}
		d.CreatedAt = createdAt.Time
		out = append(out, &d)
	}
	return out, rows.Err()
}

// AllVerifiedDomains feeds the autocert HostPolicy.
func (r *SaaSRepo) AllVerifiedDomains(ctx context.Context) ([]string, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT domain FROM custom_domains WHERE verified = 1`)
	if err != nil {
		return nil, fmt.Errorf("listing verified domains: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 8)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, wrapErr("scanning domain", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteDomain removes a custom domain.
func (r *SaaSRepo) DeleteDomain(ctx context.Context, id string) error {
	_, err := r.db.W.ExecContext(ctx, `DELETE FROM custom_domains WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting domain: %w", err)
	}
	return nil
}
