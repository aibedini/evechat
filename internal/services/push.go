package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/integrations/fcm"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// Push types understood by the Android app. All other values make the app
// perform a full sync.
const (
	pushTypeMessageReceived    = "message.received"
	pushTypeConversationNew    = "conversation.new"
	pushTypeConversationUpdate = "conversation.updated"
	pushTypeConversationDone   = "conversation.resolved"
	pushTypeAgentAssigned      = "agent.assigned"
)

// PushChannel turns notification events into data-only FCM messages. It keeps
// the recipient selection in one place so it mirrors the socket rules:
// unassigned conversations go to every active agent, assigned ones only to the
// owner, and agent/internal/system messages are never pushed.
type PushChannel struct {
	sender fcm.Sender
	repo   *repo.Repo
	log    zerolog.Logger
}

// NewPushChannel builds the channel.
func NewPushChannel(sender fcm.Sender, r *repo.Repo, log zerolog.Logger) *PushChannel {
	return &PushChannel{sender: sender, repo: r, log: log}
}

// Name implements Channel.
func (c *PushChannel) Name() string { return "fcm" }

// Notify implements Channel.
func (c *PushChannel) Notify(ctx context.Context, ev Event) error {
	if c.sender == nil || c.repo == nil {
		return nil
	}
	switch ev.Name {
	case EventMessageReceived:
		if ev.Str("sender_type") != string(models.SenderVisitor) {
			return nil
		}
		if internal, _ := ev.Data["is_internal"].(bool); internal {
			return nil
		}
		conv, err := c.conversation(ctx, ev.ConversationID)
		if err != nil || conv == nil {
			return err
		}
		devices, err := c.devicesForConversation(ctx, conv)
		if err != nil {
			return err
		}
		data := c.baseData(ev, pushTypeMessageReceived)
		if id := ev.Str("message_id"); id != "" {
			data["message_id"] = id
		}
		data["title"] = c.visitorName(ctx, conv, ev)
		if body := preview(ev.Str("content")); body != "" {
			data["body"] = body
		}
		c.send(ctx, devices, data, fcm.Options{Priority: "HIGH", TTL: "3600s"}, pushTypeMessageReceived)

	case EventChatStarted:
		conv, err := c.conversation(ctx, ev.ConversationID)
		if err != nil || conv == nil {
			return err
		}
		devices, err := c.devicesForConversation(ctx, conv)
		if err != nil {
			return err
		}
		data := c.baseData(ev, pushTypeConversationNew)
		if id := ev.Str("message_id"); id != "" {
			data["message_id"] = id
		}
		data["title"] = c.visitorName(ctx, conv, ev)
		if body := preview(ev.Str("message")); body != "" {
			data["body"] = body
		}
		c.send(ctx, devices, data, fcm.Options{Priority: "HIGH", TTL: "3600s"}, pushTypeConversationNew)

	case EventChatAssigned:
		agentID := ev.Str("agent_id")
		if agentID == "" {
			return nil
		}
		conv, err := c.conversation(ctx, ev.ConversationID)
		if err != nil || conv == nil {
			return err
		}
		assigned, err := c.repo.Push.ListByAgent(ctx, agentID)
		if err != nil {
			return err
		}
		data := c.baseData(ev, pushTypeAgentAssigned)
		data["title"] = c.visitorName(ctx, conv, ev)
		data["body"] = "گفتگو به شما ارجاع شد"
		c.send(ctx, assigned, data, normalOptions(ev.ConversationID), pushTypeAgentAssigned)

		others, err := c.repo.Push.ListByProjectExceptAgent(ctx, ev.ProjectID, agentID)
		if err != nil {
			return err
		}
		update := c.baseData(ev, pushTypeConversationUpdate)
		update["title"] = c.visitorName(ctx, conv, ev)
		update["body"] = "گفتگو به کارشناس دیگری ارجاع شد"
		c.send(ctx, others, update, normalOptions(ev.ConversationID), pushTypeConversationUpdate)

	case EventChatResolved:
		conv, err := c.conversation(ctx, ev.ConversationID)
		if err != nil || conv == nil || conv.AssignedAgentID == nil {
			return err
		}
		devices, err := c.repo.Push.ListByAgent(ctx, *conv.AssignedAgentID)
		if err != nil {
			return err
		}
		data := c.baseData(ev, pushTypeConversationDone)
		data["title"] = c.visitorName(ctx, conv, ev)
		data["body"] = "گفتگو بسته شد"
		c.send(ctx, devices, data, normalOptions(ev.ConversationID), pushTypeConversationDone)

	case EventConversationReopened:
		conv, err := c.conversation(ctx, ev.ConversationID)
		if err != nil || conv == nil || conv.AssignedAgentID == nil {
			return err
		}
		devices, err := c.repo.Push.ListByAgent(ctx, *conv.AssignedAgentID)
		if err != nil {
			return err
		}
		data := c.baseData(ev, pushTypeConversationUpdate)
		data["title"] = c.visitorName(ctx, conv, ev)
		data["body"] = "گفتگو دوباره باز شد"
		c.send(ctx, devices, data, normalOptions(ev.ConversationID), pushTypeConversationUpdate)
	}
	return nil
}

