package smsbiz

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/79E/auto-openai-account/internal/domain"
)

type PhonePool struct {
	client   *http.Client
	store    PhonePoolStore
	configID string
	jobID    int64
	mailboxID int64
	disableOnError string
	states   map[string]*phonePoolActivationState
}

type phonePoolActivationState struct {
	item       domain.PhonePoolItem
	attemptID  int64
	submitted  bool
	finished   bool
	lastCode   string
}

func NewPhonePool(cfg Config) (*PhonePool, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("phone pool store is required")
	}
	if strings.TrimSpace(cfg.SMSConfigID) == "" {
		return nil, fmt.Errorf("phone pool sms config id is required")
	}
	return &PhonePool{
		client: &http.Client{Timeout: 15 * time.Second},
		store: cfg.Store,
		configID: strings.TrimSpace(cfg.SMSConfigID),
		jobID: cfg.JobID,
		mailboxID: cfg.MailboxID,
		disableOnError: strings.TrimSpace(cfg.DisableOnError),
		states: map[string]*phonePoolActivationState{},
	}, nil
}

func (p *PhonePool) GetNumber(ctx context.Context, serviceID string, countryID int, maxPrice float64) (*Activation, error) {
	item, err := p.store.ReserveNextPhonePoolItem(p.configID)
	if err != nil {
		return nil, err
	}
	attemptID, err := p.store.CreatePhonePoolAttempt(item.ID, item.SMSConfigID, p.jobID, p.mailboxID, item.PhoneNumber)
	if err != nil {
		_ = p.store.ReleasePhonePoolItem(item.ID, err.Error())
		return nil, err
	}
	activationID := strconv.FormatInt(item.ID, 10)
	p.states[activationID] = &phonePoolActivationState{item: item, attemptID: attemptID}
	return &Activation{ActivationID: activationID, PhoneNumber: item.PhoneNumber}, nil
}

func (p *PhonePool) MarkSubmitted(ctx context.Context, activationID string) error {
	state, itemID, err := p.lookupState(activationID)
	if err != nil {
		return err
	}
	if state.submitted {
		return nil
	}
	if err := p.store.MarkPhonePoolItemSubmitted(itemID, p.jobID, p.mailboxID); err != nil {
		return err
	}
	state.submitted = true
	return nil
}

func (p *PhonePool) GetStatus(ctx context.Context, activationID string) (*SMSCodeResult, error) {
	state, _, err := p.lookupState(activationID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, state.item.CodeFetchURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	text := string(body)
	if code := ExtractCodeFromText(text); code != "" {
		state.lastCode = code
		return &SMSCodeResult{Code: code, Status: StatusOK}, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("code fetch returned http %d", resp.StatusCode)
	}
	return &SMSCodeResult{Status: StatusWaitCode}, nil
}

func (p *PhonePool) SetStatus(ctx context.Context, activationID string, status int) error {
	state, itemID, err := p.lookupState(activationID)
	if err != nil {
		return err
	}
	if state.finished {
		return nil
	}
	switch status {
	case 6:
		if err := p.store.CompletePhonePoolItem(itemID); err != nil {
			return err
		}
		if state.attemptID > 0 {
			if err := p.store.FinishPhonePoolAttempt(state.attemptID, "success", "", "", state.lastCode); err != nil {
				return err
			}
		}
	case 8:
		disable := false
		result := "cancelled"
		if state.submitted {
			disable = p.disableOnError == domain.SMSDisableOnAnyFailure
			result = "provider_error"
			if err := p.store.FailPhonePoolItem(itemID, disable, "activation cancelled"); err != nil {
				return err
			}
		} else {
			if err := p.store.ReleasePhonePoolItem(itemID, "activation cancelled"); err != nil {
				return err
			}
		}
		if state.attemptID > 0 {
			if err := p.store.FinishPhonePoolAttempt(state.attemptID, result, "cancelled", "activation cancelled", state.lastCode); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	state.finished = true
	return nil
}

func (p *PhonePool) Close() {
	p.client.CloseIdleConnections()
}

func (p *PhonePool) lookupState(activationID string) (*phonePoolActivationState, int64, error) {
	state, ok := p.states[strings.TrimSpace(activationID)]
	if !ok {
		return nil, 0, fmt.Errorf("activation %q not found", activationID)
	}
	itemID, err := strconv.ParseInt(strings.TrimSpace(activationID), 10, 64)
	if err != nil || itemID <= 0 {
		return nil, 0, fmt.Errorf("invalid activation id %q", activationID)
	}
	return state, itemID, nil
}
