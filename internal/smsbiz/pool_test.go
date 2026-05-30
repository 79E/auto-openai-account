package smsbiz

import (
	"context"
	"testing"

	"github.com/79E/auto-openai-account/internal/domain"
)

type phonePoolStoreMock struct {
	reservedItem       domain.PhonePoolItem
	createdAttemptID   int64
	exhaustedItemID    int64
	exhaustedError     string
	finishedAttemptID  int64
	finishedResult     string
	finishedErrorCode  string
	finishedErrorMsg   string
}

func (m *phonePoolStoreMock) ReserveNextPhonePoolItem(configID string) (domain.PhonePoolItem, error) {
	return m.reservedItem, nil
}

func (m *phonePoolStoreMock) GetPhonePoolItem(id int64) (domain.PhonePoolItem, error) {
	return m.reservedItem, nil
}

func (m *phonePoolStoreMock) MarkPhonePoolItemSubmitted(itemID int64, jobID int64, mailboxID int64) error {
	return nil
}

func (m *phonePoolStoreMock) CompletePhonePoolItem(itemID int64) error {
	return nil
}

func (m *phonePoolStoreMock) ExhaustPhonePoolItem(itemID int64, errMessage string) error {
	m.exhaustedItemID = itemID
	m.exhaustedError = errMessage
	return nil
}

func (m *phonePoolStoreMock) ReleasePhonePoolItem(itemID int64, errMessage string) error {
	return nil
}

func (m *phonePoolStoreMock) FailPhonePoolItem(itemID int64, disable bool, errMessage string) error {
	return nil
}

func (m *phonePoolStoreMock) CreatePhonePoolAttempt(itemID int64, configID string, jobID int64, mailboxID int64, phoneNumber string) (int64, error) {
	return m.createdAttemptID, nil
}

func (m *phonePoolStoreMock) FinishPhonePoolAttempt(attemptID int64, result string, errorCode string, errorMessage string, verificationCode string) error {
	m.finishedAttemptID = attemptID
	m.finishedResult = result
	m.finishedErrorCode = errorCode
	m.finishedErrorMsg = errorMessage
	return nil
}

func TestPhonePoolCancelPermanentMarksMaxUsageRejectedNumberUsedUp(t *testing.T) {
	store := &phonePoolStoreMock{
		reservedItem: domain.PhonePoolItem{
			ID:          42,
			SMSConfigID: "cfg-1",
			PhoneNumber: "+12082469513",
			Status:      domain.PhonePoolStatusReserved,
		},
		createdAttemptID: 88,
	}
	provider, err := NewPhonePool(Config{Store: store, SMSConfigID: "cfg-1"})
	if err != nil {
		t.Fatalf("NewPhonePool() error = %v", err)
	}
	activation, err := provider.GetNumber(context.Background(), "", 0, 0)
	if err != nil {
		t.Fatalf("GetNumber() error = %v", err)
	}
	message := "HTTP 403，phone_max_usage_exceeded：This phone number is already linked to the maximum number of accounts."
	if err := provider.CancelPermanent(context.Background(), activation.ActivationID, "phone_max_usage_exceeded", message); err != nil {
		t.Fatalf("CancelPermanent() error = %v", err)
	}
	if store.exhaustedItemID != 42 {
		t.Fatalf("ExhaustPhonePoolItem() itemID = %d, want 42", store.exhaustedItemID)
	}
	if store.exhaustedError != message {
		t.Fatalf("ExhaustPhonePoolItem() error = %q, want %q", store.exhaustedError, message)
	}
	if store.finishedAttemptID != 88 {
		t.Fatalf("FinishPhonePoolAttempt() attemptID = %d, want 88", store.finishedAttemptID)
	}
	if store.finishedResult != "provider_error" {
		t.Fatalf("FinishPhonePoolAttempt() result = %q, want provider_error", store.finishedResult)
	}
	if store.finishedErrorCode != "phone_max_usage_exceeded" {
		t.Fatalf("FinishPhonePoolAttempt() errorCode = %q, want phone_max_usage_exceeded", store.finishedErrorCode)
	}
	if store.finishedErrorMsg != message {
		t.Fatalf("FinishPhonePoolAttempt() errorMessage = %q, want %q", store.finishedErrorMsg, message)
	}
}
