package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/doctor"
	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/runtimecfg"
)

// Diagnostics returns the full doctor report (superadmin only).
func (h *Handlers) Diagnostics(w http.ResponseWriter, r *http.Request) error {
	report := doctor.RunAll(r.Context(), doctor.Options{
		ConfigPath: h.ConfigPath,
		Cfg:        h.Cfg,
		Log:        h.Log,
		Version:    h.Version,
		Only:       parseCSVQuery(r.URL.Query().Get("only")),
	})
	writeJSON(w, http.StatusOK, report)
	return nil
}

func parseCSVQuery(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Metrics serves the Prometheus exposition.
func (h *Handlers) Metrics(w http.ResponseWriter, r *http.Request) error {
	if !h.Cfg.Metrics.Enabled || h.MetricsReg == nil {
		return apierr.NotFound("متریک‌ها غیرفعال هستند")
	}
	if token := strings.TrimSpace(h.Cfg.Metrics.Token); token != "" {
		got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
		ip := middleware.ClientIPWithProxies(r, h.Cfg.Server.TrustProxy, h.Cfg.Server.TrustedProxies)
		loopback := ip == "127.0.0.1" || ip == "::1"
		if !loopback && got != token {
			return apierr.Unauthorized("توکن متریک نامعتبر است")
		}
	}
	h.snapshotMetrics(r)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	h.MetricsReg.Render(w)
	return nil
}

// snapshotMetrics refreshes the DB-derived gauges right before a scrape.
func (h *Handlers) snapshotMetrics(r *http.Request) {
	ctx := r.Context()
	rows, err := h.Repo.DB().R.QueryContext(ctx, `SELECT status, COUNT(*) FROM conversations GROUP BY status`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var status string
			var n int
			if rows.Scan(&status, &n) == nil {
				h.MetricsReg.SetGauge("evechat_conversations_total", map[string]string{"status": status}, float64(n))
			}
		}
	}
	rows2, err := h.Repo.DB().R.QueryContext(ctx, `SELECT sender_type, COUNT(*) FROM messages GROUP BY sender_type`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var sender string
			var n int
			if rows2.Scan(&sender, &n) == nil {
				h.MetricsReg.SetGauge("evechat_messages_total", map[string]string{"sender_type": sender}, float64(n))
			}
		}
	}
	if stats := h.Hub.Stats(); h.MetricsReg != nil {
		h.MetricsReg.SetGauge("evechat_ws_connections", map[string]string{"kind": "visitor"}, float64(stats.Visitors))
		h.MetricsReg.SetGauge("evechat_ws_connections", map[string]string{"kind": "agent"}, float64(stats.Agents))
		h.MetricsReg.SetGauge("evechat_ws_clients_dropped_total", map[string]string{"reason": "slow"}, float64(stats.Dropped))
	}
}

// ListAudit returns the audit log page.
func (h *Handlers) ListAudit(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	limit, offset := parsePage(r)
	f := repo.AuditFilter{
		ProjectID: h.scopeProject(r),
		Action:    r.URL.Query().Get("action"),
		ActorID:   r.URL.Query().Get("actor"),
		From:      r.URL.Query().Get("from"),
		To:        r.URL.Query().Get("to"),
		Limit:     limit,
		Offset:    offset,
	}
	if c := claimsOf(r); c != nil && c.Role == "superadmin" && r.URL.Query().Get("project_id") != "" {
		f.ProjectID = r.URL.Query().Get("project_id")
	}
	entries, total, err := h.Repo.Audit.List(r.Context(), f)
	if err != nil {
		return apierr.Internal(err)
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="audit.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"created_at", "action", "actor_type", "actor_id", "actor_name", "target_type", "target_id", "ip", "detail"})
		for _, e := range entries {
			_ = cw.Write([]string{e.CreatedAt, e.Action, e.ActorType, e.ActorID, e.ActorName, e.TargetType, e.TargetID, e.IP, e.Detail})
		}
		cw.Flush()
		return cw.Error()
	}
	writeJSON(w, http.StatusOK, newList(entries, total, limit, offset))
	return nil
}

// GetBlocklist returns the runtime abuse blocklists.
func (h *Handlers) GetBlocklist(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"blocked_ips":         splitLines(h.State.Value(runtimecfg.KeyBlockedIPs)),
		"blocked_cidrs":       splitLines(h.State.Value(runtimecfg.KeyBlockedCIDRs)),
		"blocked_user_agents": splitLines(h.State.Value(runtimecfg.KeyBlockedUAs)),
		"pow_difficulty":      h.State.POWDifficulty(h.Cfg.Security.POWDifficulty),
		"active":              blocklistCounts(h.State),
	})
	return nil
}

// UpdateBlocklist replaces the runtime blocklists.
func (h *Handlers) UpdateBlocklist(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	var in struct {
		BlockedIPs        []string `json:"blocked_ips"`
		BlockedCIDRs      []string `json:"blocked_cidrs"`
		BlockedUserAgents []string `json:"blocked_user_agents"`
		POWDifficulty     *int     `json:"pow_difficulty"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	if err := h.State.Set(ctx, runtimecfg.KeyBlockedIPs, strings.Join(in.BlockedIPs, "\n")); err != nil {
		return apierr.Internal(err)
	}
	if err := h.State.Set(ctx, runtimecfg.KeyBlockedCIDRs, strings.Join(in.BlockedCIDRs, "\n")); err != nil {
		return apierr.Internal(err)
	}
	if err := h.State.Set(ctx, runtimecfg.KeyBlockedUAs, strings.Join(in.BlockedUserAgents, "\n")); err != nil {
		return apierr.Internal(err)
	}
	if in.POWDifficulty != nil {
		_ = h.State.Set(ctx, runtimecfg.KeyPOWDifficulty, strconv.Itoa(*in.POWDifficulty))
	}
	if err := h.State.ReloadBlocklist(ctx); err != nil {
		return apierr.BadRequest("قانون مسدودسازی نامعتبر است: " + err.Error())
	}
	h.recordAudit(r, h.scopeProject(r), "security.blocklist_updated", "settings", "", map[string]any{
		"ips": len(in.BlockedIPs), "cidrs": len(in.BlockedCIDRs), "uas": len(in.BlockedUserAgents),
	})
	return okMsg(w)
}

func splitLines(raw string) []string {
	out := []string{}
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func blocklistCounts(state *runtimecfg.State) map[string]int {
	ips, cidrs, uas := state.Blocklist().Rules()
	return map[string]int{"ips": ips, "cidrs": cidrs, "user_agents": uas}
}

// PublicConfig exposes non-secret runtime information to the dashboard.
func (h *Handlers) PublicConfig(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     h.Version,
		"commit":      h.Commit,
		"build_date":  h.BuildDate,
		"instance_id": h.State.InstanceID(),
		"started_at":  h.StartedAt.UTC(),
	})
	return nil
}

var _ = time.Now
var _ = fmt.Sprint
