package handlers

import (
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/middleware"
)

// WSAgent upgrades /ws/agent to a websocket and runs the agent loops.
// Browsers cannot set headers on a websocket handshake, so ?token= is accepted
// as well as the Authorization header (the query form is the primary path).
func (h *Handlers) WSAgent(w http.ResponseWriter, r *http.Request) error {
	token := middleware.BearerToken(r)
	if token == "" {
		return apierr.Unauthorized("توکن دسترسی لازم است")
	}
	claims, err := h.Tokens.ParseTyp(token, "access")
	if err != nil {
		return apierr.Unauthorized("نشست شما منقضی شده است")
	}
	agent, err := h.Repo.Agents.GetByID(r.Context(), claims.Subject)
	if err != nil || !agent.IsActive {
		return apierr.Unauthorized("حساب کاربری معتبر نیست")
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		return nil
	}
	conn.SetReadLimit(int64(h.Cfg.Hub.MaxMessageBytes))

	client := h.Hub.NewClient(conn, hub.KindAgent, agent.ID, agent.ProjectID, string(agent.Role), agent.Name)
	h.Hub.Register(client)
	defer h.Hub.Unregister(client)

	online := h.Hub.OnlineAgents(agent.ProjectID)
	client.Send(hub.NewEnvelope(hub.TypeSyncResponse, map[string]any{
		"now":           nowMS(),
		"online_agents": online,
		"agent":         agent,
	}))

	client.Run(r.Context())
	return nil
}

func nowMS() int64 { return time.Now().UnixMilli() }