func (c *PushChannel) conversation(ctx context.Context, id string) (*models.Conversation, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil
	}
	conv, err := c.repo.Conversations.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("loading conversation for push: %w", err)
	}
	return conv, nil
}

func (c *PushChannel) devicesForConversation(ctx context.Context, conv *models.Conversation) ([]*models.PushDevice, error) {
	if conv.AssignedAgentID != nil {
		return c.repo.Push.ListByAgent(ctx, *conv.AssignedAgentID)
	}
	return c.repo.Push.ListByProject(ctx, conv.ProjectID)
}

func (c *PushChannel) visitorName(ctx context.Context, conv *models.Conversation, ev Event) string {
	name := strings.TrimSpace(ev.Str("visitor_name"))
	if name == "" && conv != nil && conv.VisitorID != "" {
		if v, err := c.repo.Visitors.GetByID(ctx, conv.VisitorID); err == nil {
			name = strings.TrimSpace(v.DisplayName())
		}
	}
	if name == "" {
		name = strings.TrimSpace(ev.Str("sender_name"))
	}
	if name == "" {
		name = "بازدیدکننده"
	}
	return name
}

func (c *PushChannel) baseData(ev Event, typ string) map[string]string {
	at := ev.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return map[string]string{
		"source":          "evechat",
		"v":               "1",
		"type":            typ,
		"conversation_id": ev.ConversationID,
		"sent_at":         strconv.FormatInt(at.UnixMilli(), 10),
	}
}

func normalOptions(conversationID string) fcm.Options {
	return fcm.Options{Priority: "NORMAL", TTL: "600s", CollapseKey: conversationID}
}

func (c *PushChannel) send(ctx context.Context, devices []*models.PushDevice, data map[string]string, opts fcm.Options, typ string) {
	if len(devices) == 0 {
		return
	}
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, d := range devices {
		if d == nil || strings.TrimSpace(d.Token) == "" {
			continue
		}
		wg.Add(1)
		go func(d *models.PushDevice) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			payload := make(map[string]string, len(data)+1)
			for k, v := range data {
				payload[k] = v
			}
			payload["agent_id"] = d.AgentID
			err := c.sender.Send(ctx, fcm.Message{Token: d.Token, Data: payload, Options: opts})
			if err != nil {
				var fe *fcm.Error
				if errors.As(err, &fe) && fe.InvalidToken() {
					if derr := c.repo.Push.DeleteByToken(ctx, d.Token); derr != nil {
						c.log.Warn().Err(derr).Str("device_id", d.DeviceID).Msg("removing invalid FCM token failed")
					}
					c.log.Info().Str("device_id", d.DeviceID).Str("agent_id", d.AgentID).Str("type", typ).Msg("invalid FCM token removed")
					return
				}
				c.log.Warn().Err(err).Str("device_id", d.DeviceID).Str("agent_id", d.AgentID).Str("type", typ).Msg("FCM push failed")
				return
			}
			if terr := c.repo.Push.TouchSuccess(ctx, d.DeviceID); terr != nil {
				c.log.Debug().Err(terr).Str("device_id", d.DeviceID).Msg("recording FCM success failed")
			}
		}(d)
	}
	wg.Wait()
}

func preview(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	const max = 160
	r := []rune(s)
	if len(r) > max {
		return strings.TrimSpace(string(r[:max])) + "…"
	}
	return s
}
