package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
)

// ListPlans returns the SaaS plan catalogue (superadmin).
func (h *Handlers) ListPlans(w http.ResponseWriter, r *http.Request) error {
	plans, err := h.Repo.SaaS.ListPlans(r.Context())
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(plans, len(plans), len(plans), 0))
	return nil
}

// UpsertPlan creates or updates a plan (superadmin).
func (h *Handlers) UpsertPlan(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code                  string `json:"code"`
		Name                  string `json:"name"`
		PriceMonthly          int    `json:"price_monthly"`
		MaxAgents             int    `json:"max_agents"`
		MaxConversationsMonth int    `json:"max_conversations_month"`
		MaxAITokensMonth      int    `json:"max_ai_tokens_month"`
		WidgetBranding        *bool  `json:"widget_branding"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Name) == "" {
		return apierr.BadRequest("کد و نام پلن لازم است")
	}
	branding := true
	if in.WidgetBranding != nil {
		branding = *in.WidgetBranding
	}
	p := &models.Plan{
		ID:                    id.NewPrefixed("plan"),
		Code:                  strings.TrimSpace(in.Code),
		Name:                  strings.TrimSpace(in.Name),
		PriceMonthly:          in.PriceMonthly,
		MaxAgents:             in.MaxAgents,
		MaxConversationsMonth: in.MaxConversationsMonth,
		MaxAITokensMonth:      in.MaxAITokensMonth,
		WidgetBranding:        branding,
	}
	if err := h.Repo.SaaS.UpsertPlan(r.Context(), p); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, p)
}

// GetUsage returns the metered counters of a project (superadmin).
func (h *Handlers) GetUsage(w http.ResponseWriter, r *http.Request) error {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		return apierr.BadRequest("project_id لازم است")
	}
	period := r.URL.Query().Get("period")
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}
	usage, err := h.Repo.SaaS.UsageList(r.Context(), projectID, period)
	if err != nil {
		return apierr.Internal(err)
	}
	tenant, terr := h.Repo.SaaS.GetTenantPlan(r.Context(), projectID)
	resp := map[string]any{"period": period, "usage": usage}
	if terr == nil {
		resp["tenant_plan"] = tenant
	}
	return ok(w, resp)
}

// SetTenantPlan assigns a plan to a project (superadmin).
func (h *Handlers) SetTenantPlan(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		ProjectID string `json:"project_id"`
		PlanCode  string `json:"plan_code"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.ProjectID == "" || in.PlanCode == "" {
		return apierr.BadRequest("شناسه پروژه و کد پلن لازم است")
	}
	if err := h.Repo.SaaS.SetTenantPlan(r.Context(), in.ProjectID, in.PlanCode); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// ListDomains returns the custom widget domains of the project.
func (h *Handlers) ListDomains(w http.ResponseWriter, r *http.Request) error {
	domains, err := h.Repo.SaaS.ListDomains(r.Context(), h.scopeProject(r))
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(domains, len(domains), len(domains), 0))
	return nil
}

// AddDomain registers a CNAME target for the widget.
func (h *Handlers) AddDomain(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	var in struct {
		Domain string `json:"domain"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	domain := strings.ToLower(strings.TrimSpace(in.Domain))
	if domain == "" || !strings.Contains(domain, ".") {
		return apierr.BadRequest("دامنه معتبر نیست")
	}
	d := &models.CustomDomain{
		ID:        id.NewPrefixed("dom"),
		ProjectID: h.scopeProject(r),
		Domain:    domain,
	}
	if err := h.Repo.SaaS.AddDomain(r.Context(), d); err != nil {
		if err.Error() == "duplicate" {
			return apierr.Conflict("این دامنه قبلاً ثبت شده است")
		}
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"domain":   d,
		"dns_hint": "CNAME " + domain + " -> " + h.Cfg.BaseURL(),
	})
	return nil
}

// VerifyDomain marks a custom domain verified.
func (h *Handlers) VerifyDomain(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	if err := h.Repo.SaaS.VerifyDomain(r.Context(), chiURLParam(r, "id")); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// DeleteDomain removes a custom domain.
func (h *Handlers) DeleteDomain(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	if err := h.Repo.SaaS.DeleteDomain(r.Context(), chiURLParam(r, "id")); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// ListProjects returns every tenant (superadmin).
func (h *Handlers) ListProjects(w http.ResponseWriter, r *http.Request) error {
	projects, err := h.Repo.Projects.List(r.Context())
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(projects, len(projects), len(projects), 0))
	return nil
}

// CreateProject provisions a tenant (superadmin).
func (h *Handlers) CreateProject(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name    string   `json:"name"`
		Domains []string `json:"domains"`
		Plan    string   `json:"plan"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Name) == "" {
		return apierr.BadRequest("نام پروژه لازم است")
	}
	p := &models.Project{
		ID:        id.NewPrefixed(id.PrefixProject),
		Name:      strings.TrimSpace(in.Name),
		APIKey:    id.NewPrefixed("pk"),
		SecretKey: id.Token(32),
		Domains:   models.StringList(in.Domains),
		Settings:  models.JSONMap{},
		IsActive:  true,
	}
	if err := h.Repo.Projects.Create(r.Context(), p); err != nil {
		return apierr.Internal(err)
	}
	if in.Plan != "" {
		_ = h.Repo.SaaS.SetTenantPlan(r.Context(), p.ID, in.Plan)
	}
	writeJSON(w, http.StatusCreated, p)
	return nil
}
