package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/services"
)

// ListMessages returns the decrypted message history of a conversation.
func (h *Handlers) ListMessages(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, cerr := strconv.Atoi(v); cerr == nil {
			limit = n
		}
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var since int64
	if v := r.URL.Query().Get("since"); v != "" {
		since, _ = strconv.ParseInt(v, 10, 64)
	}
	msgs, err := h.Msgs.List(r.Context(), conv.ID, since, limit)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(msgs, len(msgs), limit, 0))
	return nil
}

// PostMessage sends an agent message through REST.
func (h *Handlers) PostMessage(w http.ResponseWriter, r *http.Request) error {
	agent, err := h.callerAgent(r)
	if err != nil {
		return err
	}
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	var in struct {
		Content         string   `json:"content"`
		ContentType     string   `json:"content_type"`
		ClientMessageID string   `json:"client_message_id"`
		Attachments     []string `json:"attachments"`
		IsInternal      bool     `json:"is_internal"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	msg, duplicate, err := h.Msgs.Send(r.Context(), services.SendInput{
		ConversationID:  conv.ID,
		SenderType:      models.SenderAgent,
		SenderID:        agent.ID,
		SenderName:      agent.Name,
		Content:         in.Content,
		ContentType:     in.ContentType,
		ClientMessageID: in.ClientMessageID,
		Attachments:     in.Attachments,
		IsInternal:      in.IsInternal,
	})
	if err != nil {
		return err
	}
	if conv.AssignedAgentID == nil && !in.IsInternal {
		_ = h.Convs.Assign(r.Context(), conv.ID, agent.ID, agent.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": msg, "duplicate": duplicate})
	return nil
}

// AddNote stores an internal note on the conversation.
func (h *Handlers) AddNote(w http.ResponseWriter, r *http.Request) error {
	agent, err := h.callerAgent(r)
	if err != nil {
		return err
	}
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	var in struct {
		Content string `json:"content"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	msg, _, err := h.Msgs.Send(r.Context(), services.SendInput{
		ConversationID: conv.ID,
		SenderType:     models.SenderAgent,
		SenderID:       agent.ID,
		SenderName:     agent.Name,
		Content:        in.Content,
		IsInternal:     true,
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, msg)
	return nil
}

// MarkConversationRead clears the agent unread counter.
func (h *Handlers) MarkConversationRead(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	var in struct {
		UpToMessageID string `json:"up_to_message_id"`
	}
	if err := bind(r, &in); err != nil && !strings.Contains(err.Error(), "خالی") {
		return err
	}
	if err := h.Repo.Messages.MarkRead(r.Context(), conv.ID, in.UpToMessageID, "agent"); err != nil {
		return apierr.Internal(err)
	}
	if err := h.Repo.Conversations.ClearUnread(r.Context(), conv.ID, "agent"); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// SearchMessages searches the bounded decrypted window.
func (h *Handlers) SearchMessages(w http.ResponseWriter, r *http.Request) error {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		return apierr.BadRequest("عبارت جستجو لازم است")
	}
	limit, _ := parsePage(r)
	hits, err := h.Search.Messages(r.Context(), h.scopeProject(r), q, limit)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(hits, len(hits), limit, 0))
	return nil
}
