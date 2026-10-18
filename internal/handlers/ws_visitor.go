package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/coder/websocket"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/hub"
)

// WSVisitor upgrades /ws/visitor to a websocket and runs the visitor loops.
//
// Origin verification is intentionally disabled at the websocket layer: the
// widget is embedded on arbitrary customer domains, so authenticity is proven
// by the session token and the project domain allow-list (apikey middleware)
// instead of the browser Origin header.
func (h *Handlers) WSVisitor(w http.ResponseWriter, r *http.Request) error {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		return apierr.Unauthorized("نشست ویجت یافت نشد")
	}
	sess, err := h.Repo.Sessions.Get(r.Context(), token)
	if err != nil {
		return apierr.Unauthorized("نشست شما منقضی شده است")
	}
	if projectID := r.URL.Query().Get("project"); projectID != "" && projectID != sess.ProjectID {
		return apierr.Forbidden("پروژه نشست مطابقت ندارد")
	}
	v, err := h.Repo.Visitors.GetByID(r.Context(), sess.VisitorID)
	if err != nil {
		return apierr.Unauthorized("کاربر یافت نشد")
	}
	banned, _ := h.Repo.Visitors.IsBanned(r.Context(), v.ID)
	if banned {
		return apierr.Forbidden("دسترسی شما به گفتگو مسدود شده است")
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		return nil
	}
	conn.SetReadLimit(int64(h.Cfg.Hub.MaxMessageBytes))

	client := h.Hub.NewClient(conn, hub.KindVisitor, v.ID, sess.ProjectID, "", v.DisplayName())
	h.Hub.Register(client)
	defer h.Hub.Unregister(client)

	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	if conv := h.activeConversation(r, sess); conv != nil {
		if res, serr := h.Realtime.Sync(r.Context(), conv.ID, since); serr == nil {
			client.Send(hub.NewEnvelope(hub.TypeSyncResponse, res))
		}
	} else {
		client.Send(hub.NewEnvelope(hub.TypeSyncResponse, map[string]any{
			"now":      nowMS(),
			"messages": []any{},
		}))
	}

	client.Run(r.Context())
	return nil
}
