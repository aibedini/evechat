package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// VisitorRepo persists widget end users.
type VisitorRepo struct{ db *db.DB }

const visitorCols = `id, project_id, name, email, phone, ip, country, device_type, os, browser,
	user_agent, language, eve_mobile, eve_user_id, eve_reseller_id, eve_subscription_info,
	metadata, is_banned, banned_until, first_seen_at, last_seen_at`

// VisitorFilter narrows a visitor listing.
type VisitorFilter struct {
	Query      string
	Online     bool
	OnlyOnline bool
	Reseller   string
	DeviceType string
	Banned     *bool
	Limit      int
	Offset     int
}

func scanVisitor(s scanner) (*models.Visitor, error) {
	var (
		v      models.Visitor
		name   sql.NullString
		email  sql.NullString
		phone  sql.NullString
		ip     sql.NullString
		ctry   sql.NullString
		device sql.NullString
		os     sql.NullString
		browser sql.NullString
		ua     sql.NullString
		lang   sql.NullString
		mobile sql.NullString
		uid    sql.NullString
		rid    sql.NullString
		banned models.NullTime
		first  models.NullTime
		last   models.NullTime
	)
	if err := s.Scan(&v.ID, &v.ProjectID, &name, &email, &phone, &ip, &ctry, &device, &os, &browser,
		&ua, &lang, &mobile, &uid, &rid, &v.SubscriptionInfo, &v.Metadata, &v.IsBanned, &banned, &first, &last); err != nil {
		return nil, err
	}
	v.Name = str(name)
	v.Email = str(email)
	v.Phone = str(phone)
	v.IP = str(ip)
	v.Country = str(ctry)
	v.DeviceType = str(device)
	v.OS = str(os)
	v.Browser = str(browser)
	v.UserAgent = str(ua)
	v.Language = str(lang)
	v.EVEMobile = str(mobile)
	v.EVEUserID = str(uid)
	v.EVEResellerID = str(rid)
	v.BannedUntil = banned.Ptr()
	v.FirstSeenAt = first.Time
	v.LastSeenAt = last.Time
	return &v, nil
}

// Upsert inserts or refreshes a visitor. Incoming empty values never overwrite
// stored non-empty values.
func (r *VisitorRepo) Upsert(ctx context.Context, v *models.Visitor) error {
	now := models.NowStr()
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO visitors
		(id, project_id, name, email, phone, ip, country, device_type, os, browser, user_agent, language,
		 eve_mobile, eve_user_id, eve_reseller_id, eve_subscription_info, metadata, is_banned, banned_until,
		 first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NULL, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = COALESCE(NULLIF(excluded.name, ''), visitors.name),
			email = COALESCE(NULLIF(excluded.email, ''), visitors.email),
			phone = COALESCE(NULLIF(excluded.phone, ''), visitors.phone),
			ip = COALESCE(NULLIF(excluded.ip, ''), visitors.ip),
			country = COALESCE(NULLIF(excluded.country, ''), visitors.country),
			device_type = COALESCE(NULLIF(excluded.device_type, ''), visitors.device_type),
			os = COALESCE(NULLIF(excluded.os, ''), visitors.os),
			browser = COALESCE(NULLIF(excluded.browser, ''), visitors.browser),
			user_agent = COALESCE(NULLIF(excluded.user_agent, ''), visitors.user_agent),
			language = COALESCE(NULLIF(excluded.language, ''), visitors.language),
			eve_mobile = COALESCE(NULLIF(excluded.eve_mobile, ''), visitors.eve_mobile),
			eve_user_id = COALESCE(NULLIF(excluded.eve_user_id, ''), visitors.eve_user_id),
			eve_reseller_id = COALESCE(NULLIF(excluded.eve_reseller_id, ''), visitors.eve_reseller_id),
			eve_subscription_info = CASE WHEN excluded.eve_subscription_info = '{}' THEN visitors.eve_subscription_info ELSE excluded.eve_subscription_info END,
			metadata = CASE WHEN excluded.metadata = '{}' THEN visitors.metadata ELSE excluded.metadata END,
			last_seen_at = excluded.last_seen_at`,
		v.ID, v.ProjectID, toNullStr(v.Name), toNullStr(v.Email), toNullStr(v.Phone), toNullStr(v.IP),
		toNullStr(v.Country), toNullStr(v.DeviceType), toNullStr(v.OS), toNullStr(v.Browser),
		toNullStr(v.UserAgent), toNullStr(v.Language), toNullStr(v.EVEMobile), toNullStr(v.EVEUserID),
		toNullStr(v.EVEResellerID), v.SubscriptionInfo, v.Metadata, now, now)
	if err != nil {
		return fmt.Errorf("upserting visitor: %w", err)
	}
	return nil
}

// GetByID loads one visitor.
func (r *VisitorRepo) GetByID(ctx context.Context, id string) (*models.Visitor, error) {
	v, err := scanVisitor(r.db.R.QueryRowContext(ctx, `SELECT `+visitorCols+` FROM visitors WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting visitor", err)
	}
	return v, nil
}

