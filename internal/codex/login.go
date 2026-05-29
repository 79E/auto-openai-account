package codex

import (
	"context"
	"encoding/json"
	"time"

	"github.com/79E/auto-openai-account/internal/legacy"
)

type LoginStep string

const (
	StepAuthorize         LoginStep = "codex_authorize"
	StepSubmitEmail       LoginStep = "submit_email"
	StepPasswordLogin     LoginStep = "password_login"
	StepAddPhone          LoginStep = "add_phone"
	StepPhoneVerification LoginStep = "phone_verification"
	StepExchangeToken     LoginStep = "exchange_token"
)

type LoginProgress struct {
	Step      LoginStep
	StepIndex int
	StepTotal int
	Message   string
	Details   map[string]any
}

type SMSActivation = legacy.CodexSMSActivation

type SMSProvider interface {
	GetNumber(context.Context) (*SMSActivation, error)
	PollCode(context.Context, string) (string, error)
	Complete(context.Context, string) error
	Cancel(context.Context, string) error
}

type LoginOptions struct {
	Email                    string
	Password                 string
	Proxy                    string
	ProxyController          legacy.ProxyController
	SMSProvider              SMSProvider
	OTPFetcher               func(context.Context) (string, error)
	ProgressChan             chan<- LoginProgress
	MaxPhoneAttempts         int
	PasswordVerifyRetries    int
	PasswordVerifyRetryDelay time.Duration
}

type LoginResult struct {
	TokenPayload map[string]any
	PhoneNumber  string
	TokenJSON    string
}

func LoginWithCodex(ctx context.Context, opts LoginOptions) (*LoginResult, error) {
	result, err := legacy.CodexLogin(ctx, legacy.CodexLoginInput{
		Email:                    opts.Email,
		Password:                 opts.Password,
		Proxy:                    opts.Proxy,
		ProxyController:          opts.ProxyController,
		SMSProvider:              opts.SMSProvider,
		OTPFetcher:               opts.OTPFetcher,
		MaxPhoneAttempts:         opts.MaxPhoneAttempts,
		PasswordVerifyRetries:    opts.PasswordVerifyRetries,
		PasswordVerifyRetryDelay: opts.PasswordVerifyRetryDelay,
		Progress: func(step string, index int, total int, message string) {
			if opts.ProgressChan == nil {
				return
			}
			opts.ProgressChan <- LoginProgress{
				Step:      LoginStep(step),
				StepIndex: index,
				StepTotal: total,
				Message:   message,
			}
		},
	})
	if err != nil {
		return nil, err
	}
	out := &LoginResult{
		TokenPayload: result.TokenPayload,
		PhoneNumber:  result.PhoneNumber,
	}
	if data, err := json.Marshal(result.TokenPayload); err == nil {
		out.TokenJSON = string(data)
	}
	return out, nil
}
