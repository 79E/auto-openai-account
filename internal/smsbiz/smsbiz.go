package smsbiz

import (
	"context"
	"fmt"
	"time"

	"github.com/79E/auto-openai-account/internal/domain"
)

type Activation struct {
	ActivationID       string  `json:"activationId"`
	PhoneNumber        string  `json:"phoneNumber"`
	ActivationCost     float64 `json:"activationCost"`
	CountryCode        int     `json:"countryCode"`
	CountryPhoneCode   int     `json:"countryPhoneCode,omitempty"`
	CanGetAnotherSms   bool    `json:"canGetAnotherSms"`
	ActivationTime     string  `json:"activationTime"`
	ActivationEndTime  string  `json:"activationEndTime"`
	ActivationOperator string  `json:"activationOperator"`
}

type SMSCodeResult struct {
	Code   string
	Status string
}

const (
	StatusWaitCode     = "STATUS_WAIT_CODE"
	StatusWaitRetry    = "STATUS_WAIT_RETRY"
	StatusWaitResend   = "STATUS_WAIT_RESEND"
	StatusCancel       = "STATUS_CANCEL"
	StatusOK           = "STATUS_OK"
	StatusAccessReady  = "ACCESS_READY"
	StatusAccessRetry  = "ACCESS_RETRY_GET"
	StatusActivation   = "ACCESS_ACTIVATION"
	StatusAccessCancel = "ACCESS_CANCEL"
)

type Provider interface {
	GetNumber(ctx context.Context, serviceID string, countryID int, maxPrice float64) (*Activation, error)
	MarkSubmitted(ctx context.Context, activationID string) error
	GetStatus(ctx context.Context, activationID string) (*SMSCodeResult, error)
	SetStatus(ctx context.Context, activationID string, status int) error
	Close()
}

type PhonePoolStore interface {
	ReserveNextPhonePoolItem(configID string) (domain.PhonePoolItem, error)
	GetPhonePoolItem(id int64) (domain.PhonePoolItem, error)
	MarkPhonePoolItemSubmitted(itemID int64, jobID int64, mailboxID int64) error
	CompletePhonePoolItem(itemID int64) error
	ExhaustPhonePoolItem(itemID int64, errMessage string) error
	ReleasePhonePoolItem(itemID int64, errMessage string) error
	FailPhonePoolItem(itemID int64, disable bool, errMessage string) error
	CreatePhonePoolAttempt(itemID int64, configID string, jobID int64, mailboxID int64, phoneNumber string) (int64, error)
	FinishPhonePoolAttempt(attemptID int64, result string, errorCode string, errorMessage string, verificationCode string) error
}

type Config struct {
	Platform         string
	APIKey           string
	ServiceID        string
	CountryID        int
	MaxPrice         float64
	SMSConfigID      string
	MaxUsagePerPhone int
	DisableOnError   string
	Store            PhonePoolStore
	JobID            int64
	MailboxID        int64
}

func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Platform {
	case "custom", "pool":
		return NewPhonePool(cfg)
	case "hero", "hero-sms", "herosms":
		return NewHeroSMS(cfg), nil
	case "smsbower", "bower":
		return NewSMSBower(cfg), nil
	default:
		return NewSMSBower(cfg), nil
	}
}

func PollForCode(ctx context.Context, provider Provider, activationID string, timeout time.Duration, pollInterval time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := provider.GetStatus(ctx, activationID)
		if err != nil {
			return "", err
		}

		switch result.Status {
		case StatusOK:
			return result.Code, nil
		case StatusActivation:
			return result.Code, nil
		case StatusCancel:
			return "", fmt.Errorf("activation cancelled")
		case StatusAccessCancel:
			return "", fmt.Errorf("activation cancelled by provider")
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}
	return "", fmt.Errorf("timeout waiting for SMS code")
}

func MustGetCode(ctx context.Context, provider Provider, serviceID string, countryID int, maxPrice float64, maxRetries int) (string, string, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		activation, err := provider.GetNumber(ctx, serviceID, countryID, maxPrice)
		if err != nil {
			if attempt < maxRetries-1 {
				time.Sleep(time.Second * 2)
				continue
			}
			return "", "", fmt.Errorf("failed to get phone number after %d attempts: %w", maxRetries, err)
		}

		code, err := PollForCode(ctx, provider, activation.ActivationID, 150*time.Second, 5*time.Second)
		if err != nil {
			_ = provider.SetStatus(ctx, activation.ActivationID, 8)
			if attempt < maxRetries-1 {
				time.Sleep(time.Second * 2)
				continue
			}
			return "", activation.PhoneNumber, fmt.Errorf("failed to get SMS code after %d attempts: %w", maxRetries, err)
		}

		_ = provider.SetStatus(ctx, activation.ActivationID, 6)
		return code, activation.PhoneNumber, nil
	}
	return "", "", fmt.Errorf("max retries exceeded")
}
