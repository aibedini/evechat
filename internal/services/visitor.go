package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// VisitorService creates and resumes widget visitors.
type VisitorService struct {
	repo       *repo.Repo
	sessionTTL time.Duration
	log        zerolog.Logger
}

// NewVisitorService builds the service.
func NewVisitorService(r *repo.Repo, sessionTTL time.Duration, log zerolog.Logger) *VisitorService {
	if sessionTTL <= 0 {
		sessionTTL = 30 * 24 * time.Hour
	}
	return &VisitorService{repo: r, sessionTTL: sessionTTL, log: log}
}

// PageInfo describes the page the widget is open on.
type PageInfo struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Referrer string `json:"referrer"`
}

// VisitorInput is the visitor identity sent by the host page.
type VisitorInput struct {
	Name          string         `json:"name"`
	Email         string         `json:"email"`
	Mobile        string         `json:"mobile"`
	EVEUserID     string         `json:"eve_user_id"`
	EVEResellerID string         `json:"eve_reseller_id"`
	EVEMobile     string         `json:"eve_mobile"`
	Metadata      models.JSONMap `json:"metadata"`
	Subscription  models.JSONMap `json:"subscription"`
}

// SessionRequest is the input of IdentifyOrCreate.
type SessionRequest struct {
	ProjectID string
	Token     string
	Page      PageInfo
	Visitor   VisitorInput
	IP        string
	UserAgent string
	Language  string
}

// IdentifyOrCreate resumes an existing session or creates a visitor.
func (s *VisitorService) IdentifyOrCreate(ctx context.Context, req SessionRequest) (*models.Visitor, *models.VisitorSession, error) {
	if req.Token != "" {
		sess, err := s.repo.Sessions.Get(ctx, req.Token)
		if err == nil && sess.ProjectID == req.ProjectID {
			v, gerr := s.repo.Visitors.GetByID(ctx, sess.VisitorID)
			if gerr == nil {
				s.merge(v, req)
				v.LastSeenAt = time.Now().UTC()
				if err := s.repo.Visitors.Upsert(ctx, v); err != nil {
					return nil, nil, err
				}
				_ = s.repo.Sessions.Touch(ctx, sess.Token, s.sessionTTL)
				sess.ExpiresAt = time.Now().UTC().Add(s.sessionTTL)
				return v, sess, nil
			}
		}
	}

	device, osName, browser := ParseUA(req.UserAgent)
	v := &models.Visitor{
		ID:               id.NewPrefixed(id.PrefixVisitor),
		ProjectID:        req.ProjectID,
		IP:               nilIfEmpty(req.IP),
		DeviceType:       nilIfEmpty(device),
		OS:               nilIfEmpty(osName),
		Browser:          nilIfEmpty(browser),
		UserAgent:        nilIfEmpty(req.UserAgent),
		Language:         nilIfEmpty(req.Language),
		Metadata:         models.JSONMap{},
		SubscriptionInfo: models.JSONMap{},
	}
	s.merge(v, req)
	if err := s.repo.Visitors.Upsert(ctx, v); err != nil {
		return nil, nil, fmt.Errorf("creating visitor: %w", err)
	}
	sess, err := s.repo.Sessions.Create(ctx, v.ID, req.ProjectID, id.Token(32), s.sessionTTL)
	if err != nil {
		return nil, nil, err
	}
	s.log.Info().Str("visitor_id", v.ID).Str("project_id", req.ProjectID).Msg("visitor created")
	return v, sess, nil
}

