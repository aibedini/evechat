package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// PushRepo stores FCM device registrations.
//
// DeviceID is the app-generated stable key. A re-registration updates the
// token/metadata and moves the device to the current agent (the Android app
// signs out one agent before signing in another on the same install). Tokens
// are unique; a token that somehow moves to a new DeviceID replaces the old
// row so a stale device cannot keep receiving another agent's pushes.
type PushRepo struct{ db *db.DB }

const pushCols = `device_id, agent_id, project_id, token, platform, provider,
	app_version, app_build, os_version, device_model, locale, timezone,
	created_at, updated_at, last_success_at`

const pushColsPD = `pd.device_id, pd.agent_id, pd.project_id, pd.token, pd.platform, pd.provider,
	pd.app_version, pd.app_build, pd.os_version, pd.device_model, pd.locale, pd.timezone,
	pd.created_at, pd.updated_at, pd.last_success_at`

func scanPushDevice(s scanner) (*models.PushDevice, error) {
	var (
		d         models.PushDevice
		createdAt models.NullTime
		updatedAt models.NullTime
		lastOK    models.NullTime
	)
	if err := s.Scan(&d.DeviceID, &d.AgentID, &d.ProjectID, &d.Token, &d.Platform, &d.Provider,
		&d.AppVersion, &d.AppBuild, &d.OSVersion, &d.DeviceModel, &d.Locale, &d.Timezone,
		&createdAt, &updatedAt, &lastOK); err != nil {
		return nil, err
	}
	d.CreatedAt = createdAt.Time
	d.UpdatedAt = updatedAt.Time
	d.LastSuccess = lastOK.Ptr()
	return &d, nil
}

// Upsert inserts a device or updates it in place. It reports whether a new row
// was created so the handler can answer 201 or 200.
func (r *PushRepo) Upsert(ctx context.Context, d *models.PushDevice) (bool, error) {
	created := false
	now := models.NowStr()
	err := r.db.Tx(ctx, func(tx *sql.Tx) error {
		var one int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM push_devices WHERE device_id = ?`, d.DeviceID).Scan(&one)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			created = true
		case err != nil:
			return fmt.Errorf("checking push device: %w", err)
		}

		// A token identifies one installation. If it was previously attached to
		// another DeviceID, remove that stale row before enforcing UNIQUE(token).
		if _, err := tx.ExecContext(ctx, `DELETE FROM push_devices WHERE token = ? AND device_id <> ?`,
			d.Token, d.DeviceID); err != nil {
			return fmt.Errorf("removing stale push token: %w", err)
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO push_devices
			(device_id, agent_id, project_id, token, platform, provider, app_version,
			 app_build, os_version, device_model, locale, timezone, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(device_id) DO UPDATE SET
				agent_id = excluded.agent_id,
				project_id = excluded.project_id,
				token = excluded.token,
				platform = excluded.platform,
				provider = excluded.provider,
				app_version = excluded.app_version,
				app_build = excluded.app_build,
				os_version = excluded.os_version,
				device_model = excluded.device_model,
				locale = excluded.locale,
				timezone = excluded.timezone,
				updated_at = excluded.updated_at,
				last_success_at = CASE
					WHEN push_devices.token = excluded.token THEN push_devices.last_success_at
					ELSE NULL
				END`,
			d.DeviceID, d.AgentID, d.ProjectID, d.Token, d.Platform, d.Provider,
			d.AppVersion, d.AppBuild, d.OSVersion, d.DeviceModel, d.Locale, d.Timezone, now, now)
		if err != nil {
			if isUnique(err) {
				return ErrConflict
			}
			return fmt.Errorf("upserting push device: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}

// Get loads one device.
func (r *PushRepo) Get(ctx context.Context, deviceID string) (*models.PushDevice, error) {
	d, err := scanPushDevice(r.db.R.QueryRowContext(ctx, `SELECT `+pushCols+` FROM push_devices WHERE device_id = ?`, deviceID))
	if err != nil {
		return nil, wrapErr("getting push device", err)
	}
	return d, nil
}

// Delete removes a device by id.
func (r *PushRepo) Delete(ctx context.Context, deviceID string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM push_devices WHERE device_id = ?`, deviceID)
	if err != nil {
		return fmt.Errorf("deleting push device: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByToken removes every registration for a token FCM reported invalid.
func (r *PushRepo) DeleteByToken(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	_, err := r.db.W.ExecContext(ctx, `DELETE FROM push_devices WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("deleting push token: %w", err)
	}
	return nil
}

// TouchSuccess records a successful FCM delivery.
func (r *PushRepo) TouchSuccess(ctx context.Context, deviceID string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE push_devices SET last_success_at = ? WHERE device_id = ?`,
		models.NowStr(), deviceID)
	if err != nil {
		return fmt.Errorf("recording push success: %w", err)
	}
	return nil
}

// ListByAgent returns the active registrations of one active agent.
func (r *PushRepo) ListByAgent(ctx context.Context, agentID string) ([]*models.PushDevice, error) {
	return r.list(ctx, `SELECT `+pushColsPD+` FROM push_devices pd
		JOIN agents a ON a.id = pd.agent_id
		WHERE pd.agent_id = ? AND a.is_active = 1
		ORDER BY pd.updated_at DESC`, agentID)
}

// ListByProject returns registrations of every active agent in a project.
func (r *PushRepo) ListByProject(ctx context.Context, projectID string) ([]*models.PushDevice, error) {
	return r.list(ctx, `SELECT `+pushColsPD+` FROM push_devices pd
		JOIN agents a ON a.id = pd.agent_id
		WHERE pd.project_id = ? AND a.is_active = 1
		ORDER BY pd.updated_at DESC`, projectID)
}

// ListByProjectExceptAgent is used for the "withdraw the notification from
// everyone else" conversation.updated push after an assignment.
func (r *PushRepo) ListByProjectExceptAgent(ctx context.Context, projectID, agentID string) ([]*models.PushDevice, error) {
	return r.list(ctx, `SELECT `+pushColsPD+` FROM push_devices pd
		JOIN agents a ON a.id = pd.agent_id
		WHERE pd.project_id = ? AND pd.agent_id <> ? AND a.is_active = 1
		ORDER BY pd.updated_at DESC`, projectID, agentID)
}

func (r *PushRepo) list(ctx context.Context, query string, args ...any) ([]*models.PushDevice, error) {
	rows, err := r.db.R.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing push devices: %w", err)
	}
	defer rows.Close()
	out := make([]*models.PushDevice, 0, 8)
	for rows.Next() {
		d, err := scanPushDevice(rows)
		if err != nil {
			return nil, wrapErr("scanning push device", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// PruneStale removes registrations not refreshed since before the cutoff.
func (r *PushRepo) PruneStale(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM push_devices WHERE updated_at < ?`, models.FormatTime(before))
	if err != nil {
		return 0, fmt.Errorf("pruning push devices: %w", err)
	}
	return res.RowsAffected()
}
