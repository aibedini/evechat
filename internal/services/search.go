package services

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// SearchService implements message and knowledge-base search.
//
// LIMITATION: messages are encrypted at rest, so FTS5 cannot index them.
// Message search therefore decrypts a bounded window (maxMessageWindow recent
// messages across the recent conversations of a project). This is documented
// behaviour, not an oversight: the window keeps memory bounded and never
// exposes plaintext to the database.
type SearchService struct {
	repo *repo.Repo
	msgs *MessageService
	log  zerolog.Logger
}

const (
	maxSearchConversations = 500
	maxMessageWindow       = 2000
)

// NewSearchService builds the search service.
func NewSearchService(r *repo.Repo, msgs *MessageService, log zerolog.Logger) *SearchService {
	return &SearchService{repo: r, msgs: msgs, log: log}
}

// MessageHit is one search result.
type MessageHit struct {
	*models.Message
	ConversationID string `json:"conversation_id"`
	Snippet        string `json:"snippet"`
}

// Messages searches a bounded window of decrypted messages.
func (s *SearchService) Messages(ctx context.Context, projectID, query string, limit int) ([]*MessageHit, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []*MessageHit{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	convIDs, err := s.repo.Conversations.ConversationIDsOfProject(ctx, projectID, time.Now().UTC().AddDate(0, -6, 0), maxSearchConversations)
	if err != nil {
		return nil, err
	}
	window, err := s.repo.Messages.SearchWindow(ctx, convIDs, maxMessageWindow)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(q)
	out := make([]*MessageHit, 0, limit)
	for _, m := range window {
		if len(out) >= limit {
			break
		}
		if err := s.msgs.Decrypt(m); err != nil {
			continue
		}
		lower := strings.ToLower(m.Content)
		idx := strings.Index(lower, needle)
		if idx < 0 {
			continue
		}
		start := idx - 40
		if start < 0 {
			start = 0
		}
		end := idx + len(q) + 60
		if end > len(m.Content) {
			end = len(m.Content)
		}
		out = append(out, &MessageHit{
			Message:        m,
			ConversationID: m.ConversationID,
			Snippet:        m.Content[start:end],
		})
	}
	return out, nil
}

// KnowledeBase searches published articles with bm25 ranking.
func (s *SearchService) KnowledgeBase(ctx context.Context, projectID, query string, limit int) ([]*models.KBArticle, error) {
	return s.repo.KB.Search(ctx, projectID, query, limit)
}
