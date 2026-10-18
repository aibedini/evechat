package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// sqliteTimeLayout is the canonical textual timestamp format used everywhere.
// Keeping a fixed UTC layout makes comparisons against CURRENT_TIMESTAMP and
// between bound parameters lexicographically correct.
const sqliteTimeLayout = "2006-01-02 15:04:05"

// FormatTime renders a time for SQLite storage.
func FormatTime(t time.Time) string { return t.UTC().Format(sqliteTimeLayout) }

// NowStr is time.Now in storage format.
func NowStr() string { return FormatTime(time.Now()) }

// NullTime scans DATETIME columns regardless of whether the driver hands back
// a time.Time, a string or raw bytes.
type NullTime struct {
	Time  time.Time
	Valid bool
}

var timeLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02",
	time.RFC3339Nano,
	time.RFC3339,
}

// ParseTime parses any of the layouts SQLite/driver may emit.
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time")
	}
	for _, l := range timeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format %q", s)
}

func (n *NullTime) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		n.Time, n.Valid = time.Time{}, false
		return nil
	case time.Time:
		n.Time, n.Valid = v.UTC(), true
		return nil
	case string:
		if v == "" {
			n.Time, n.Valid = time.Time{}, false
			return nil
		}
		t, err := ParseTime(v)
		if err != nil {
			return err
		}
		n.Time, n.Valid = t, true
		return nil
	case []byte:
		return n.Scan(string(v))
	default:
		return fmt.Errorf("models.NullTime: cannot scan %T", src)
	}
}

func (n NullTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return FormatTime(n.Time), nil
}

// Ptr returns a pointer when valid, nil otherwise.
func (n NullTime) Ptr() *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time
	return &t
}

// JSONMap is a JSON object stored in a TEXT column.
type JSONMap map[string]any

func (j *JSONMap) Scan(src any) error {
	if src == nil {
		*j = JSONMap{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("models.JSONMap: cannot scan %T", src)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		*j = JSONMap{}
		return nil
	}
	m := JSONMap{}
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("models.JSONMap: %w", err)
	}
	*j = m
	return nil
}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Str reads a string value from the map.
func (j JSONMap) Str(key string) string {
	if v, ok := j[key].(string); ok {
		return v
	}
	return ""
}

// Bool reads a bool value from the map.
func (j JSONMap) Bool(key string) bool {
	if v, ok := j[key].(bool); ok {
		return v
	}
	return false
}

// StringList is a JSON array of strings.
type StringList []string

