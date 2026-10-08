package client

import "encoding/json"

// Organization represents a Flaggr organization.
type Organization struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// OrgMember represents a member of an organization.
type OrgMember struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId"`
	UserID    string `json:"userId"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	// User is the member's user as the API lists and adds them (absent on role updates).
	User *OrgMemberUser `json:"user,omitempty"`
}

// OrgMemberUser is what the API shows of a member's user.
type OrgMemberUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// Project represents a Flaggr project.
type Project struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	Description    string          `json:"description,omitempty"`
	OrganizationID string          `json:"orgId,omitempty"`
	Settings       json.RawMessage `json:"settings,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
	CreatedBy      string          `json:"createdBy,omitempty"`
}

// Service represents a Flaggr service.
type Service struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	ProjectID   string   `json:"projectId"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	CreatedBy   string   `json:"createdBy,omitempty"`
}

// Flag represents a Flaggr feature flag.
type Flag struct {
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Type         string          `json:"type"`
	Enabled      bool            `json:"enabled"`
	DefaultValue json.RawMessage `json:"defaultValue"`
	ProjectID    string          `json:"projectId"`
	ServiceID    string          `json:"serviceId"`
	Environment  string          `json:"environment"`
	Tags         []string        `json:"tags,omitempty"`
	IsPublic     bool            `json:"isPublic,omitempty"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
	CreatedBy    string          `json:"createdBy,omitempty"`
}

// MetricSource represents a Flaggr metric source.
type MetricSource struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"projectId"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	Config        json.RawMessage `json:"config"`
	Enabled       bool            `json:"enabled"`
	LastCheckedAt string          `json:"lastCheckedAt,omitempty"`
	LastStatus    string          `json:"lastStatus,omitempty"`
	CreatedAt     string          `json:"createdAt"`
	UpdatedAt     string          `json:"updatedAt"`
	CreatedBy     string          `json:"createdBy,omitempty"`
}

// AlertChannel represents an alert notification channel.
type AlertChannel struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"projectId"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

// AlertRule represents an alert rule configuration.
type AlertRule struct {
	ID              string            `json:"id"`
	ProjectID       string            `json:"projectId"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	Severity        string            `json:"severity"`
	ConditionType   string            `json:"conditionType"`
	Threshold       float64           `json:"threshold"`
	WindowMinutes   int               `json:"windowMinutes"`
	Channels        []AlertChannelRef `json:"channels"`
	Enabled         bool              `json:"enabled"`
	CooldownMinutes int               `json:"cooldownMinutes"`
	CreatedBy       string            `json:"createdBy,omitempty"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt"`
}

// AlertChannelRef is a reference to an alert channel.
type AlertChannelRef struct {
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
}

// Environment represents a Flaggr project environment.
type Environment struct {
	ID          string               `json:"id"`
	ProjectID   string               `json:"projectId"`
	Slug        string               `json:"slug"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Color       string               `json:"color,omitempty"`
	Order       int                  `json:"order"`
	IsDefault   bool                 `json:"isDefault"`
	Settings    *EnvironmentSettings `json:"settings,omitempty"`
	CreatedBy   string               `json:"createdBy,omitempty"`
	CreatedAt   string               `json:"createdAt"`
	UpdatedAt   string               `json:"updatedAt"`
}

// EnvironmentSettings holds environment-level settings.
type EnvironmentSettings struct {
	DefaultFlagEnabled   *bool `json:"defaultFlagEnabled,omitempty"`
	RequireApproval      *bool `json:"requireApproval,omitempty"`
	ProtectedEnvironment *bool `json:"protectedEnvironment,omitempty"`
}

// --- API Response Wrappers ---

type ProjectListResponse struct {
	Projects []Project `json:"projects"`
}

type ServiceListResponse struct {
	Services []Service `json:"services"`
	Total    int       `json:"total"`
}

type FlagListResponse struct {
	Flags []Flag `json:"flags"`
	Total int    `json:"total"`
}

type MetricSourceListResponse struct {
	Sources []MetricSource `json:"sources"`
}

type AlertChannelListResponse struct {
	Channels []AlertChannel `json:"channels"`
}

type AlertRuleListResponse struct {
	Rules []AlertRule `json:"rules"`
}

type EnvironmentListResponse struct {
	Environments []Environment `json:"environments"`
}

type OrganizationListResponse struct {
	Organizations []Organization `json:"organizations"`
}

type OrgMemberListResponse struct {
	Members []OrgMember `json:"members"`
}
