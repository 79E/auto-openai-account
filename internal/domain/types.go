package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	MailboxStatusNew         = "new"
	MailboxStatusRegistering = "registering"
	MailboxStatusRegistered  = "registered"
	MailboxStatusLogining    = "logining"
	MailboxStatusAbnormal    = "abnormal"

	SMSConfigTypeProvider = "provider"
	SMSConfigTypePool     = "pool"

	SMSDisableOnPermanentOnly = "permanent_only"
	SMSDisableOnAnyFailure    = "any_failure"

	PhonePoolStatusReady    = "ready"
	PhonePoolStatusReserved = "reserved"
	PhonePoolStatusUsedUp   = "used_up"
	PhonePoolStatusDisabled = "disabled"
	PhonePoolStatusError    = "error"

	DefaultFixedPassword = "Mima1234567890."

	JobStatusRunning  = "running"
	JobStatusFinished = "finished"
	JobStatusStopped  = "stopped"
	JobStatusFailed   = "failed"

	JobTypeRegister      = "register"
	JobTypeRegisterLogin = "register_login"
	JobTypeRegisterCodex = "register_codex"
	JobTypeLogin         = "login"
	JobTypeCodexLogin    = "codex_login"
)

type SMSConfig struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Type             string          `json:"type"`
	Platform         string          `json:"platform"`
	PlatformLabel    string          `json:"platform_label,omitempty"`
	APIKey           string          `json:"api_key,omitempty"`
	ServiceID        string          `json:"service_id,omitempty"`
	CountryID        int             `json:"country_id,omitempty"`
	MaxPrice         float64         `json:"max_price,omitempty"`
	MaxUsagePerPhone int             `json:"max_usage_per_phone,omitempty"`
	DisableOnError   string          `json:"disable_on_error,omitempty"`
	PoolSummary      *SMSPoolSummary `json:"pool_summary,omitempty"`
}

type SMSPoolSummary struct {
	TotalCount    int `json:"total_count"`
	ReadyCount    int `json:"ready_count"`
	ReservedCount int `json:"reserved_count"`
	UsedUpCount   int `json:"used_up_count"`
	DisabledCount int `json:"disabled_count"`
	ErrorCount    int `json:"error_count"`
	RemainingUses int `json:"remaining_uses"`
}

type PhonePoolItem struct {
	ID            int64  `json:"id"`
	SMSConfigID   string `json:"sms_config_id"`
	PhoneNumber   string `json:"phone_number"`
	CodeFetchURL  string `json:"code_fetch_url"`
	Status        string `json:"status"`
	UseCount      int    `json:"use_count"`
	MaxUseCount   int    `json:"max_use_count"`
	LastError     string `json:"last_error,omitempty"`
	LastJobID     int64  `json:"last_job_id,omitempty"`
	LastMailboxID int64  `json:"last_mailbox_id,omitempty"`
	ReservedAt    string `json:"reserved_at,omitempty"`
	LastUsedAt    string `json:"last_used_at,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type PhonePoolListResult struct {
	Total int             `json:"total"`
	Items []PhonePoolItem `json:"items"`
}

type PhonePoolAttempt struct {
	ID               int64  `json:"id"`
	PhonePoolItemID  int64  `json:"phone_pool_item_id"`
	SMSConfigID      string `json:"sms_config_id"`
	JobID            int64  `json:"job_id"`
	MailboxID        int64  `json:"mailbox_id"`
	PhoneNumber      string `json:"phone_number"`
	Result           string `json:"result"`
	ErrorCode        string `json:"error_code,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
	VerificationCode string `json:"verification_code,omitempty"`
	CreatedAt        string `json:"created_at"`
	FinishedAt       string `json:"finished_at,omitempty"`
}

type ProxyGroup struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Mode    string   `json:"mode"`
	Proxies []string `json:"proxies"`
}

type Settings struct {
	ProxyMode              string       `json:"proxy_mode"`
	Proxies                []string     `json:"proxies"`
	ProxyGroups            []ProxyGroup `json:"proxy_groups"`
	PasswordMode           string       `json:"password_mode"`
	FixedPassword          string       `json:"fixed_password"`
	RegisterConcurrency    int          `json:"register_concurrency"`
	OTPTimeoutSeconds      int          `json:"otp_timeout_seconds"`
	OTPPollIntervalSeconds int          `json:"otp_poll_interval_seconds"`
	IMAPHost               string       `json:"imap_host"`
	IMAPPort               int          `json:"imap_port"`
	IMAPAuthMode           string       `json:"imap_auth_mode"`
	Listen                 string       `json:"listen"`
	SMSConfigs             []SMSConfig  `json:"sms_configs"`
}

