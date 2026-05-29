package smsbiz

import "testing"

func TestParseCountriesArray(t *testing.T) {
	countries, err := parseCountries([]byte(`[
		{"id":38,"eng":"United States","chn":"美国"},
		{"id":2,"eng":"Kazakhstan","chn":"哈萨克斯坦"}
	]`))
	if err != nil {
		t.Fatalf("parseCountries returned error: %v", err)
	}
	if len(countries) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(countries))
	}
	if countries[0].ID != 38 || countries[0].Chn != "美国" {
		t.Fatalf("unexpected first country: %+v", countries[0])
	}
}

func TestParseCountriesWrappedObject(t *testing.T) {
	countries, err := parseCountries([]byte(`{
		"status": "success",
		"countries": {
			"38": {"eng":"United States","chn":"美国"},
			"1003": {"eng":"Bermuda","chn":"百慕大"}
		}
	}`))
	if err != nil {
		t.Fatalf("parseCountries returned error: %v", err)
	}
	if len(countries) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(countries))
	}
	ids := map[int]bool{}
	for _, country := range countries {
		ids[country.ID] = true
	}
	if !ids[38] || !ids[1003] {
		t.Fatalf("expected ids from object keys, got %+v", countries)
	}
}

func TestParseCountriesTopLevelObject(t *testing.T) {
	countries, err := parseCountries([]byte(`{
		"38": {"id":38,"eng":"United States","chn":"美国"},
		"1003": {"id":1003,"eng":"Bermuda","chn":"百慕大"}
	}`))
	if err != nil {
		t.Fatalf("parseCountries returned error: %v", err)
	}
	if len(countries) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(countries))
	}
}

func TestParseCountriesStringID(t *testing.T) {
	countries, err := parseCountries([]byte(`{
		"38": {"id":"38","eng":"United States","chn":"美国"},
		"1003": {"id":"1003","eng":"Bermuda","chn":"百慕大"}
	}`))
	if err != nil {
		t.Fatalf("parseCountries returned error: %v", err)
	}
	ids := map[int]bool{}
	for _, country := range countries {
		ids[country.ID] = true
	}
	if !ids[38] || !ids[1003] {
		t.Fatalf("expected string ids to parse as ints, got %+v", countries)
	}
}

func TestRawValueStringSupportsNumberAndString(t *testing.T) {
	cases := map[string]string{
		`123456789`:     "123456789",
		`"123456789"`:   "123456789",
		`79584123456`:   "79584123456",
		`"79584123456"`: "79584123456",
	}
	for raw, want := range cases {
		if got := rawValueString([]byte(raw)); got != want {
			t.Fatalf("rawValueString(%s) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseSMSBowerActivationRealShape(t *testing.T) {
	activation, err := parseSMSBowerActivation([]byte(`{
		"activationId": 342949143,
		"phoneNumber": "233533211902",
		"activationCost": "0.054",
		"countryCode": "38",
		"canGetAnotherSms": "1",
		"activationTime": "2026-05-27 20:42:13",
		"activationOperator": null
	}`))
	if err != nil {
		t.Fatalf("parseSMSBowerActivation returned error: %v", err)
	}
	if activation.ActivationID != "342949143" {
		t.Fatalf("unexpected activation id: %+v", activation)
	}
	if activation.PhoneNumber != "233533211902" {
		t.Fatalf("unexpected phone number: %+v", activation)
	}
	if activation.ActivationCost != 0.054 {
		t.Fatalf("unexpected activation cost: %+v", activation)
	}
	if activation.CountryCode != 38 || !activation.CanGetAnotherSms {
		t.Fatalf("unexpected country/can retry fields: %+v", activation)
	}
	if activation.ActivationOperator != "" {
		t.Fatalf("expected null operator to be empty, got %q", activation.ActivationOperator)
	}
}

func TestParseSMSBowerActivationPlainTextError(t *testing.T) {
	_, err := parseSMSBowerActivation([]byte("NO_NUMBERS"))
	if err == nil {
		t.Fatal("expected NO_NUMBERS to return error")
	}
	if err.Error() != "no phone numbers available" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseHeroSMSActivationRealShape(t *testing.T) {
	activation, err := parseHeroSMSActivation([]byte(`{
		"activationId": "425244763",
		"phoneNumber": "77479160054",
		"activationCost": 1.25,
		"currency": 840,
		"countryCode": 2,
		"countryPhoneCode": 7,
		"canGetAnotherSms": true,
		"activationTime": "2026-05-27 20:48:43",
		"activationEndTime": "2026-05-27 21:08:43",
		"activationOperator": "tele2",
		"serviceCode": "tg",
		"subtype": 1,
		"status": 4
	}`))
	if err != nil {
		t.Fatalf("parseHeroSMSActivation returned error: %v", err)
	}
	if activation.ActivationID != "425244763" || activation.PhoneNumber != "77479160054" {
		t.Fatalf("unexpected activation identifiers: %+v", activation)
	}
	if activation.ActivationCost != 1.25 || activation.CountryCode != 2 || activation.CountryPhoneCode != 7 {
		t.Fatalf("unexpected numeric fields: %+v", activation)
	}
	if !activation.CanGetAnotherSms || activation.ActivationOperator != "tele2" {
		t.Fatalf("unexpected bool/operator fields: %+v", activation)
	}
}

func TestParseSMSStatusResponse(t *testing.T) {
	cases := map[string]struct {
		status string
		code   string
	}{
		"STATUS_WAIT_CODE":         {status: StatusWaitCode},
		"STATUS_WAIT_RETRY:100001": {status: StatusWaitRetry, code: "100001"},
		"STATUS_WAIT_RESEND":       {status: StatusWaitResend},
		"STATUS_CANCEL":            {status: StatusCancel},
		"STATUS_OK:100001":         {status: StatusOK, code: "100001"},
	}
	for raw, want := range cases {
		got, err := parseSMSStatusResponse(raw)
		if err != nil {
			t.Fatalf("parseSMSStatusResponse(%q) returned error: %v", raw, err)
		}
		if got.Status != want.status || got.Code != want.code {
			t.Fatalf("parseSMSStatusResponse(%q) = %+v, want status=%s code=%s", raw, got, want.status, want.code)
		}
	}
}

func TestParseSetStatusResponse(t *testing.T) {
	for _, raw := range []string{"ACCESS_RETRY_GET", "ACCESS_ACTIVATION", "ACCESS_CANCEL"} {
		if err := parseSetStatusResponse(raw); err != nil {
			t.Fatalf("parseSetStatusResponse(%q) returned error: %v", raw, err)
		}
	}
}
