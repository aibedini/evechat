package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/evechat/evechat/internal/apierr"
)

// StatsOverview returns the KPI cards.
func (h *Handlers) StatsOverview(w http.ResponseWriter, r *http.Request) error {
	from, to, err := defaultWindow(r)
	if err != nil {
		return err
	}
	stats, err := h.Repo.Stats.Overview(r.Context(), h.scopeProject(r), from, to)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from": from,
		"to":   to,
		"data": stats,
	})
	return nil
}

// StatsSeries returns the chart buckets.
func (h *Handlers) StatsSeries(w http.ResponseWriter, r *http.Request) error {
	from, to, err := defaultWindow(r)
	if err != nil {
		return err
	}
	bucket := r.URL.Query().Get("bucket")
	if bucket != "hour" {
		bucket = "day"
	}
	series, err := h.Repo.Stats.Series(r.Context(), h.scopeProject(r), from, to, bucket)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from":   from,
		"to":     to,
		"bucket": bucket,
		"data":   series,
	})
	return nil
}

// StatsAgents returns the leaderboard.
func (h *Handlers) StatsAgents(w http.ResponseWriter, r *http.Request) error {
	from, to, err := defaultWindow(r)
	if err != nil {
		return err
	}
	stats, err := h.Repo.Stats.AgentStats(r.Context(), h.scopeProject(r), from, to)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"from": from, "to": to, "data": stats})
	return nil
}

// Health reports process, database and hub health.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) error {
	dbStatus := "ok"
	if err := h.Repo.DB().R.PingContext(r.Context()); err != nil {
		dbStatus = "error"
	}
	hubStats := h.Hub.Stats()
	status := "ok"
	if dbStatus != "ok" {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         status,
		"version":        h.Version,
		"commit":         h.Commit,
		"build_date":     h.BuildDate,
		"go_version":     runtime.Version(),
		"instance_id":    h.State.InstanceID(),
		"setup_complete": h.State.SetupCompleted(),
		"uptime":         int64(time.Since(h.StartedAt).Seconds()),
		"uptime_seconds": int64(time.Since(h.StartedAt).Seconds()),
		"time":           time.Now().UTC(),
		"db":             dbStatus,
		"hub": map[string]any{
			"conns":    hubStats.Connections,
			"rooms":    hubStats.Rooms,
			"visitors": hubStats.Visitors,
			"agents":   hubStats.Agents,
			"dropped":  hubStats.Dropped,
		},
		"notifications_dropped": h.Notif.Dropped(),
	})
	return nil
}
