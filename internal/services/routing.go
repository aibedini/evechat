package services

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// Decision is the outcome of routing a conversation.
type Decision struct {
	RuleID     string
	AgentID    string
	TelegramID string
	Phone      string
	AutoReply  string
	Fallback   bool
}

// RoutingService matches routing rules and falls back to agent capacity.
type RoutingService struct {
	repo *repo.Repo
	log  zerolog.Logger
	now  func() time.Time
}

// NewRoutingService builds the service.
func NewRoutingService(r *repo.Repo, log zerolog.Logger) *RoutingService {
	return &RoutingService{repo: r, log: log, now: func() time.Time { return time.Now().UTC() }}
}

// MatchContext is everything a rule condition may inspect.
type MatchContext struct {
	PageURL         string
	PageTitle       string
	DeviceType      string
	Language        string
	ResellerID      string
	Metadata        models.JSONMap
	Subscription    models.JSONMap
	InBusinessHours bool
	Now             time.Time
}

// MatchContextOf builds a context from a visitor and conversation.
func MatchContextOf(v *models.Visitor, conv *models.Conversation, settings models.JSONMap, now time.Time) MatchContext {
	mc := MatchContext{Now: now, Metadata: models.JSONMap{}, Subscription: models.JSONMap{}}
	if v != nil {
		if v.DeviceType != nil {
			mc.DeviceType = *v.DeviceType
		}
		if v.Language != nil {
			mc.Language = *v.Language
		}
		if v.EVEResellerID != nil {
			mc.ResellerID = *v.EVEResellerID
		}
		if v.Metadata != nil {
			mc.Metadata = v.Metadata
		}
		if v.SubscriptionInfo != nil {
			mc.Subscription = v.SubscriptionInfo
		}
	}
	if conv != nil {
		if conv.PageURL != nil {
			mc.PageURL = *conv.PageURL
		}
		if conv.PageTitle != nil {
			mc.PageTitle = *conv.PageTitle
		}
	}
	mc.InBusinessHours = InBusinessHours(settings, now)
	return mc
}

// Decide returns the first matching rule, or a capacity-based fallback.
func (s *RoutingService) Decide(ctx context.Context, conv *models.Conversation, v *models.Visitor) (Decision, error) {
	var settings models.JSONMap
	project, err := s.repo.Projects.GetByID(ctx, conv.ProjectID)
	if err == nil {
		settings = project.Settings
	}
	now := s.now()
	mc := MatchContextOf(v, conv, settings, now)

	rules, err := s.repo.Rules.List(ctx, conv.ProjectID, true)
	if err != nil {
		return Decision{}, fmt.Errorf("listing routing rules: %w", err)
	}
	for _, rule := range rules {
		if !matchConditions(rule.Conditions, mc) {
			continue
		}
		d := Decision{RuleID: rule.ID}
		d.AgentID = strOf(rule.RouteTo["agent_id"])
		d.TelegramID = strOf(rule.RouteTo["telegram_id"])
		d.Phone = strOf(rule.RouteTo["phone"])
		if rule.AutoReply != nil {
			d.AutoReply = *rule.AutoReply
		}
		if d.AgentID == "" && d.TelegramID == "" && d.Phone == "" {
			// A matching rule with no target still selects the rule (for
			// auto-replies) but must not stop the capacity fallback.
			d.AgentID = ""
		}
		return d, nil
	}

	return s.fallback(ctx, conv, settings)
}

func (s *RoutingService) fallback(ctx context.Context, conv *models.Conversation, settings models.JSONMap) (Decision, error) {
	d := Decision{Fallback: true}
	agents, err := s.repo.Agents.List(ctx, conv.ProjectID)
	if err != nil {
		return d, fmt.Errorf("listing agents for fallback: %w", err)
	}
	load, err := s.repo.Agents.ActiveLoad(ctx, conv.ProjectID)
	if err != nil {
		return d, fmt.Errorf("reading agent load: %w", err)
	}
	best := ""
	bestLoad := -1
	anyOnline := ""
	for _, a := range agents {
		if !a.IsOnline || !a.IsActive {
			continue
		}
		if anyOnline == "" {
			anyOnline = a.ID
		}
		l := load[a.ID]
		limit := a.MaxConversations
		if limit <= 0 {
			limit = 10
		}
		if l >= limit {
			continue
		}
		if bestLoad == -1 || l < bestLoad {
			best, bestLoad = a.ID, l
		}
	}
	if best != "" {
		d.AgentID = best
		return d, nil
	}
	if anyOnline != "" {
		d.AgentID = anyOnline
		return d, nil
	}
	if tg := strOf(settings["default_telegram_id"]); tg != "" {
		d.TelegramID = tg
		return d, nil
	}
	s.log.Debug().Str("conv_id", conv.ID).Msg("no agent available, conversation stays pending")
	return d, nil
}

// ---- pure matching logic (unit tested) ----

