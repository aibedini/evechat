package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// ProjectRepo persists tenants.
type ProjectRepo struct{ db *db.DB }

const projectCols = `id, name, api_key, secret_key, domains, settings, is_active, created_at, updated_at`

func scanProject(s scanner) (*models.Project, error) {
	var (
		p         models.Project
		createdAt models.NullTime
		updatedAt models.NullTime
	)
	if err := s.Scan(&p.ID, &p.Name, &p.APIKey, &p.SecretKey, &p.Domains, &p.Settings, &p.IsActive, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Time
	p.UpdatedAt = updatedAt.Time
	return &p, nil
}

// Create inserts a project.
func (r *ProjectRepo) Create(ctx context.Context, p *models.Project) error {
	now := models.NowStr()
	if p.CreatedAt.IsZero() {
		p.CreatedAt, _ = models.ParseTime(now)
	}
	p.UpdatedAt = p.CreatedAt
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO projects (id, name, api_key, secret_key, domains, settings, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.APIKey, p.SecretKey, p.Domains, p.Settings, boolInt(p.IsActive), now, now)
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("creating project: %w", err)
	}
	return nil
}

// GetByID loads one project.
func (r *ProjectRepo) GetByID(ctx context.Context, id string) (*models.Project, error) {
	p, err := scanProject(r.db.R.QueryRowContext(ctx, `SELECT `+projectCols+` FROM projects WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting project", err)
	}
	return p, nil
}

// GetByAPIKey loads a project by its public key.
func (r *ProjectRepo) GetByAPIKey(ctx context.Context, apiKey string) (*models.Project, error) {
	p, err := scanProject(r.db.R.QueryRowContext(ctx, `SELECT `+projectCols+` FROM projects WHERE api_key = ?`, apiKey))
	if err != nil {
		return nil, wrapErr("getting project by api key", err)
	}
	return p, nil
}

// List returns every project (superadmin).
func (r *ProjectRepo) List(ctx context.Context) ([]*models.Project, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+projectCols+` FROM projects ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Project, 0, 8)
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, wrapErr("scanning project", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Update persists the mutable project fields.
func (r *ProjectRepo) Update(ctx context.Context, p *models.Project) error {
	p.UpdatedAt = time.Now().UTC()
	res, err := r.db.W.ExecContext(ctx, `UPDATE projects SET name = ?, domains = ?, settings = ?, is_active = ?, updated_at = ?
		WHERE id = ?`, p.Name, p.Domains, p.Settings, boolInt(p.IsActive), models.FormatTime(p.UpdatedAt), p.ID)
	if err != nil {
		return fmt.Errorf("updating project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a project and all of its data.
func (r *ProjectRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RotateAPIKey replaces the public widget key.
func (r *ProjectRepo) RotateAPIKey(ctx context.Context, id, apiKey string) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE projects SET api_key = ?, updated_at = ? WHERE id = ?`,
		apiKey, models.NowStr(), id)
	if err != nil {
		return fmt.Errorf("rotating api key: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
