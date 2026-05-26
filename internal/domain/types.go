package domain

const (
	MailboxStatusNew         = "new"
	MailboxStatusRegistering = "registering"
	MailboxStatusRegistered  = "registered"
	MailboxStatusLogining    = "logining"
	MailboxStatusAbnormal    = "abnormal"

	DefaultFixedPassword = "Mima1234567890."

	JobStatusRunning  = "running"
	JobStatusFinished = "finished"
	JobStatusStopped  = "stopped"
	JobStatusFailed   = "failed"

	JobTypeRegister = "register"
	JobTypeLogin    = "login"
)

type Settings struct {
	ProxyMode              string   `json:"proxy_mode"`
	Proxies                []string `json:"proxies"`
	PasswordMode           string   `json:"password_mode"`
	FixedPassword          string   `json:"fixed_password"`
	RegisterConcurrency    int      `json:"register_concurrency"`
	OTPTimeoutSeconds      int      `json:"otp_timeout_seconds"`
	OTPPollIntervalSeconds int      `json:"otp_poll_interval_seconds"`
	IMAPHost               string   `json:"imap_host"`
	IMAPPort               int      `json:"imap_port"`
	IMAPAuthMode           string   `json:"imap_auth_mode"`
	Listen                 string   `json:"listen"`
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
	if s.Proxies == nil {
		s.Proxies = []string{}
	}
	if s.ProxyMode != "single" && s.ProxyMode != "round_robin" {
		s.ProxyMode = "random"
	}
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
	return s
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
