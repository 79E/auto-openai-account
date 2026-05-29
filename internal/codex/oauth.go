package codex

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	AuthURL     = "https://auth.openai.com/oauth/authorize"
	TokenURL    = "https://auth.openai.com/oauth/token"
	ClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	RedirectURI = "http://localhost:1455/auth/callback"
	Scopes      = "openid profile email offline_access api.connectors.read api.connectors.invoke"
)

type PKCECodes struct {
	CodeVerifier  string
	CodeChallenge string
}

func GeneratePKCECodes() (*PKCECodes, error) {
	verifierBytes := make([]byte, 96)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("generate code verifier failed: %w", err)
	}
	codeVerifier := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(verifierBytes)

	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])

	return &PKCECodes{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
	}, nil
}

func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func BuildAuthURL(state string, pkceCodes *PKCECodes) string {
	params := url.Values{
		"response_type":              {"code"},
		"client_id":                  {ClientID},
		"redirect_uri":               {RedirectURI},
		"scope":                      {Scopes},
		"state":                      {state},
		"code_challenge":             {pkceCodes.CodeChallenge},
		"code_challenge_method":      {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
	}
	return fmt.Sprintf("%s?%s", AuthURL, params.Encode())
}

type TokenResponse struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type OAuthResult struct {
	Code  string
	State string
	Error string
}

type CallbackServer struct {
	port     int
	server   *http.Server
	resultCh chan *OAuthResult
}

func NewCallbackServer(port int) *CallbackServer {
	return &CallbackServer{
		port:     port,
		resultCh: make(chan *OAuthResult, 1),
	}
}

func (s *CallbackServer) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return fmt.Errorf("port %d is in use: %w", s.port, err)
	}
	listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", s.handleCallback)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("callback server error: %v\n", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	return nil
}

func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if errParam := query.Get("error"); errParam != "" {
		s.resultCh <- &OAuthResult{Error: errParam}
		http.Error(w, fmt.Sprintf("authorization failed: %s", errParam), http.StatusBadRequest)
		return
	}

	code := query.Get("code")
	if code == "" {
		s.resultCh <- &OAuthResult{Error: "missing_code"}
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	state := query.Get("state")
	if state == "" {
		s.resultCh <- &OAuthResult{Error: "missing_state"}
		http.Error(w, "missing state parameter", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<html><body><h1>✅ Authorization successful</h1><p>You can close this window</p></body></html>`))

	s.resultCh <- &OAuthResult{Code: code, State: state}
}

func (s *CallbackServer) WaitForResult(timeout time.Duration) (*OAuthResult, error) {
	select {
	case result := <-s.resultCh:
		return result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("callback timeout (%v)", timeout)
	}
}

func (s *CallbackServer) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}

func ExchangeCodeForToken(ctx context.Context, code, state, expectedState string, pkceCodes *PKCECodes) (*TokenResponse, error) {
	if state != expectedState {
		return nil, fmt.Errorf("state mismatch: expected=%s, got=%s", expectedState, state)
	}

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {ClientID},
		"code":          {code},
		"redirect_uri":  {RedirectURI},
		"code_verifier": {pkceCodes.CodeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response failed: %w", err)
	}

	return &tokenResp, nil
}