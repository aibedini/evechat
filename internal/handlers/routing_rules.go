package handlers

import (
	"net/http"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/services"
)

// ListRoutingRules returns the project rules, highest priority first.
func (h *Handlers) ListRoutingRules(w http.ResponseWriter, r *http.Request) error {
	rules, err := h.Repo.Rules.List(r.Context(), h.scopeProject(r), false)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(rules, len(rules), len(rules), 0))
	return nil
}

// GetRoutingRule returns one rule.
func (h *Handlers) GetRoutingRule(w http.ResponseWriter, r *http.Request) error {
	rule, err := h.Repo.Rules.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("قانون یافت نشد")
	}
	if err := h.checkProjectScope(r, rule.ProjectID); err != nil {
		return err
	}
	return ok(w, rule)
}

type ruleInput struct {
	Name       string         `json:"name"`
	Priority   int            `json:"priority"`
	Conditions models.JSONMap `json:"conditions"`
	RouteTo    models.JSONMap `json:"route_to"`
	AutoReply  *string        `json:"auto_reply"`
	IsActive   *bool          `json:"is_active"`
}

// CreateRoutingRule creates a rule.
func (h *Handlers) CreateRoutingRule(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	var in ruleInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Name == "" {
		return apierr.BadRequest("نام قانون لازم است")
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	rule := &models.RoutingRule{
		ID:         id.NewPrefixed(id.PrefixRule),
		ProjectID:  h.scopeProject(r),
		Name:       in.Name,
		Priority:   in.Priority,
		Conditions: in.Conditions,
		RouteTo:    in.RouteTo,
		AutoReply:  in.AutoReply,
		IsActive:   active,
	}
	if rule.Conditions == nil {
		rule.Conditions = models.JSONMap{}
	}
	if rule.RouteTo == nil {
		rule.RouteTo = models.JSONMap{}
	}
	if err := h.Repo.Rules.Create(r.Context(), rule); err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusCreated, rule)
	return nil
}

// UpdateRoutingRule updates a rule.
func (h *Handlers) UpdateRoutingRule(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	rule, err := h.Repo.Rules.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("قانون یافت نشد")
	}
	if err := h.checkProjectScope(r, rule.ProjectID); err != nil {
		return err
	}
	var in ruleInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Name != "" {
		rule.Name = in.Name
	}
	rule.Priority = in.Priority
	if in.Conditions != nil {
		rule.Conditions = in.Conditions
	}
	if in.RouteTo != nil {
		rule.RouteTo = in.RouteTo
	}
	rule.AutoReply = in.AutoReply
	if in.IsActive != nil {
		rule.IsActive = *in.IsActive
	}
	if err := h.Repo.Rules.Update(r.Context(), rule); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, rule)
}

// DeleteRoutingRule deletes a rule.
func (h *Handlers) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	rule, err := h.Repo.Rules.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("قانون یافت نشد")
	}
	if err := h.checkProjectScope(r, rule.ProjectID); err != nil {
		return err
	}
	if err := h.Repo.Rules.Delete(r.Context(), rule.ID); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// ReorderRoutingRules applies a drag-and-drop ordering.
func (h *Handlers) ReorderRoutingRules(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	var in struct {
		Order []repo.IDPriority `json:"order"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if len(in.Order) == 0 {
		return apierr.BadRequest("ترتیب جدید خالی است")
	}
	if err := h.Repo.Rules.Reorder(r.Context(), in.Order); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// TestRoutingRule reports which rule a sample visitor would match.
func (h *Handlers) TestRoutingRule(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Visitor struct {
			DeviceType    string         `json:"device_type"`
			Language      string         `json:"language"`
			EVEResellerID string         `json:"eve_reseller_id"`
			Metadata      models.JSONMap `json:"metadata"`
			Subscription  models.JSONMap `json:"subscription"`
		} `json:"visitor_sample"`
		Page struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"page"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	projectID := h.scopeProject(r)
	project, err := h.projectOf(r.Context(), projectID)
	if err != nil {
		return err
	}
	v := &models.Visitor{
		DeviceType:       nilIfEmptyH(in.Visitor.DeviceType),
		Language:         nilIfEmptyH(in.Visitor.Language),
		EVEResellerID:    nilIfEmptyH(in.Visitor.EVEResellerID),
		Metadata:         in.Visitor.Metadata,
		SubscriptionInfo: in.Visitor.Subscription,
	}
	if v.Metadata == nil {
		v.Metadata = models.JSONMap{}
	}
	if v.SubscriptionInfo == nil {
		v.SubscriptionInfo = models.JSONMap{}
	}
	conv := &models.Conversation{PageURL: nilIfEmptyH(in.Page.URL), PageTitle: nilIfEmptyH(in.Page.Title)}
	decision, err := h.Routing.Decide(r.Context(), &models.Conversation{ProjectID: projectID, PageURL: conv.PageURL, PageTitle: conv.PageTitle}, v)
	if err != nil {
		return apierr.Internal(err)
	}
	var matched *models.RoutingRule
	if decision.RuleID != "" {
		matched, _ = h.Repo.Rules.GetByID(r.Context(), decision.RuleID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"matched_rule": matched,
		"decision": map[string]any{
			"rule_id":     decision.RuleID,
			"agent_id":    decision.AgentID,
			"telegram_id": decision.TelegramID,
			"phone":       decision.Phone,
			"auto_reply":  decision.AutoReply,
			"fallback":    decision.Fallback,
		},
		"business_hours": services.InBusinessHours(project.Settings, timeNowH()),
	})
	return nil
}

func nilIfEmptyH(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// requireManager rejects plain agents for configuration endpoints.
func (h *Handlers) requireManager(r *http.Request) error {
	c := claimsOf(r)
	if c == nil {
		return apierr.Unauthorized("احراز هویت لازم است")
	}
	if c.Role != models.RoleAdmin && c.Role != models.RoleSuperAdmin {
		return apierr.Forbidden("فقط مدیران امکان تغییر تنظیمات را دارند")
	}
	return nil
}
