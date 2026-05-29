package legacy

const (
	MailboxStatusNew         = "new"
	MailboxStatusRegistering = "registering"
	MailboxStatusRegistered  = "registered"
	MailboxStatusLogining    = "logining"
	MailboxStatusAbnormal    = "abnormal"

	JobStatusRunning  = "running"
	JobStatusFinished = "finished"
	JobStatusStopped  = "stopped"
	JobStatusFailed   = "failed"
)

type Settings struct {
	Proxy                  string `json:"proxy"`
	PasswordMode           string `json:"password_mode"`
	FixedPassword          string `json:"fixed_password"`
	RegisterConcurrency    int    `json:"register_concurrency"`
	OTPTimeoutSeconds      int    `json:"otp_timeout_seconds"`
	OTPPollIntervalSeconds int    `json:"otp_poll_interval_seconds"`
	IMAPHost               string `json:"imap_host"`
	IMAPPort               int    `json:"imap_port"`
	IMAPAuthMode           string `json:"imap_auth_mode"`
	Listen                 string `json:"listen"`
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
	RegisteredAt     string `json:"registered_at,omitempty"`
	LastLoginAt      string `json:"last_login_at,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func (m Mailbox) CanFetchEmailOTP(settings Settings) bool {
	settings = normalizeSettings(settings)
	if Clean(m.Email) == "" {
		return false
	}
	if settings.IMAPAuthMode == "password" {
		return Clean(m.Password) != ""
	}
	if Clean(m.ClientID) != "" && Clean(m.AccessToken) != "" {
		return true
	}
	if settings.IMAPAuthMode == "auto" {
		return Clean(m.Password) != ""
	}
	return false
}

type RegisterJob struct {
	ID              int64             `json:"id"`
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

func defaultSettings() Settings {
	cfg := loadConfig()
	mode := "random"
	if cfg.PasswordMode == "manual" {
		mode = "fixed"
	}
	return normalizeSettings(Settings{
		Proxy:                  cfg.DefaultProxy,
		PasswordMode:           mode,
		FixedPassword:          cfg.DefaultPassword,
		RegisterConcurrency:    1,
		OTPTimeoutSeconds:      180,
		OTPPollIntervalSeconds: 5,
		IMAPHost:               "outlook.office365.com",
		IMAPPort:               993,
		IMAPAuthMode:           "auto",
		Listen:                 ":8080",
	})
}

func normalizeSettings(settings Settings) Settings {
	if settings.PasswordMode != "fixed" {
		settings.PasswordMode = "random"
	}
	if settings.RegisterConcurrency < 1 {
		settings.RegisterConcurrency = 1
	}
	if settings.OTPTimeoutSeconds < 30 {
		settings.OTPTimeoutSeconds = 180
	}
	if settings.OTPPollIntervalSeconds < 1 {
		settings.OTPPollIntervalSeconds = 5
	}
	if settings.IMAPHost == "" {
		settings.IMAPHost = "outlook.office365.com"
	}
	if settings.IMAPPort <= 0 {
		settings.IMAPPort = 993
	}
	switch settings.IMAPAuthMode {
	case "password", "auto", "xoauth2":
	default:
		settings.IMAPAuthMode = "auto"
	}
	if settings.Listen == "" {
		settings.Listen = ":8080"
	}
	return settings
}

func mailboxStatusText(status string) string {
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

func withStatusText(item Mailbox) Mailbox {
	item.StatusText = mailboxStatusText(item.Status)
	return item
}
