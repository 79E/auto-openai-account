package smsbiz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Catalog struct {
	Services  []CatalogService `json:"services"`
	Countries []CatalogCountry `json:"countries"`
}

type CatalogService struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type CatalogCountry struct {
	ID      int    `json:"id"`
	Rus     string `json:"rus,omitempty"`
	Eng     string `json:"eng,omitempty"`
	Chn     string `json:"chn,omitempty"`
	Visible int    `json:"visible,omitempty"`
	Retry   int    `json:"retry,omitempty"`
}

func FetchCatalog(ctx context.Context, platform string, apiKey string) (*Catalog, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("sms api key is required")
	}
	baseURL, err := catalogBaseURL(platform)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	services, err := fetchServices(ctx, client, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	countries, err := fetchCountries(ctx, client, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return &Catalog{Services: services, Countries: countries}, nil
}

func catalogBaseURL(platform string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "hero", "hero-sms", "herosms":
		return "https://hero-sms.com/stubs/handler_api.php", nil
	case "smsbower", "bower", "":
		return "https://smsbower.page/stubs/handler_api.php", nil
	default:
		return "", fmt.Errorf("unsupported sms platform: %s", platform)
	}
}

func fetchServices(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]CatalogService, error) {
	body, err := fetchCatalogAction(ctx, client, baseURL, apiKey, "getServicesList")
	if err != nil {
		return nil, err
	}
	var result struct {
		Status   string           `json:"status"`
		Services []CatalogService `json:"services"`
		Title    string           `json:"title,omitempty"`
		Details  string           `json:"details,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse sms services: %w", err)
	}
	if result.Title != "" {
		return nil, fmt.Errorf("%s: %s", result.Title, result.Details)
	}
	sort.SliceStable(result.Services, func(i, j int) bool {
		return strings.ToLower(result.Services[i].Name) < strings.ToLower(result.Services[j].Name)
	})
	return result.Services, nil
}

func fetchCountries(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]CatalogCountry, error) {
	body, err := fetchCatalogAction(ctx, client, baseURL, apiKey, "getCountries")
	if err != nil {
		return nil, err
	}
	countries, err := parseCountries(body)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(countries, func(i, j int) bool {
		if countries[i].Chn != "" && countries[j].Chn != "" {
			return countries[i].Chn < countries[j].Chn
		}
		if countries[i].Eng != "" && countries[j].Eng != "" {
			return strings.ToLower(countries[i].Eng) < strings.ToLower(countries[j].Eng)
		}
		return countries[i].ID < countries[j].ID
	})
	return countries, nil
}

func fetchCatalogAction(ctx context.Context, client *http.Client, baseURL, apiKey, action string) ([]byte, error) {
	params := url.Values{}
	params.Set("api_key", apiKey)
	params.Set("action", action)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sms provider returned http %d", resp.StatusCode)
	}
	if strings.Contains(text, "BAD_KEY") {
		return nil, fmt.Errorf("invalid sms api key")
	}
	if strings.Contains(text, "BAD_ACTION") {
		return nil, fmt.Errorf("invalid sms action")
	}
	var apiErr struct {
		Title   string `json:"title"`
		Details string `json:"details"`
	}
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Title != "" {
		return nil, fmt.Errorf("%s: %s", apiErr.Title, apiErr.Details)
	}
	return body, nil
}

func parseCountries(body []byte) ([]CatalogCountry, error) {
	if countries, ok := parseCountryArray(body); ok {
		return countries, nil
	}

	var rawObject map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawObject); err != nil {
		return nil, fmt.Errorf("failed to parse sms countries")
	}

	if title := rawString(rawObject["title"]); title != "" {
		return nil, fmt.Errorf("%s: %s", title, rawString(rawObject["details"]))
	}

	if countriesRaw, ok := rawObject["countries"]; ok {
		if countries, ok := parseCountryArray(countriesRaw); ok {
			return countries, nil
		}
		if countries, ok := parseCountryObject(countriesRaw); ok {
			return countries, nil
		}
	}

	if countries, ok := countriesFromRawMap(rawObject); ok {
		return countries, nil
	}
	return nil, fmt.Errorf("failed to parse sms countries")
}

func parseCountryArray(raw []byte) ([]CatalogCountry, bool) {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(raw, &rawItems); err != nil {
		return nil, false
	}
	countries := make([]CatalogCountry, 0, len(rawItems))
	for _, rawItem := range rawItems {
		country, ok := parseCountryItem(rawItem, "")
		if ok {
			countries = append(countries, country)
		}
	}
	return countries, true
}

func parseCountryObject(raw []byte) ([]CatalogCountry, bool) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, false
	}
	return countriesFromRawMap(rawMap)
}

func countriesFromRawMap(rawMap map[string]json.RawMessage) ([]CatalogCountry, bool) {
	countries := make([]CatalogCountry, 0, len(rawMap))
	for key, raw := range rawMap {
		if key == "status" || key == "title" || key == "details" || key == "countries" {
			continue
		}
		if country, ok := parseCountryItem(raw, key); ok {
			countries = append(countries, country)
		}
	}
	return countries, len(countries) > 0
}

func parseCountryItem(raw json.RawMessage, fallbackID string) (CatalogCountry, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return CatalogCountry{}, false
	}
	country := CatalogCountry{
		ID:      rawInt(fields["id"]),
		Rus:     rawString(fields["rus"]),
		Eng:     rawString(fields["eng"]),
		Chn:     rawString(fields["chn"]),
		Visible: rawInt(fields["visible"]),
		Retry:   rawInt(fields["retry"]),
	}
	if country.ID == 0 {
		id, err := strconv.Atoi(fallbackID)
		if err == nil {
			country.ID = id
		}
	}
	return country, country.ID != 0
}

func rawInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		value, _ := strconv.Atoi(number.String())
		return value
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		value, _ := strconv.Atoi(strings.TrimSpace(text))
		return value
	}
	return 0
}

func rawFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		value, _ := strconv.ParseFloat(number.String(), 64)
		return value
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		value, _ := strconv.ParseFloat(strings.TrimSpace(text), 64)
		return value
	}
	return 0
}

func rawBool(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	text := strings.ToLower(strings.TrimSpace(rawValueString(raw)))
	return text == "1" || text == "true" || text == "yes"
}

func rawString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func rawValueString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return ""
	}
	if value := rawString(raw); value != "" {
		return value
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		return number.String()
	}
	return strings.Trim(string(raw), `"`)
}

func parseSMSStatusResponse(text string) (*SMSCodeResult, error) {
	text = strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(text, StatusOK+":"):
		return &SMSCodeResult{
			Code:   strings.TrimSpace(strings.TrimPrefix(text, StatusOK+":")),
			Status: StatusOK,
		}, nil
	case strings.HasPrefix(text, StatusWaitRetry+":"):
		return &SMSCodeResult{
			Code:   strings.TrimSpace(strings.TrimPrefix(text, StatusWaitRetry+":")),
			Status: StatusWaitRetry,
		}, nil
	case text == StatusWaitCode:
		return &SMSCodeResult{Status: StatusWaitCode}, nil
	case text == StatusWaitResend:
		return &SMSCodeResult{Status: StatusWaitResend}, nil
	case text == StatusCancel:
		return &SMSCodeResult{Status: StatusCancel}, nil
	case strings.Contains(text, "BAD_KEY"):
		return nil, fmt.Errorf("invalid API key")
	case strings.Contains(text, "BAD_ACTION"):
		return nil, fmt.Errorf("invalid action")
	case strings.Contains(text, "NO_ACTIVATION"):
		return nil, fmt.Errorf("activation not found")
	default:
		return nil, fmt.Errorf("unexpected status response: %s", text)
	}
}