// List returns a filtered page of visitors plus the total count.
func (r *VisitorRepo) List(ctx context.Context, projectID string, f VisitorFilter) ([]*models.Visitor, int, error) {
	limit, offset := pageClamp(f.Limit, f.Offset)
	where := []string{"project_id = ?"}
	args := []any{projectID}

	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, `(name LIKE ? ESCAPE '\' OR email LIKE ? ESCAPE '\' OR phone LIKE ? ESCAPE '\' OR eve_mobile LIKE ? ESCAPE '\')`)
		pat := likeTerm(q)
		args = append(args, pat, pat, pat, pat)
	}
	if f.Online || f.OnlyOnline {
		where = append(where, "last_seen_at >= ?")
		args = append(args, models.FormatTime(time.Now().Add(-5*time.Minute)))
	}
	if f.Reseller != "" {
		where = append(where, "eve_reseller_id = ?")
		args = append(args, f.Reseller)
	}
	if f.DeviceType != "" {
		where = append(where, "device_type = ?")
		args = append(args, f.DeviceType)
	}
	if f.Banned != nil {
		where = append(where, "is_banned = ?")
		args = append(args, boolInt(*f.Banned))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM visitors WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting visitors: %w", err)
	}

	qargs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+visitorCols+` FROM visitors WHERE `+cond+
		` ORDER BY last_seen_at DESC LIMIT ? OFFSET ?`, qargs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing visitors: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Visitor, 0, limit)
	for rows.Next() {
		v, err := scanVisitor(rows)
		if err != nil {
			return nil, 0, wrapErr("scanning visitor", err)
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// Update persists editable visitor fields.
func (r *VisitorRepo) Update(ctx context.Context, v *models.Visitor) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE visitors SET name = ?, email = ?, phone = ?, country = ?,
		language = ?, metadata = ?, eve_mobile = ?, eve_user_id = ?, eve_reseller_id = ?, eve_subscription_info = ?
		WHERE id = ?`,
		toNullStr(v.Name), toNullStr(v.Email), toNullStr(v.Phone), toNullStr(v.Country), toNullStr(v.Language),
		v.Metadata, toNullStr(v.EVEMobile), toNullStr(v.EVEUserID), toNullStr(v.EVEResellerID), v.SubscriptionInfo, v.ID)
	if err != nil {
		return fmt.Errorf("updating visitor: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLastSeen refreshes presence.
func (r *VisitorRepo) UpdateLastSeen(ctx context.Context, id string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE visitors SET last_seen_at = ? WHERE id = ?`, models.NowStr(), id)
	if err != nil {
		return fmt.Errorf("updating visitor last seen: %w", err)
	}
	return nil
}

// Ban blocks a visitor until the given time (nil = permanent).
func (r *VisitorRepo) Ban(ctx context.Context, id string, until *time.Time) error {
	var exp any
	if until != nil {
		exp = models.FormatTime(*until)
	}
	res, err := r.db.W.ExecContext(ctx, `UPDATE visitors SET is_banned = 1, banned_until = ? WHERE id = ?`, exp, id)
	if err != nil {
		return fmt.Errorf("banning visitor: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Unban lifts a ban.
func (r *VisitorRepo) Unban(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE visitors SET is_banned = 0, banned_until = NULL WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("unbanning visitor: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// IsBanned reports whether the visitor is currently banned.
func (r *VisitorRepo) IsBanned(ctx context.Context, id string) (bool, error) {
	var b bool
	var until models.NullTime
	err := r.db.R.QueryRowContext(ctx, `SELECT is_banned, banned_until FROM visitors WHERE id = ?`, id).Scan(&b, &until)
	if err != nil {
		return false, wrapErr("checking ban", err)
	}
	if !b {
		return false, nil
	}
	if until.Valid && until.Time.Before(time.Now().UTC()) {
		_ = r.Unban(ctx, id)
		return false, nil
	}
	return true, nil
}

// Anonymize removes all personal data (GDPR erase).
func (r *VisitorRepo) Anonymize(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE visitors SET
		name = NULL, email = NULL, phone = NULL, ip = NULL, user_agent = NULL,
		eve_mobile = NULL, eve_user_id = NULL, eve_reseller_id = NULL,
		eve_subscription_info = '{}', metadata = '{}'
		WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("anonymizing visitor: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Count returns the number of visitors in a project.
func (r *VisitorRepo) Count(ctx context.Context, projectID string) (int, error) {
	var n int
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM visitors WHERE project_id = ?`, projectID).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting visitors: %w", err)
	}
	return n, nil
}

// GetByIDs batch-loads visitors for list enrichment.
func (r *VisitorRepo) GetByIDs(ctx context.Context, ids []string) (map[string]*models.Visitor, error) {
	out := make(map[string]*models.Visitor, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+visitorCols+` FROM visitors WHERE id IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("batch loading visitors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		v, err := scanVisitor(rows)
		if err != nil {
			return nil, wrapErr("scanning visitor", err)
		}
		out[v.ID] = v
	}
	return out, rows.Err()
}
