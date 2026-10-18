package handlers

import (
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/services"
)

var auditIDPrefixes = []string{
	"agt_", "conv_", "vis_", "proj_", "tkt_", "kb_", "wh_", "rule_", "msg_", "ch_", "sess_", "plan_", "user_",
}

var auditActionVerbs = map[string]bool{
	"resolve": true, "reopen": true, "assign": true, "transfer": true, "ban": true,
	"anonymize": true, "rotate-key": true, "reorder": true, "status": true,
	"password": true, "test": true, "notes": true, "send": true, "set-webhook": true,
	"convert-ticket": true, "read": true, "import": true, "export": true,
}

// auditAction turns a mutating request into a stable dotted action name, e.g.
// POST /api/v1/routing-rules/reorder -> routing-rules.reorder.
func auditAction(method, path string) string {
	if !strings.HasPrefix(path, "/api/v1/") {
		return ""
	}
	rest := strings.Trim(strings.TrimPrefix(path, "/api/v1/"), "/")
	if rest == "" {
		return ""
	}
	keep := make([]string, 0, 4)
	for _, seg := range strings.Split(rest, "/") {
		if seg == "" {
			continue
		}
		isID := false
		for _, prefix := range auditIDPrefixes {
			if strings.HasPrefix(seg, prefix) {
				isID = true
				break
			}
		}
		if isID {
			continue
		}
		keep = append(keep, seg)
	}
	verb := map[string]string{
		http.MethodPost:   "create",
		http.MethodPut:    "update",
		http.MethodPatch:  "update",
		http.MethodDelete: "delete",
	}[method]
	if verb == "" {
		return ""
	}
	if len(keep) > 0 {
		if last := keep[len(keep)-1]; auditActionVerbs[last] {
			verb = last
			keep = keep[:len(keep)-1]
		}
	}
	if len(keep) == 0 {
		return verb
	}
	return strings.Join(keep, ".") + "." + verb
}

// AuditMutation records an authenticated mutating API request. It is installed
// as middleware.MutationHook by the router, so every handler is covered without
// per-handler boilerplate; handlers that know more detail record richer entries
// themselves (setup, blocklists, ...).
func (h *Handlers) AuditMutation(r *http.Request, c *middleware.Claims, status int) {
	if h.Audit == nil || status < 200 || status >= 300 {
		return
	}
	action := auditAction(r.Method, r.URL.Path)
	if action == "" {
		return
	}
	projectID := ""
	if c != nil {
		projectID = c.ProjectID
	}
	in := services.AuditInput{
		ProjectID:  projectID,
		ActorType:  "agent",
		Action:     action,
		TargetType: "request",
		TargetID:   r.URL.Path,
		IP:         middleware.ClientIP(r, h.Cfg.Server.TrustProxy),
		UserAgent:  r.UserAgent(),
		Detail:     map[string]any{"method": r.Method, "status": status},
	}
	if c != nil {
		in.ActorID = c.Subject
		in.ActorName = c.Name
	}
	h.Audit.Record(r.Context(), in)
}