type Mailbox struct {
	ID               int64  `json:"id"`
	Email            string `json:"email"`
	Password         string `json:"password,omitempty"`
	ClientID         string `json:"client_id,omitempty"`
	AccessToken      string `json:"access_token,omitempty"`
	Status           string `json:"status"`
	StatusText       string `json:"status_text"`
	RegisterPassword string `json:"register_password,omitempty"`
	TokenJSON        string `json:"token_json,omitempty"`
	Remark           string `json:"remark,omitempty"`
	LastError        string `json:"last_error,omitempty"`
	CurrentStep      string `json:"current_step,omitempty"`
	CurrentStepIndex int    `json:"current_step_index,omitempty"`
	CurrentStepTotal int    `json:"current_step_total,omitempty"`
	Proxy            string `json:"proxy,omitempty"`
	RegisteredAt     string `json:"registered_at,omitempty"`
	LastLoginAt      string `json:"last_login_at,omitempty"`
	PhoneNumber      string `json:"phone_number,omitempty"`
	LastJobID        int64  `json:"last_job_id,omitempty"`
	LastJobType      string `json:"last_job_type,omitempty"`
	LastJobStatus    string `json:"last_job_status,omitempty"`
	LastJobError     string `json:"last_job_error,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type RegisterJob struct {
	ID              int64             `json:"id"`
	Type            string            `json:"type"`
	Status          string            `json:"status"`
	RequestedCount  int               `json:"requested_count"`
	TotalCount      int               `json:"total_count"`
	SuccessCount    int               `json:"success_count"`
	FailedCount     int               `json:"failed_count"`
	SuccessRate     float64           `json:"success_rate"`
	AvgDurationMS   int64             `json:"avg_duration_ms"`
	TotalDurationMS int64             `json:"total_duration_ms"`
	StartedAt       string            `json:"started_at,omitempty"`
	FinishedAt      string            `json:"finished_at,omitempty"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
	Items           []RegisterJobItem `json:"items,omitempty"`
}

