package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/i18n"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// AIConfig tunes the Phase 2 assistant.
type AIConfig struct {
	Enabled      bool
	APIKey       string
	Model        string
	MaxTokens    int
	HandoffAfter int
	BaseURL      string
}

// AIService answers visitor messages from the knowledge base. It never invents
// answers: when unsure it replies NEED_HUMAN and escalates to a human.
type AIService struct {
	repo   *repo.Repo
	msgs   *MessageService
	hub    *hub.Hub
	convs  *ConversationService
	cfg    AIConfig
	client *http.Client
	log    zerolog.Logger
}

// NewAIService builds the assistant.
func NewAIService(r *repo.Repo, msgs *MessageService, h *hub.Hub, convs *ConversationService, cfg AIConfig, log zerolog.Logger) *AIService {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Model == "" {
		cfg.Model = "claude-haiku-4-5"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 512
	}
	if cfg.HandoffAfter <= 0 {
		cfg.HandoffAfter = 3
	}
	return &AIService{repo: r, msgs: msgs, hub: h, convs: convs, cfg: cfg, log: log,
		client: &http.Client{Timeout: 12 * time.Second}}
}

// MaybeAnswer tries to answer an unassigned conversation.
func (s *AIService) MaybeAnswer(ctx context.Context, conv *models.Conversation, trigger *models.Message) error {
	if !s.cfg.Enabled || s.cfg.APIKey == "" || trigger == nil {
		return nil
	}
	if trigger.SenderType != models.SenderVisitor {
		return nil
	}
	fresh, err := s.repo.Conversations.GetByID(ctx, conv.ID)
	if err != nil {
		return err
	}
	if fresh.AssignedAgentID != nil {
		return nil // a human owns it now
	}

	turns, err := s.aiTurnsSinceHuman(ctx, conv.ID)
	if err == nil && turns >= s.cfg.HandoffAfter {
		return s.escalate(ctx, fresh, "turn-limit")
	}

	articles, err := s.repo.KB.Search(ctx, conv.ProjectID, trigger.Content, 4)
	if err != nil || len(articles) == 0 {
		return nil
	}

	answer, usage, err := s.ask(ctx, articles, trigger.Content)
	if err != nil {
		// Any failure escalates silently per the spec.
		return s.escalate(ctx, fresh, "ai-error")
	}
	if usage.Total() > 0 {
		period := time.Now().UTC().Format("2006-01")
		_ = s.repo.SaaS.IncrementUsage(ctx, conv.ProjectID, "ai_tokens", period, int64(usage.Total()))
	}

	if strings.Contains(answer, "NEED_HUMAN") {
		return s.escalate(ctx, fresh, "need-human")
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return s.escalate(ctx, fresh, "empty-answer")
	}
	msg, err := s.msgs.BotMessage(ctx, conv.ID, answer)
	if err != nil {
		return fmt.Errorf("storing ai answer: %w", err)
	}
	s.log.Info().Str("conv_id", conv.ID).Int("kb_hits", len(articles)).Int("tokens", usage.Total()).
		Str("message_id", msg.ID).Msg("ai answered")
	return nil
}

func (s *AIService) aiTurnsSinceHuman(ctx context.Context, convID string) (int, error) {
	msgs, err := s.repo.Messages.LatestN(ctx, convID, 40)
	if err != nil {
		return 0, err
	}
	turns := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		switch msgs[i].SenderType {
		case models.SenderAgent:
			return turns, nil
		case models.SenderBot:
			turns++
		}
	}
	return turns, nil
}

func (s *AIService) escalate(ctx context.Context, conv *models.Conversation, reason string) error {
	s.log.Info().Str("conv_id", conv.ID).Str("reason", reason).Msg("ai escalation")
	if _, err := s.msgs.SystemMessage(ctx, conv.ID, i18n.AIHandoff); err != nil {
		return err
	}
	s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
		"conversation_id": conv.ID,
		"ai_handoff":      true,
		"reason":          reason,
	}))
	return nil
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// TokenUsage is the metered token count of one assistant turn.
type TokenUsage struct {
	Input  int
	Output int
}

// Total sums input and output tokens.
func (u TokenUsage) Total() int { return u.Input + u.Output }

func (s *AIService) ask(ctx context.Context, articles []*models.KBArticle, question string) (string, TokenUsage, error) {
	var kb strings.Builder
	for i, a := range articles {
		fmt.Fprintf(&kb, "\n[مقاله %d] %s\n%s\n", i+1, a.Title, truncate(a.Content, 4000))
	}
	system := `تو دستیار پشتیبانی فارسی‌زبان هستی. فقط و فقط بر اساس مقاله‌های ارائه‌شده پاسخ بده.
اگر پاسخ در مقاله‌ها نبود یا مطمئن نبودی، تنها کلمه NEED_HUMAN را بنویس و هیچ چیز دیگری اضافه نکن.
پاسخ کوتاه، محترمانه و فارسی باشد. هرگز اطلاعاتی خارج از مقاله‌ها نساز.
مقاله‌ها:` + kb.String()

	body, err := json.Marshal(anthropicRequest{
		Model:     s.cfg.Model,
		MaxTokens: s.cfg.MaxTokens,
		System:    system,
		Messages:  []anthropicMessage{{Role: "user", Content: question}},
	})
	if err != nil {
		return "", TokenUsage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.cfg.BaseURL, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", TokenUsage{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", s.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", TokenUsage{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", TokenUsage{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", TokenUsage{}, fmt.Errorf("anthropic status %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var out anthropicResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", TokenUsage{}, err
	}
	if out.Error != nil {
		return "", TokenUsage{}, fmt.Errorf("anthropic error: %s", out.Error.Message)
	}
	var text strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	return text.String(), TokenUsage{Input: out.Usage.InputTokens, Output: out.Usage.OutputTokens}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