func matchConditions(cond models.JSONMap, mc MatchContext) bool {
	if cond == nil {
		return false
	}
	if v, ok := cond["domains"]; ok {
		list := asStringList(v)
		if len(list) > 0 {
			host := hostOf(mc.PageURL)
			ok := false
			for _, d := range list {
				d = strings.ToLower(strings.TrimSpace(d))
				if d == "" {
					continue
				}
				if host == d || strings.HasSuffix(host, "."+d) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		}
	}
	if v, ok := cond["pages"]; ok {
		list := asStringList(v)
		if len(list) > 0 {
			path := pathOf(mc.PageURL)
			matched := false
			for _, pat := range list {
				if globMatch(pat, path) || globMatch(pat, mc.PageURL) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
	}
	if v, ok := cond["eve_reseller_id"]; ok {
		list := asStringList(v)
		if len(list) > 0 && !containsFold(list, mc.ResellerID) {
			return false
		}
	}
	if v, ok := cond["device_types"]; ok {
		list := asStringList(v)
		if len(list) > 0 && !containsFold(list, mc.DeviceType) {
			return false
		}
	}
	if v, ok := cond["languages"]; ok {
		list := asStringList(v)
		if len(list) > 0 {
			lang := strings.ToLower(mc.Language)
			matched := false
			for _, l := range list {
				l = strings.ToLower(l)
				if lang == l || strings.HasPrefix(lang, l+"-") || strings.HasPrefix(lang, l+"_") {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
	}
	if v, ok := cond["has_subscription"]; ok {
		want, ok2 := asBool(v)
		if ok2 && subscriptionActive(mc.Subscription, mc.Now) != want {
			return false
		}
	}
	if v, ok := cond["business_hours"]; ok {
		want, ok2 := asBool(v)
		if ok2 && mc.InBusinessHours != want {
			return false
		}
	}
	if v, ok := cond["variables"]; ok {
		if vars, ok2 := v.(map[string]any); ok2 {
			for key, raw := range vars {
				list := asStringList(raw)
				if len(list) == 0 {
					continue
				}
				actual := strOf(mc.Metadata[key])
				if actual == "" || !containsFold(list, actual) {
					return false
				}
			}
		}
	}
	return true
}

func asStringList(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return []string{t}
	case nil:
		return nil
	default:
		return nil
	}
}

func asBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		b, err := strconv.ParseBool(t)
		return b, err == nil
	default:
		return false, false
	}
}

func strOf(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func containsFold(list []string, v string) bool {
	for _, x := range list {
		if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}

func hostOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(raw)
	}
	return strings.ToLower(u.Hostname())
}

func pathOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Path == "" {
		return "/"
	}
	return u.Path
}

// globMatch supports '*' only, which is all the condition DSL promises.
func globMatch(pattern, s string) bool {
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(s[pos:], part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		pos += idx + len(part)
	}
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(s, last)
}

func subscriptionActive(sub models.JSONMap, now time.Time) bool {
	if len(sub) == 0 {
		return false
	}
	plan := strOf(sub["plan"])
	if plan == "" {
		plan = strOf(sub["subscription_plan"])
	}
	expiry := strOf(sub["expiry"])
	if expiry == "" {
		expiry = strOf(sub["subscription_expiry"])
	}
	if expiry == "" {
		expiry = strOf(sub["expires_at"])
	}
	if expiry == "" {
		// A plan without an expiry is treated as active (lifetime plan).
		return plan != ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02", "2006/01/02"} {
		if t, err := time.Parse(layout, expiry); err == nil {
			return t.After(now) && plan != ""
		}
	}
	if ts, err := strconv.ParseInt(expiry, 10, 64); err == nil {
		unit := time.Unix(ts, 0)
		if ts > 1e12 {
			unit = time.UnixMilli(ts)
		}
		return unit.After(now) && plan != ""
	}
	return plan != ""
}

// InBusinessHours evaluates project business-hour settings.
// When business hours are not configured the service is treated as always open.
func InBusinessHours(settings models.JSONMap, now time.Time) bool {
	raw, ok := settings["business_hours"]
	if !ok {
		return true
	}
	bh, ok := raw.(map[string]any)
	if !ok {
		return true
	}
	enabled, _ := asBool(bh["enabled"])
	if !enabled {
		return true
	}
	loc := time.FixedZone("+03:30", 3*3600+1800)
	if tz := strOf(bh["timezone"]); tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	local := now.In(loc)
	if days := asStringList(bh["days"]); len(days) > 0 {
		wd := strconv.Itoa(int(local.Weekday()))
		if !containsFold(days, wd) {
			return false
		}
	}
	start := strOf(bh["start"])
	end := strOf(bh["end"])
	if start == "" || end == "" {
		return true
	}
	minutes := local.Hour()*60 + local.Minute()
	s, ok1 := parseHHMM(start)
	e, ok2 := parseHHMM(end)
	if !ok1 || !ok2 {
		return true
	}
	if s <= e {
		return minutes >= s && minutes < e
	}
	// Overnight range, e.g. 21:00 -> 03:00.
	return minutes >= s || minutes < e
}

func parseHHMM(s string) (int, bool) {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}
