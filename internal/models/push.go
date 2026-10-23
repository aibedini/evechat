package models

import "time"

// PushDevice is one FCM registration token owned by an agent. DeviceID is the
// stable key supplied by the Android app; Token changes when FCM rotates it.
type PushDevice struct {
	DeviceID    string     `json:"device_id"`
	AgentID     string     `json:"agent_id"`
	ProjectID   string     `json:"project_id"`
	Token       string     `json:"-"`
	Platform    string     `json:"platform"`
	Provider    string     `json:"provider"`
	AppVersion  string     `json:"app_version"`
	AppBuild    int        `json:"app_build"`
	OSVersion   string     `json:"os_version"`
	DeviceModel string     `json:"device_model"`
	Locale      string     `json:"locale"`
	Timezone    string     `json:"timezone"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastSuccess *time.Time `json:"last_success_at,omitempty"`
}
