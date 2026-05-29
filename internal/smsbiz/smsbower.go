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

type SMSBower struct {
	client *http.Client
	apiKey string
}

func NewSMSBower(cfg Config) *SMSBower {
	return &SMSBower{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: cfg.APIKey,
	}
}

func (s *SMSBower) GetNumber(ctx context.Context, serviceID string, countryID int, maxPrice float64) (*Activation, error) {
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("action", "getNumberV2")
	params.Set("service", serviceID)
	params.Set("country", fmt.Sprintf("%d", countryID))
	if maxPrice > 0 {
		params.Set("maxPrice", fmt.Sprintf("%.4f", maxPrice))
	}

	reqURL := "https://smsbower.page/stubs/handler_api.php?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(body), "BAD_KEY") {
		return nil, fmt.Errorf("invalid API key")
	}
	if strings.Contains(string(body), "BAD_ACTION") {
		return nil, fmt.Errorf("invalid action")
	}
	if strings.Contains(string(body), "BAD_SERVICE") {
		return nil, fmt.Errorf("invalid service")
	}
	if err := providerPlainTextError(string(body)); err != nil {
		return nil, err
	}

	return parseSMSBowerActivation(body)
}

func parseSMSBowerActivation(body []byte) (*Activation, error) {
	var result struct {
		ActivationID       json.RawMessage `json:"activationId"`
		PhoneNumber        json.RawMessage `json:"phoneNumber"`
		ActivationCost     json.RawMessage `json:"activationCost"`
		CountryCode        json.RawMessage `json:"countryCode"`
		CanGetAnotherSms   json.RawMessage `json:"canGetAnotherSms"`
		ActivationTime     string          `json:"activationTime"`
		ActivationOperator json.RawMessage `json:"activationOperator"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		if textErr := providerPlainTextError(string(body)); textErr != nil {
			return nil, textErr
		}
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &Activation{
		ActivationID:       rawValueString(result.ActivationID),
		PhoneNumber:        rawValueString(result.PhoneNumber),
		ActivationCost:     rawFloat(result.ActivationCost),
		CountryCode:        rawInt(result.CountryCode),
		CanGetAnotherSms:   rawBool(result.CanGetAnotherSms),
		ActivationTime:     result.ActivationTime,
		ActivationOperator: rawValueString(result.ActivationOperator),
	}, nil
}

func (s *SMSBower) GetStatus(ctx context.Context, activationID string) (*SMSCodeResult, error) {
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("action", "getStatus")
	params.Set("id", activationID)

	reqURL := "https://smsbower.page/stubs/handler_api.php?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
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

func (s *SMSBower) SetStatus(ctx context.Context, activationID string, status int) error {
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("action", "setStatus")
	params.Set("id", activationID)
	params.Set("status", fmt.Sprintf("%d", status))

	reqURL := "https://smsbower.page/stubs/handler_api.php?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
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

func (s *SMSBower) Close() {
	s.client.CloseIdleConnections()
}