func (s *StringList) Scan(src any) error {
	if src == nil {
		*s = StringList{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("models.StringList: cannot scan %T", src)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		*s = StringList{}
		return nil
	}
	out := StringList{}
	if err := json.Unmarshal(b, &out); err != nil {
		return fmt.Errorf("models.StringList: %w", err)
	}
	*s = out
	return nil
}

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Contains reports whether v is in the list (case-insensitive).
func (s StringList) Contains(v string) bool {
	for _, x := range s {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}

// Note is a single ticket internal note.
type Note struct {
	ID       string    `json:"id"`
	AuthorID string    `json:"author_id"`
	Text     string    `json:"text"`
	At       time.Time `json:"at"`
}

// NoteList is a JSON array of ticket notes.
type NoteList []Note

func (n *NoteList) Scan(src any) error {
	if src == nil {
		*n = NoteList{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("models.NoteList: cannot scan %T", src)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		*n = NoteList{}
		return nil
	}
	raw := []struct {
		ID       string `json:"id"`
		AuthorID string `json:"author_id"`
		Text     string `json:"text"`
		At       string `json:"at"`
	}{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("models.NoteList: %w", err)
	}
	out := make(NoteList, 0, len(raw))
	for _, r := range raw {
		t, _ := ParseTime(r.At)
		out = append(out, Note{ID: r.ID, AuthorID: r.AuthorID, Text: r.Text, At: t})
	}
	*n = out
	return nil
}

func (n NoteList) Value() (driver.Value, error) {
	if n == nil {
		return "[]", nil
	}
	b, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// ---- Enums ----

// ConvStatus is the lifecycle state of a conversation.
type ConvStatus string

const (
	StatusPending  ConvStatus = "pending"
	StatusActive   ConvStatus = "active"
	StatusResolved ConvStatus = "resolved"
	StatusClosed   ConvStatus = "closed"
)

// SenderType identifies who wrote a message.
type SenderType string

const (
	SenderVisitor SenderType = "visitor"
	SenderAgent   SenderType = "agent"
	SenderBot     SenderType = "bot"
	SenderSystem  SenderType = "system"
)

// Role is the dashboard permission level.
type Role string

const (
	RoleSuperAdmin Role = "superadmin"
	RoleAdmin      Role = "admin"
	RoleAgent      Role = "agent"
)

// CanManageTeam reports whether the role may edit agents and settings.
func (r Role) CanManageTeam() bool { return r == RoleSuperAdmin || r == RoleAdmin }

// TicketStatus is the Kanban column of a ticket.
type TicketStatus string

const (
	TicketOpen       TicketStatus = "open"
	TicketInProgress TicketStatus = "in_progress"
	TicketWaiting    TicketStatus = "waiting"
	TicketResolved   TicketStatus = "resolved"
	TicketClosed     TicketStatus = "closed"
)

// TicketPriority is the urgency of a ticket.
type TicketPriority string

const (
	PriorityLow    TicketPriority = "low"
	PriorityNormal TicketPriority = "normal"
	PriorityHigh   TicketPriority = "high"
	PriorityUrgent TicketPriority = "urgent"
)

// ---- Tables ----

// Project is a tenant (one widget installation / customer).
type Project struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	APIKey    string     `json:"api_key"`
	SecretKey string     `json:"-"`
	Domains   StringList `json:"domains"`
	Settings  JSONMap    `json:"settings"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Agent is a dashboard user.
type Agent struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	Email            string     `json:"email"`
	PasswordHash     string     `json:"-"`
	Name             string     `json:"name"`
	AvatarURL        *string    `json:"avatar_url,omitempty"`
	Role             Role       `json:"role"`
	TelegramID       *string    `json:"telegram_id,omitempty"`
	MaxConversations int        `json:"max_conversations"`
	IsOnline         bool       `json:"is_online"`
	IsActive         bool       `json:"is_active"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Visitor is an end user of the widget.
type Visitor struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	Name             *string    `json:"name,omitempty"`
	Email            *string    `json:"email,omitempty"`
	Phone            *string    `json:"phone,omitempty"`
	IP               *string    `json:"ip,omitempty"`
	Country          *string    `json:"country,omitempty"`
	DeviceType       *string    `json:"device_type,omitempty"`
	OS               *string    `json:"os,omitempty"`
	Browser          *string    `json:"browser,omitempty"`
	UserAgent        *string    `json:"user_agent,omitempty"`
	Language         *string    `json:"language,omitempty"`
	EVEMobile        *string    `json:"eve_mobile,omitempty"`
	EVEUserID        *string    `json:"eve_user_id,omitempty"`
	EVEResellerID    *string    `json:"eve_reseller_id,omitempty"`
	SubscriptionInfo JSONMap    `json:"eve_subscription_info"`
	Metadata         JSONMap    `json:"metadata"`
	IsBanned         bool       `json:"is_banned"`
	BannedUntil      *time.Time `json:"banned_until,omitempty"`
	FirstSeenAt      time.Time  `json:"first_seen_at"`
	LastSeenAt       time.Time  `json:"last_seen_at"`
}

// DisplayName is the visitor name or a neutral fallback.
func (v *Visitor) DisplayName() string {
	if v == nil {
		return ""
	}
	if v.Name != nil && strings.TrimSpace(*v.Name) != "" {
		return *v.Name
	}
	if v.Phone != nil && *v.Phone != "" {
		return *v.Phone
	}
	if v.EVEMobile != nil && *v.EVEMobile != "" {
		return *v.EVEMobile
	}
	return ""
}

// RoutingRule matches a visitor to routing targets.
type RoutingRule struct {
	ID         string     `json:"id"`
	ProjectID  string     `json:"project_id"`
	Name       string     `json:"name"`
	Priority   int        `json:"priority"`
	Conditions JSONMap    `json:"conditions"`
	RouteTo    JSONMap    `json:"route_to"`
	AutoReply  *string    `json:"auto_reply,omitempty"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Conversation is one chat thread.
type Conversation struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	VisitorID        string     `json:"visitor_id"`
	AssignedAgentID  *string    `json:"assigned_agent_id,omitempty"`
	RoutingRuleID    *string    `json:"routing_rule_id,omitempty"`
	Status           ConvStatus `json:"status"`
	Channel          string     `json:"channel"`
	Subject          *string    `json:"subject,omitempty"`
	PageURL          *string    `json:"page_url,omitempty"`
	PageTitle        *string    `json:"page_title,omitempty"`
	Referrer         *string    `json:"referrer,omitempty"`
	RoutedTelegramID *string    `json:"routed_telegram_id,omitempty"`
	RoutedPhone      *string    `json:"routed_phone,omitempty"`
	TicketID         *string    `json:"ticket_id,omitempty"`
	UnreadAgent      int        `json:"unread_agent"`
	UnreadVisitor    int        `json:"unread_visitor"`
	LastMessageAt    *time.Time `json:"last_message_at,omitempty"`
	FirstResponseAt  *time.Time `json:"first_response_at,omitempty"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	CSATScore        *int       `json:"csat_score,omitempty"`
	CSATComment      *string    `json:"csat_comment,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// Message is an encrypted chat message.
type Message struct {
	ID                string     `json:"id"`
	ConversationID    string     `json:"conversation_id"`
	ClientMessageID   *string    `json:"client_message_id,omitempty"`
	SenderType        SenderType `json:"sender_type"`
	SenderID          *string    `json:"sender_id,omitempty"`
	SenderName        *string    `json:"sender_name,omitempty"`
	ContentType       string     `json:"content_type"`
	ContentEncrypted  string     `json:"-"`
	ContentIV         string     `json:"-"`
	KeyVersion        int        `json:"-"`
	Attachments       StringList `json:"attachments"`
	TelegramMessageID *int64     `json:"telegram_message_id,omitempty"`
	IsInternal        bool       `json:"is_internal"`
	IsRead            bool       `json:"is_read"`
	CreatedAt         time.Time  `json:"created_at"`

	// Content is filled in by the service layer after decryption.
	Content string `json:"content,omitempty"`
}

// VisitorSession is a widget session token.
type VisitorSession struct {
	Token        string    `json:"token"`
	VisitorID    string    `json:"visitor_id"`
	ProjectID    string    `json:"project_id"`
	ActiveConvID *string   `json:"active_conversation_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Ticket is a tracked issue, optionally converted from a conversation.
type Ticket struct {
	ID              string         `json:"id"`
	ProjectID       string         `json:"project_id"`
	ConversationID  *string        `json:"conversation_id,omitempty"`
	Title           string         `json:"title"`
	Description     *string        `json:"description,omitempty"`
	Status          TicketStatus   `json:"status"`
	Priority        TicketPriority `json:"priority"`
	AssignedAgentID *string        `json:"assigned_agent_id,omitempty"`
	VisitorID       *string        `json:"visitor_id,omitempty"`
	Tags            StringList     `json:"tags"`
	InternalNotes   NoteList       `json:"internal_notes"`
	DueAt           *time.Time     `json:"due_at,omitempty"`
	ResolvedAt      *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// KBArticle is a help-center article.
type KBArticle struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Content     string     `json:"content"`
	Excerpt     *string    `json:"excerpt,omitempty"`
	Tags        StringList `json:"tags"`
	IsPublished bool       `json:"is_published"`
	ViewCount   int        `json:"view_count"`
	CreatedBy   *string    `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// Rank is populated by FTS searches only.
	Rank float64 `json:"rank,omitempty"`
}

// Webhook is an outbound HTTP endpoint.
type Webhook struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	URL         string     `json:"url"`
	Events      StringList `json:"events"`
	Secret      string     `json:"secret,omitempty"`
	Description *string    `json:"description,omitempty"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// WebhookDelivery is one delivery attempt record.
type WebhookDelivery struct {
	ID           string     `json:"id"`
	WebhookID    string     `json:"webhook_id"`
	Event        string     `json:"event"`
	Payload      string     `json:"payload"`
	StatusCode   *int       `json:"status_code,omitempty"`
	ResponseBody *string    `json:"response_body,omitempty"`
	Attempt      int        `json:"attempt"`
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
	Error        *string    `json:"error,omitempty"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// CannedResponse is a saved reply.
type CannedResponse struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Shortcut  string    `json:"shortcut"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// Plan is a SaaS subscription plan.
type Plan struct {
	ID                     string    `json:"id"`
	Code                   string    `json:"code"`
	Name                   string    `json:"name"`
	PriceMonthly           int       `json:"price_monthly"`
	MaxAgents              int       `json:"max_agents"`
	MaxConversationsMonth  int       `json:"max_conversations_month"`
	MaxAITokensMonth       int       `json:"max_ai_tokens_month"`
	WidgetBranding         bool      `json:"widget_branding"`
	CreatedAt              time.Time `json:"created_at"`
}

// TenantPlan assigns a plan to a project.
type TenantPlan struct {
	ProjectID   string    `json:"project_id"`
	PlanCode    string    `json:"plan_code"`
	PeriodStart time.Time `json:"period_start"`
	IsActive    bool      `json:"is_active"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CustomDomain is a CNAME pointing to the chat host.
type CustomDomain struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Domain    string    `json:"domain"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

// UsageStat is one metered counter value.
type UsageStat struct {
	ProjectID string    `json:"project_id"`
	Metric    string    `json:"metric"`
	Period    string    `json:"period"`
	Value     int64     `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