type RegisterJobItem struct {
	ID         int64  `json:"id"`
	JobID      int64  `json:"job_id"`
	MailboxID  int64  `json:"mailbox_id"`
	Email      string `json:"email"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type JobTokenExportItem map[string]any

type RuntimeLog struct {
	ID        int64  `json:"id"`
	JobID     int64  `json:"job_id"`
	MailboxID int64  `json:"mailbox_id"`
	Email     string `json:"email"`
	Level     string `json:"level"`
	Step      string `json:"step"`
	StepIndex int    `json:"step_index"`
	StepTotal int    `json:"step_total"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type MailboxListResult struct {
	Total int       `json:"total"`
	Items []Mailbox `json:"items"`
}

type JobListResult struct {
	Total int           `json:"total"`
	Items []RegisterJob `json:"items"`
}

func DefaultSettings() Settings {
	return NormalizeSettings(Settings{ProxyMode: "random", PasswordMode: "random", RegisterConcurrency: 1, OTPTimeoutSeconds: 180, OTPPollIntervalSeconds: 5, IMAPHost: "outlook.office365.com", IMAPPort: 993, IMAPAuthMode: "auto", Listen: ":8080"})
}

func NormalizeSettings(s Settings) Settings {
	s.Proxies = normalizeProxyList(s.Proxies)
	if s.Proxies == nil {
		s.Proxies = []string{}
	}
	if s.ProxyGroups == nil {
		s.ProxyGroups = []ProxyGroup{}
	}
	if s.SMSConfigs == nil {
		s.SMSConfigs = []SMSConfig{}
	}
	if s.ProxyMode != "local" && s.ProxyMode != "single" && s.ProxyMode != "round_robin" {
		s.ProxyMode = "random"
	}
	seenGroupIDs := map[string]struct{}{}
	normalizedGroups := make([]ProxyGroup, 0, len(s.ProxyGroups))
	for _, group := range s.ProxyGroups {
		group.ID = normalizeSettingsItemID(group.ID, seenGroupIDs)
		group.Name = strings.TrimSpace(group.Name)
		group.Mode = normalizeProxyGroupMode(group.Mode)
		group.Proxies = normalizeProxyList(group.Proxies)
		if group.Name == "" || len(group.Proxies) == 0 {
			continue
		}
		normalizedGroups = append(normalizedGroups, group)
	}
	if len(normalizedGroups) == 0 && len(s.Proxies) > 0 {
		mode := "random"
		if s.ProxyMode == "round_robin" {
			mode = "round_robin"
		}
		normalizedGroups = []ProxyGroup{{ID: normalizeSettingsItemID("", seenGroupIDs), Name: "默认分组", Mode: mode, Proxies: append([]string(nil), s.Proxies...)}}
	}
	s.ProxyGroups = normalizedGroups
	if s.PasswordMode != "fixed" {
		s.PasswordMode = "random"
	}
	if s.FixedPassword == "" {
		s.FixedPassword = DefaultFixedPassword
	}
	if s.RegisterConcurrency < 1 {
		s.RegisterConcurrency = 1
	}
	if s.OTPTimeoutSeconds < 30 {
		s.OTPTimeoutSeconds = 180
	}
	if s.OTPPollIntervalSeconds < 1 {
		s.OTPPollIntervalSeconds = 5
	}
	if s.IMAPHost == "" {
		s.IMAPHost = "outlook.office365.com"
	}
	if s.IMAPPort <= 0 {
		s.IMAPPort = 993
	}
	if s.IMAPAuthMode != "password" && s.IMAPAuthMode != "auto" && s.IMAPAuthMode != "xoauth2" {
		s.IMAPAuthMode = "auto"
	}
	if s.Listen == "" {
		s.Listen = ":8080"
	}
	seenSMSIDs := map[string]struct{}{}
	for i := range s.SMSConfigs {
		s.SMSConfigs[i].ID = normalizeSettingsItemID(s.SMSConfigs[i].ID, seenSMSIDs)
		s.SMSConfigs[i].Name = strings.TrimSpace(s.SMSConfigs[i].Name)
		s.SMSConfigs[i].Type = normalizeSMSConfigType(s.SMSConfigs[i].Type)
		s.SMSConfigs[i].PlatformLabel = strings.TrimSpace(s.SMSConfigs[i].PlatformLabel)
		s.SMSConfigs[i].DisableOnError = normalizeSMSDisableOnError(s.SMSConfigs[i].DisableOnError)
		s.SMSConfigs[i].PoolSummary = nil
		if s.SMSConfigs[i].Type == SMSConfigTypePool {
			if s.SMSConfigs[i].Platform == "" {
				s.SMSConfigs[i].Platform = "custom"
			}
			if s.SMSConfigs[i].MaxUsagePerPhone < 1 {
				s.SMSConfigs[i].MaxUsagePerPhone = 1
			}
			continue
		}
		if s.SMSConfigs[i].Platform == "" {
			s.SMSConfigs[i].Platform = "smsbower"
		}
		if s.SMSConfigs[i].ServiceID == "" {
			s.SMSConfigs[i].ServiceID = "dr"
		}
		if s.SMSConfigs[i].CountryID <= 0 {
			s.SMSConfigs[i].CountryID = 38
		}
	}
	return s
}

func ValidateSettings(s Settings) error {
	s = NormalizeSettings(s)
	seenProxyNames := map[string]string{}
	seenProxyIDs := map[string]struct{}{}
	for _, group := range s.ProxyGroups {
		if strings.TrimSpace(group.ID) == "" {
			return fmt.Errorf("proxy group id is required")
		}
		if _, ok := seenProxyIDs[group.ID]; ok {
			return fmt.Errorf("proxy group id %q already exists", group.ID)
		}
		seenProxyIDs[group.ID] = struct{}{}
		name := strings.TrimSpace(group.Name)
		if name == "" {
			return fmt.Errorf("proxy group name is required")
		}
		if len(group.Proxies) == 0 {
			return fmt.Errorf("proxy group %q must contain at least one proxy", name)
		}
		key := strings.ToLower(name)
		if existing, ok := seenProxyNames[key]; ok {
			return fmt.Errorf("proxy group %q already exists", existing)
		}
		seenProxyNames[key] = name
	}
	seenSMSNames := map[string]string{}
	seenSMSIDs := map[string]struct{}{}
	for _, config := range s.SMSConfigs {
		if strings.TrimSpace(config.ID) == "" {
			return fmt.Errorf("sms config id is required")
		}
		if _, ok := seenSMSIDs[config.ID]; ok {
			return fmt.Errorf("sms config id %q already exists", config.ID)
		}
		seenSMSIDs[config.ID] = struct{}{}
		name := strings.TrimSpace(config.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if existing, ok := seenSMSNames[key]; ok {
			return fmt.Errorf("sms config %q already exists", existing)
		}
		seenSMSNames[key] = name
		switch config.Type {
		case SMSConfigTypeProvider:
			if strings.TrimSpace(config.APIKey) == "" {
				return fmt.Errorf("sms config %q missing api_key", config.Name)
			}
		case SMSConfigTypePool:
			if strings.TrimSpace(config.PlatformLabel) == "" {
				return fmt.Errorf("sms config %q missing platform_label", config.Name)
			}
			if config.MaxUsagePerPhone < 1 {
				return fmt.Errorf("sms config %q max_usage_per_phone must be greater than 0", config.Name)
			}
			if config.DisableOnError != SMSDisableOnPermanentOnly && config.DisableOnError != SMSDisableOnAnyFailure {
				return fmt.Errorf("sms config %q has unsupported disable_on_error", config.Name)
			}
		default:
			return fmt.Errorf("sms config %q has unsupported type %q", config.Name, config.Type)
		}
	}
	return nil
}

func FindSMSConfigByID(configs []SMSConfig, id string) (SMSConfig, bool) {
	target := strings.TrimSpace(id)
	if target == "" {
		return SMSConfig{}, false
	}
	for _, config := range configs {
		if strings.TrimSpace(config.ID) == target {
			return config, true
		}
	}
	return SMSConfig{}, false
}

func FindSMSConfig(configs []SMSConfig, name string) (SMSConfig, bool) {
	target := strings.TrimSpace(name)
	if target == "" {
		return SMSConfig{}, false
	}
	for _, config := range configs {
		if strings.EqualFold(strings.TrimSpace(config.Name), target) {
			return config, true
		}
	}
	return SMSConfig{}, false
}

func FindProxyGroupByID(groups []ProxyGroup, id string) (ProxyGroup, bool) {
	target := strings.TrimSpace(id)
	if target == "" {
		return ProxyGroup{}, false
	}
	for _, group := range groups {
		if strings.TrimSpace(group.ID) == target {
			return group, true
		}
	}
	return ProxyGroup{}, false
}

func FindProxyGroup(groups []ProxyGroup, name string) (ProxyGroup, bool) {
	target := strings.TrimSpace(name)
	if target == "" {
		return ProxyGroup{}, false
	}
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.Name), target) {
			return group, true
		}
	}
	return ProxyGroup{}, false
}

