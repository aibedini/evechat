package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/i18n/email"
	"github.com/evechat/evechat/internal/integrations/smtp"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// EmailChannel delivers notification events over SMTP.
type EmailChannel struct {
	client  *smtp.Client
	repo    *repo.Repo
	baseURL string
	log     zerolog.Logger
}

// NewEmailChannel builds the channel.
func NewEmailChannel(client *smtp.Client, r *repo.Repo, baseURL string, log zerolog.Logger) *EmailChannel {
	return &EmailChannel{client: client, repo: r, baseURL: strings.TrimRight(baseURL, "/"), log: log}
}

// Name implements Channel.
func (c *EmailChannel) Name() string { return "email" }

// Notify sends the relevant event to the responsible agent.
func (c *EmailChannel) Notify(ctx context.Context, ev Event) error {
	if c.client == nil {
		return nil
	}
	switch ev.Name {
	case EventChatStarted:
		agent := c.agentFor(ctx, ev)
		if agent == nil || agent.Email == "" {
			return nil
		}
		msg := email.NewConversation(ev.Str("visitor_name"), ev.Str("page_title"), ev.Str("message"), c.baseURL, ev.ConversationID)
		return c.client.Send(ctx, agent.Email, msg.Subject, msg.HTML, msg.Text)
	case EventCSATReceived:
		agent := c.agentFor(ctx, ev)
		if agent == nil || agent.Email == "" {
			return nil
		}
		score := 0
		if v, ok := ev.Data["score"].(int); ok {
			score = v
		}
		msg := email.CSATReceived(ev.Str("visitor_name"), score, ev.Str("comment"))
		return c.client.Send(ctx, agent.Email, msg.Subject, msg.HTML, msg.Text)
	}
	return nil
}

// SendPasswordReset delivers the reset link.
func (c *EmailChannel) SendPasswordReset(ctx context.Context, to, name, link string, minutes int) error {
	if c.client == nil {
		return fmt.Errorf("email channel is not configured")
	}
	msg := email.PasswordReset(name, link, minutes)
	return c.client.Send(ctx, to, msg.Subject, msg.HTML, msg.Text)
}

// agentFor resolves the responsible agent for an event.
func (c *EmailChannel) agentFor(ctx context.Context, ev Event) *models.Agent {
	if id, ok := ev.Data["agent_id"].(string); ok && id != "" {
		if a, err := c.repo.Agents.GetByID(ctx, id); err == nil {
			return a
		}
	}
	if ev.ConversationID != "" {
		if conv, err := c.repo.Conversations.GetByID(ctx, ev.ConversationID); err == nil && conv.AssignedAgentID != nil {
			if a, err := c.repo.Agents.GetByID(ctx, *conv.AssignedAgentID); err == nil {
				return a
			}
		}
	}
	agents, err := c.repo.Agents.List(ctx, ev.ProjectID)
	if err != nil {
		return nil
	}
	for _, a := range agents {
		if a.IsActive && a.Email != "" {
			return a
		}
	}
	return nil
}