func parseSetStatusResponse(text string) error {
	text = strings.TrimSpace(text)
	switch text {
	case StatusAccessReady, StatusAccessRetry, StatusActivation, StatusAccessCancel:
		return nil
	case "BAD_KEY":
		return fmt.Errorf("invalid API key")
	case "BAD_ACTION":
		return fmt.Errorf("invalid action")
	case "BAD_STATUS":
		return fmt.Errorf("invalid activation status")
	case "NO_ACTIVATION":
		return fmt.Errorf("activation not found")
	case "EARLY_CANCEL_DENIED":
		return fmt.Errorf("early cancel denied")
	default:
		return fmt.Errorf("unexpected response: %s", text)
	}
}

func providerPlainTextError(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("empty sms provider response")
	}
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return nil
	}
	switch {
	case strings.Contains(text, "BAD_KEY"):
		return fmt.Errorf("invalid API key")
	case strings.Contains(text, "BAD_ACTION"):
		return fmt.Errorf("invalid action")
	case strings.Contains(text, "BAD_SERVICE"):
		return fmt.Errorf("invalid service")
	case strings.Contains(text, "BAD_COUNTRY"):
		return fmt.Errorf("invalid country")
	case strings.Contains(text, "NO_NUMBERS"):
		return fmt.Errorf("no phone numbers available")
	case strings.Contains(text, "NO_BALANCE"):
		return fmt.Errorf("insufficient SMS balance")
	case strings.Contains(text, "WRONG_MAX_PRICE"):
		return fmt.Errorf("wrong max price")
	default:
		return fmt.Errorf("sms provider response: %s", truncateProviderText(text))
	}
}

func truncateProviderText(text string) string {
	const maxLen = 240
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