func normalizeProxyGroupMode(mode string) string {
	if strings.TrimSpace(mode) == "round_robin" {
		return "round_robin"
	}
	return "random"
}

func normalizeSMSConfigType(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == SMSConfigTypePool {
		return SMSConfigTypePool
	}
	return SMSConfigTypeProvider
}

func normalizeSMSDisableOnError(mode string) string {
	if strings.TrimSpace(mode) == SMSDisableOnAnyFailure {
		return SMSDisableOnAnyFailure
	}
	return SMSDisableOnPermanentOnly
}

func normalizeSettingsItemID(id string, seen map[string]struct{}) string {
	id = strings.TrimSpace(id)
	if id != "" {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			return id
		}
	}
	for {
		candidate := newSettingsItemID()
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		return candidate
	}
}

func newSettingsItemID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("generate settings item id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

func normalizeProxyList(values []string) []string {
	if values == nil {
		return []string{}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func MailboxStatusText(status string) string {
	switch status {
	case MailboxStatusNew:
		return "新导入"
	case MailboxStatusRegistering:
		return "注册中"
	case MailboxStatusRegistered:
		return "已注册"
	case MailboxStatusLogining:
		return "登录中"
	case MailboxStatusAbnormal:
		return "异常"
	default:
		return status
	}
}
