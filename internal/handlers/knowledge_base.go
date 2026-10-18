package handlers

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// ListKB returns the article list.
func (h *Handlers) ListKB(w http.ResponseWriter, r *http.Request) error {
	limit, offset := parsePage(r)
	published := r.URL.Query().Get("published") != "0"
	articles, total, err := h.Repo.KB.List(r.Context(), h.scopeProject(r), published, limit, offset)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(articles, total, limit, offset))
	return nil
}

// SearchKB runs the FTS5 search.
func (h *Handlers) SearchKB(w http.ResponseWriter, r *http.Request) error {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		return apierr.BadRequest("عبارت جستجو لازم است")
	}
	limit := atoiDefault(r.URL.Query().Get("limit"), 10)
	articles, err := h.Repo.KB.Search(r.Context(), h.scopeProject(r), q, limit)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(articles, len(articles), limit, 0))
	return nil
}

// GetKB returns one article.
func (h *Handlers) GetKB(w http.ResponseWriter, r *http.Request) error {
	a, err := h.Repo.KB.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("مقاله یافت نشد")
	}
	if err := h.checkProjectScope(r, a.ProjectID); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, a)
	return nil
}

type kbInput struct {
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Content     string   `json:"content"`
	Excerpt     *string  `json:"excerpt"`
	Tags        []string `json:"tags"`
	IsPublished *bool    `json:"is_published"`
}

// CreateKB adds an article.
func (h *Handlers) CreateKB(w http.ResponseWriter, r *http.Request) error {
	var in kbInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Title) == "" {
		return apierr.BadRequest("عنوان مقاله لازم است")
	}
	slug := slugify(in.Slug)
	if slug == "" {
		slug = slugify(in.Title)
	}
	projectID := h.scopeProject(r)
	if slug == "" {
		slug = id.New()
	}
	if existing, err := h.Repo.KB.GetBySlug(r.Context(), projectID, slug); err == nil && existing != nil {
		slug = slug + "-" + id.New()[:6]
	}
	published := true
	if in.IsPublished != nil {
		published = *in.IsPublished
	}
	claims := claimsOf(r)
	author := claims.Subject
	a := &models.KBArticle{
		ID:          id.NewPrefixed(id.PrefixKB),
		ProjectID:   projectID,
		Title:       strings.TrimSpace(in.Title),
		Slug:        slug,
		Content:     in.Content,
		Excerpt:     in.Excerpt,
		Tags:        models.StringList(in.Tags),
		IsPublished: published,
		CreatedBy:   &author,
	}
	if a.Excerpt == nil && len(in.Content) > 0 {
		ex := in.Content
		if len(ex) > 200 {
			ex = ex[:200]
		}
		a.Excerpt = &ex
	}
	if err := h.Repo.KB.Create(r.Context(), a); err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusCreated, a)
	return nil
}

// UpdateKB edits an article.
func (h *Handlers) UpdateKB(w http.ResponseWriter, r *http.Request) error {
	a, err := h.Repo.KB.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("مقاله یافت نشد")
	}
	if err := h.checkProjectScope(r, a.ProjectID); err != nil {
		return err
	}
	var in kbInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Title != "" {
		a.Title = strings.TrimSpace(in.Title)
	}
	if in.Slug != "" {
		a.Slug = slugify(in.Slug)
	}
	a.Content = in.Content
	if in.Excerpt != nil {
		a.Excerpt = in.Excerpt
	}
	if in.Tags != nil {
		a.Tags = models.StringList(in.Tags)
	}
	if in.IsPublished != nil {
		a.IsPublished = *in.IsPublished
	}
	if err := h.Repo.KB.Update(r.Context(), a); err != nil {
		if err == repo.ErrDuplicate {
			return apierr.Conflict("این slug قبلاً استفاده شده است")
		}
		return apierr.Internal(err)
	}
	return ok(w, a)
}

// DeleteKB removes an article.
func (h *Handlers) DeleteKB(w http.ResponseWriter, r *http.Request) error {
	a, err := h.Repo.KB.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("مقاله یافت نشد")
	}
	if err := h.checkProjectScope(r, a.ProjectID); err != nil {
		return err
	}
	if err := h.Repo.KB.Delete(r.Context(), a.ID); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// slugify keeps unicode letters/digits and turns whitespace into dashes.
func slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
