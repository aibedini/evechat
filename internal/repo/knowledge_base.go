package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// KBRepo persists knowledge-base articles and searches them through FTS5.
type KBRepo struct{ db *db.DB }

const kbCols = `id, project_id, title, slug, content, excerpt, tags, is_published, view_count, created_by, created_at, updated_at`

func scanKB(s scanner, rank *float64) (*models.KBArticle, error) {
	var (
		a         models.KBArticle
		excerpt   sql.NullString
		createdBy sql.NullString
		createdAt models.NullTime
		updatedAt models.NullTime
	)
	dest := []any{&a.ID, &a.ProjectID, &a.Title, &a.Slug, &a.Content, &excerpt, &a.Tags, &a.IsPublished,
		&a.ViewCount, &createdBy, &createdAt, &updatedAt}
	if rank != nil {
		dest = append(dest, rank)
	}
	if err := s.Scan(dest...); err != nil {
		return nil, err
	}
	a.Excerpt = str(excerpt)
	a.CreatedBy = str(createdBy)
	a.CreatedAt = createdAt.Time
	a.UpdatedAt = updatedAt.Time
	if rank != nil {
		a.Rank = *rank
	}
	return &a, nil
}

// Create inserts an article (the FTS triggers index it automatically).
func (r *KBRepo) Create(ctx context.Context, a *models.KBArticle) error {
	now := models.NowStr()
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO knowledge_base
		(id, project_id, title, slug, content, excerpt, tags, is_published, view_count, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ProjectID, a.Title, a.Slug, a.Content, toNullStr(a.Excerpt), a.Tags, boolInt(a.IsPublished),
		a.ViewCount, toNullStr(a.CreatedBy), now, now)
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("creating kb article: %w", err)
	}
	return nil
}

// GetByID loads one article.
func (r *KBRepo) GetByID(ctx context.Context, id string) (*models.KBArticle, error) {
	a, err := scanKB(r.db.R.QueryRowContext(ctx, `SELECT `+kbCols+` FROM knowledge_base WHERE id = ?`, id), nil)
	if err != nil {
		return nil, wrapErr("getting kb article", err)
	}
	return a, nil
}

// GetBySlug loads one article by slug.
func (r *KBRepo) GetBySlug(ctx context.Context, projectID, slug string) (*models.KBArticle, error) {
	a, err := scanKB(r.db.R.QueryRowContext(ctx, `SELECT `+kbCols+` FROM knowledge_base
		WHERE project_id = ? AND slug = ?`, projectID, slug), nil)
	if err != nil {
		return nil, wrapErr("getting kb article by slug", err)
	}
	return a, nil
}

// List returns a page of articles plus the total count.
func (r *KBRepo) List(ctx context.Context, projectID string, publishedOnly bool, limit, offset int) ([]*models.KBArticle, int, error) {
	limit, offset = pageClamp(limit, offset)
	where := "project_id = ?"
	args := []any{projectID}
	if publishedOnly {
		where += " AND is_published = 1"
	}
	var total int
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_base WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting kb articles: %w", err)
	}
	qargs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+kbCols+` FROM knowledge_base WHERE `+where+
		` ORDER BY updated_at DESC LIMIT ? OFFSET ?`, qargs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing kb articles: %w", err)
	}
	defer rows.Close()
	out := make([]*models.KBArticle, 0, limit)
	for rows.Next() {
		a, err := scanKB(rows, nil)
		if err != nil {
			return nil, 0, wrapErr("scanning kb article", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// Update persists an article.
func (r *KBRepo) Update(ctx context.Context, a *models.KBArticle) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE knowledge_base SET title = ?, slug = ?, content = ?, excerpt = ?,
		tags = ?, is_published = ?, updated_at = ? WHERE id = ?`,
		a.Title, a.Slug, a.Content, toNullStr(a.Excerpt), a.Tags, boolInt(a.IsPublished), models.NowStr(), a.ID)
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("updating kb article: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes an article.
func (r *KBRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM knowledge_base WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting kb article: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// IncrementView bumps the view counter.
func (r *KBRepo) IncrementView(ctx context.Context, id string) {
	_, _ = r.db.W.ExecContext(ctx, `UPDATE knowledge_base SET view_count = view_count + 1 WHERE id = ?`, id)
}

// Search runs an FTS5 bm25 query. Only published articles are returned.
func (r *KBRepo) Search(ctx context.Context, projectID, query string, limit int) ([]*models.KBArticle, error) {
	match := FTSQuery(query)
	if match == "" {
		return []*models.KBArticle{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 4
	}
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+prefixedKB()+`, bm25(knowledge_base_fts) AS rank
		FROM knowledge_base_fts f
		JOIN knowledge_base k ON k.rowid = f.rowid
		WHERE knowledge_base_fts MATCH ? AND k.project_id = ? AND k.is_published = 1
		ORDER BY rank LIMIT ?`, match, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("searching kb: %w", err)
	}
	defer rows.Close()
	out := make([]*models.KBArticle, 0, limit)
	for rows.Next() {
		var rank float64
		a, err := scanKB(rows, &rank)
		if err != nil {
			return nil, wrapErr("scanning kb search result", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// FTSQuery converts free text into a safe FTS5 prefix query.
func FTSQuery(q string) string {
	tokens := strings.Fields(q)
	parts := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		var b strings.Builder
		for _, r := range tok {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				b.WriteRune(r)
			}
		}
		s := b.String()
		if s == "" {
			continue
		}
		parts = append(parts, `"`+s+`"*`)
		if len(parts) >= 8 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func prefixedKB() string {
	parts := strings.Split(kbCols, ",")
	for i, p := range parts {
		parts[i] = "k." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
