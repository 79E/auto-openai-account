package smsbiz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HeroSMS struct {
	client *http.Client
	apiKey string
}

func NewHeroSMS(cfg Config) *HeroSMS {
	return &HeroSMS{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: cfg.APIKey,
	}
}

func (h *HeroSMS) GetNumber(ctx context.Context, serviceID string, countryID int, maxPrice float64) (*Activation, error) {
	params := url.Values{}
	params.Set("api_key", h.apiKey)
	params.Set("action", "getNumberV2")
	params.Set("service", serviceID)
	params.Set("country", fmt.Sprintf("%d", countryID))
	if maxPrice > 0 {
		params.Set("maxPrice", fmt.Sprintf("%.4f", maxPrice))
	}

	reqURL := "https://hero-sms.com/stubs/handler_api.php?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := providerPlainTextError(string(body)); err != nil {
		return nil, err
	}

	return parseHeroSMSActivation(body)
}

func parseHeroSMSActivation(body []byte) (*Activation, error) {
	var result struct {
		ActivationID       json.RawMessage `json:"activationId"`
		PhoneNumber        json.RawMessage `json:"phoneNumber"`
		ActivationCost     json.RawMessage `json:"activationCost"`
		Currency           int             `json:"currency"`
		CountryCode        json.RawMessage `json:"countryCode"`
		CountryPhoneCode   json.RawMessage `json:"countryPhoneCode"`
		CanGetAnotherSms   json.RawMessage `json:"canGetAnotherSms"`
		ActivationTime     string          `json:"activationTime"`
		ActivationEndTime  string          `json:"activationEndTime"`
		ActivationOperator string          `json:"activationOperator"`
		Title              string          `json:"title,omitempty"`
		Details            string          `json:"details,omitempty"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		if textErr := providerPlainTextError(string(body)); textErr != nil {
			return nil, textErr
		}
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Title != "" {
		return nil, fmt.Errorf("%s: %s", result.Title, result.Details)
	}

	return &Activation{
		ActivationID:       rawValueString(result.ActivationID),
		PhoneNumber:        rawValueString(result.PhoneNumber),
		ActivationCost:     rawFloat(result.ActivationCost),
		CountryCode:        rawInt(result.CountryCode),
		CountryPhoneCode:   rawInt(result.CountryPhoneCode),
		CanGetAnotherSms:   rawBool(result.CanGetAnotherSms),
		ActivationTime:     result.ActivationTime,
		ActivationEndTime:  result.ActivationEndTime,
		ActivationOperator: result.ActivationOperator,
	}, nil
}

func (h *HeroSMS) GetStatus(ctx context.Context, activationID string) (*SMSCodeResult, error) {
	params := url.Values{}
	params.Set("api_key", h.apiKey)
	params.Set("action", "getStatus")
	params.Set("id", activationID)

	reqURL := "https://hero-sms.com/stubs/handler_api.php?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseSMSStatusResponse(string(body))
}

func (h *HeroSMS) SetStatus(ctx context.Context, activationID string, status int) error {
	params := url.Values{}
	params.Set("api_key", h.apiKey)
	params.Set("action", "setStatus")
	params.Set("id", activationID)
	params.Set("status", fmt.Sprintf("%d", status))

	reqURL := "https://hero-sms.com/stubs/handler_api.php?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	text := strings.TrimSpace(string(body))
	return parseSetStatusResponse(text)
}

func (h *HeroSMS) Close() {
	h.client.CloseIdleConnections()
}