// merge copies non-empty incoming identity fields onto the visitor.
func (s *VisitorService) merge(v *models.Visitor, req SessionRequest) {
	in := req.Visitor
	if in.Name != "" {
		v.Name = &in.Name
	}
	if in.Email != "" {
		v.Email = &in.Email
	}
	mobile := in.Mobile
	if mobile == "" {
		mobile = in.EVEMobile
	}
	if mobile != "" {
		v.Phone = &mobile
	}
	if in.EVEMobile != "" {
		v.EVEMobile = &in.EVEMobile
	}
	if in.EVEUserID != "" {
		v.EVEUserID = &in.EVEUserID
	}
	if in.EVEResellerID != "" {
		v.EVEResellerID = &in.EVEResellerID
	}
	if v.Metadata == nil {
		v.Metadata = models.JSONMap{}
	}
	for k, val := range in.Metadata {
		v.Metadata[k] = val
	}
	if in.Subscription != nil && len(in.Subscription) > 0 {
		if v.SubscriptionInfo == nil {
			v.SubscriptionInfo = models.JSONMap{}
		}
		for k, val := range in.Subscription {
			v.SubscriptionInfo[k] = val
		}
	}
	if req.IP != "" && v.IP == nil {
		v.IP = &req.IP
	}
	if req.Language != "" && v.Language == nil {
		v.Language = &req.Language
	}
	if ua := req.UserAgent; ua != "" && v.UserAgent == nil {
		v.UserAgent = &ua
	}
	if v.DeviceType == nil || v.OS == nil || v.Browser == nil {
		device, osName, browser := ParseUA(req.UserAgent)
		if v.DeviceType == nil {
			v.DeviceType = nilIfEmpty(device)
		}
		if v.OS == nil {
			v.OS = nilIfEmpty(osName)
		}
		if v.Browser == nil {
			v.Browser = nilIfEmpty(browser)
		}
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ParseUA is a dependency-free user-agent classifier. Order matters: Edge and
// Opera and Samsung ship Chrome tokens, iOS Chrome ships Safari tokens.
func ParseUA(ua string) (deviceType, osName, browser string) {
	l := strings.ToLower(ua)

	switch {
	case strings.Contains(l, "ipad"), strings.Contains(l, "tablet"), strings.Contains(l, "kindle"), strings.Contains(l, "silk/"):
		deviceType = "tablet"
	case strings.Contains(l, "android") && !strings.Contains(l, "mobile"):
		deviceType = "tablet"
	case strings.Contains(l, "mobile"), strings.Contains(l, "iphone"), strings.Contains(l, "ipod"),
		strings.Contains(l, "android"), strings.Contains(l, "windows phone"):
		deviceType = "mobile"
	default:
		deviceType = "desktop"
	}

	switch {
	case strings.Contains(l, "windows"):
		osName = "Windows"
	case strings.Contains(l, "iphone"), strings.Contains(l, "ipad"), strings.Contains(l, "ipod"):
		osName = "iOS"
	case strings.Contains(l, "mac os x"), strings.Contains(l, "macintosh"):
		osName = "macOS"
	case strings.Contains(l, "android"):
		osName = "Android"
	case strings.Contains(l, "cros"):
		osName = "ChromeOS"
	case strings.Contains(l, "linux"):
		osName = "Linux"
	default:
		osName = "نامشخص"
	}

	switch {
	case strings.Contains(l, "edg/"), strings.Contains(l, "edge/"):
		browser = "Edge"
	case strings.Contains(l, "opr/"), strings.Contains(l, "opera"):
		browser = "Opera"
	case strings.Contains(l, "samsungbrowser"):
		browser = "Samsung"
	case strings.Contains(l, "firefox/"), strings.Contains(l, "fxios"):
		browser = "Firefox"
	case strings.Contains(l, "chrome/"), strings.Contains(l, "crios"):
		browser = "Chrome"
	case strings.Contains(l, "safari/"):
		browser = "Safari"
	default:
		browser = "نامشخص"
	}
	return deviceType, osName, browser
}

// MaskPhone hides all but the first 6 and last 4 digits.
func MaskPhone(p string) string {
	if len(p) <= 10 {
		return "***"
	}
	return p[:6] + "***" + p[len(p)-4:]
}

var nonDigit = regexp.MustCompile(`\D`)

// NormalizePhone strips formatting characters.
func NormalizePhone(p string) string {
	return nonDigit.ReplaceAllString(p, "")
}
