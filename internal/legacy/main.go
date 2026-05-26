package legacy

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	authBase                 = "https://auth.openai.com"
	platformBase             = "https://platform.openai.com"
	platformOAuthClientID    = "app_2SKx67EdpoN0G6j64rFvigXD"
	platformOAuthRedirectURI = platformBase + "/auth/callback"
	platformOAuthAudience    = "https://api.openai.com/v1"
	platformAuth0Client      = "eyJuYW1lIjoiYXV0aDAtc3BhLWpzIiwidmVyc2lvbiI6IjEuMjEuMCJ9"
	userAgent                = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"
	secCHUA                  = `"Google Chrome";v="145", "Not?A_Brand";v="8", "Chromium";v="145"`
	secCHUAFullVersionList   = `"Chromium";v="145.0.0.0", "Not:A-Brand";v="99.0.0.0", "Google Chrome";v="145.0.0.0"`
	sentinelBase             = "https://sentinel.openai.com"
	sentinelSDK              = sentinelBase + "/sentinel/20260124ceb8/sdk.js"
	sentinelMaxAttempts      = 500000
	sentinelErrorPrefix      = "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D"
)

var (
	firstNames  = []string{"James", "Robert", "John", "Michael", "David", "Mary", "Emma", "Olivia"}
	lastNames   = []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller"}
	debugMode   bool
	stdinReader *bufio.Reader
)

type worker struct {
	client     *http.Client
	deviceID   string
	mail       string
	otpFetcher func(context.Context) (string, error)
}

type appConfig struct {
	DefaultProxy        string `json:"default_proxy"`
	PasswordMode        string `json:"password_mode"`
	DefaultPassword     string `json:"default_password"`
	DefaultManualPrompt string `json:"default_manual_prompt"`
}

func main() {
	// Legacy business logic is embedded as a library in this project.
	// The real application entrypoint is apps/server/main.go.
}

func runCLI() {
	randomSeed()
	debugMode = hasFlag("--debug")
	loginOnly := hasFlag("--login-only")
	registerOnly := hasFlag("--register-only")
	cfg := loadConfig()
	stdinReader = bufio.NewReader(os.Stdin)

	proxyPrompt := "请输入代理地址，例如 http://127.0.0.1:7890: "
	if cfg.DefaultProxy != "" {
		proxyPrompt = fmt.Sprintf("请输入代理地址，直接回车使用默认值 %s: ", cfg.DefaultProxy)
	}
	proxy := mustReadLine(stdinReader, proxyPrompt)
	if proxy == "" {
		proxy = cfg.DefaultProxy
	}
	email := mustReadLine(stdinReader, "请输入邮箱地址: ")
	passwordPrompt := "请输入密码，直接回车则自动生成: "
	if cfg.PasswordMode == "manual" && cfg.DefaultPassword != "" {
		passwordPrompt = fmt.Sprintf("请输入密码，直接回车使用默认值 %s: ", maskPassword(cfg.DefaultPassword))
	} else if cfg.PasswordMode == "random" {
		passwordPrompt = "请输入密码，直接回车则使用随机密码: "
	}
	password := mustReadOptional(stdinReader, passwordPrompt)
	if strings.TrimSpace(password) == "" {
		if cfg.PasswordMode == "manual" && cfg.DefaultPassword != "" {
			password = cfg.DefaultPassword
		} else {
			password = randomPassword(16)
			fmt.Printf("已自动生成密码: %s\n", password)
		}
	}
	if strings.TrimSpace(password) == "" {
		password = randomPassword(16)
		fmt.Printf("已自动生成密码: %s\n", password)
	}

	ctx := context.Background()
	w, err := newWorker(proxy, email)
	if err != nil {
		fatalf("初始化失败: %v", err)
	}
	defer w.close()
	w.debugCookies("init")

	if loginOnly {
		fmt.Println("[login-only 模式] 跳过注册流程，直接登录换取 token")
		tokens, err := w.loginAndExchangeTokens(ctx, email, password)
		if err != nil {
			fatalf("换取 token 失败: %v", err)
		}
		pretty, _ := json.MarshalIndent(map[string]any{
			"email":         email,
			"password":      password,
			"access_token":  tokens["access_token"],
			"refresh_token": tokens["refresh_token"],
			"id_token":      tokens["id_token"],
		}, "", "  ")
		fmt.Println(string(pretty))
		return
	}

	if err := w.platformAuthorize(ctx, email); err != nil {
		fatalf("platform authorize 失败: %v", err)
	}
	if err := w.registerUser(ctx, email, password); err != nil {
		fatalf("提交注册密码失败: %v", err)
	}
	if err := w.sendOTP(ctx); err != nil {
		fatalf("发送验证码失败: %v", err)
	}

	code := mustReadLine(stdinReader, "请输入邮箱验证码: ")
	if err := w.validateOTP(ctx, code); err != nil {
		fatalf("验证码校验失败: %v", err)
	}

	name := randomName()
	birthdate := randomBirthdate()
	if err := w.createAccount(ctx, name, birthdate); err != nil {
		fatalf("创建账号资料失败: %v", err)
	}

	if registerOnly {
		fmt.Println("[register-only 模式] 注册完成，跳过 token 换取")
		pretty, _ := json.MarshalIndent(map[string]any{
			"email":    email,
			"password": password,
			"name":     name,
			"status":   "registered",
		}, "", "  ")
		fmt.Println(string(pretty))
		return
	}

	tokens, err := w.loginAndExchangeTokens(ctx, email, password)
	if err != nil {
		fatalf("换取 token 失败: %v", err)
	}

	pretty, _ := json.MarshalIndent(map[string]any{
		"email":         email,
		"password":      password,
		"access_token":  tokens["access_token"],
		"refresh_token": tokens["refresh_token"],
		"id_token":      tokens["id_token"],
	}, "", "  ")
	fmt.Println(string(pretty))
}

func newWorker(proxy, email string) (*worker, error) {
	return newWorkerWithOTP(proxy, email, nil)
}

func newWorkerWithOTP(proxy, email string, otpFetcher func(context.Context) (string, error)) (*worker, error) {
	deviceID := NewUUID()
	client, err := httpClientForProxy(strings.TrimSpace(proxy), 60*time.Second, deviceID)
	if err != nil {
		return nil, err
	}
	return &worker{client: client, deviceID: deviceID, mail: email, otpFetcher: otpFetcher}, nil
}

func httpClientForProxy(proxy string, timeout time.Duration, deviceID string) (*http.Client, error) {
	return httpClientForProxyStub(proxy, timeout, deviceID)
}

func httpClientForProxyStub(proxy string, timeout time.Duration, deviceID string) (*http.Client, error) {
	return browserHTTPClient(proxy, timeout), nil
}

func (w *worker) close() {
	if w.client != nil {
		w.client.CloseIdleConnections()
	}
}

func (w *worker) platformAuthorize(ctx context.Context, email string) error {
	values := authorizeParams(email, w.deviceID, randomToken(), randomToken(), pkceChallenge())
	status, payload, err := w.request(ctx, http.MethodGet, authBase+"/api/accounts/authorize?"+values.Encode(), nil, w.navigateHeaders(platformBase+"/"), true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("platform_authorize_http_%d%s", status, responseDetail(payload))
	}
	if finalURL := Clean(payload["_final_url"]); strings.Contains(finalURL, "/log-in/password") {
		return fmt.Errorf("platform_authorize_entered_login_flow: upstream sent this email to login password page instead of create-account password page; email may already exist or be routed to an existing provider, final_url=%s", finalURL)
	}
	return nil
}

func (w *worker) registerUser(ctx context.Context, email, password string) error {
	headers := w.jsonHeaders(authBase + "/create-account/password")
	token, err := w.buildSentinelToken(ctx, "username_password_create")
	if err != nil {
		return err
	}
	headers["openai-sentinel-token"] = token
	status, payload, err := w.request(ctx, http.MethodPost, authBase+"/api/accounts/user/register", map[string]any{
		"username": email,
		"password": password,
	}, headers, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		if failedToCreateAccount(payload) {
			fmt.Fprintln(os.Stderr, "邮箱域名很可能因滥用被封禁，请更换邮箱域名")
		}
		return fmt.Errorf("user_register_http_%d%s", status, responseDetail(payload))
	}
	return nil
}

func (w *worker) sendOTP(ctx context.Context) error {
	status, _, err := w.request(ctx, http.MethodGet, authBase+"/api/accounts/email-otp/send", nil, w.navigateHeaders(authBase+"/create-account/password"), true)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusFound {
		return fmt.Errorf("send_otp_http_%d", status)
	}
	return nil
}

func (w *worker) validateOTP(ctx context.Context, code string) error {
	status, payload, err := w.request(ctx, http.MethodPost, authBase+"/api/accounts/email-otp/validate", map[string]any{"code": code}, w.jsonHeaders(authBase+"/email-verification"), true)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	headers := w.jsonHeaders(authBase + "/email-verification")
	token, tokenErr := w.buildSentinelToken(ctx, "authorize_continue")
	if tokenErr != nil {
		return fmt.Errorf("validate_otp_http_%d; sentinel fallback failed: %w", status, tokenErr)
	}
	headers["openai-sentinel-token"] = token
	status, payload, err = w.request(ctx, http.MethodPost, authBase+"/api/accounts/email-otp/validate", map[string]any{"code": code}, headers, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("validate_otp_http_%d%s", status, responseDetail(payload))
	}
	return nil
}

func (w *worker) createAccount(ctx context.Context, name, birthdate string) error {
	headers := w.jsonHeaders(authBase + "/about-you")
	token, err := w.buildSentinelToken(ctx, "oauth_create_account")
	if err != nil {
		return err
	}
	headers["openai-sentinel-token"] = token
	status, payload, err := w.request(ctx, http.MethodPost, authBase+"/api/accounts/create_account", map[string]any{
		"name":      name,
		"birthdate": birthdate,
	}, headers, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusFound {
		if failedToCreateAccount(payload) {
			fmt.Fprintln(os.Stderr, "邮箱域名很可能因滥用被封禁，请更换邮箱域名")
		}
		return fmt.Errorf("create_account_http_%d%s", status, responseDetail(payload))
	}
	return nil
}

func (w *worker) validateOTPCode(ctx context.Context, code string) (map[string]any, error) {
	status, payload, err := w.request(ctx, http.MethodPost, authBase+"/api/accounts/email-otp/validate", map[string]any{"code": code}, w.jsonHeaders(authBase+"/email-verification"), true)
	if err != nil {
		return nil, err
	}
	if status == http.StatusOK {
		return payload, nil
	}
	headers := w.jsonHeaders(authBase + "/email-verification")
	token, tokenErr := w.buildSentinelToken(ctx, "authorize_continue")
	if tokenErr != nil {
		return nil, fmt.Errorf("validate_otp_http_%d; sentinel fallback failed: %w", status, tokenErr)
	}
	headers["openai-sentinel-token"] = token
	status, payload, err = w.request(ctx, http.MethodPost, authBase+"/api/accounts/email-otp-validate", map[string]any{"code": code}, headers, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("validate_otp_http_%d", status)
	}
	return payload, nil
}

func (w *worker) loginAndExchangeTokens(ctx context.Context, email, password string) (map[string]any, error) {
	codeVerifier, codeChallenge := generatePKCE()
	values := authorizeParams(email, w.deviceID, randomToken(), randomToken(), codeChallenge)
	authorizeLogin := func() (string, error) {
		status, payload, headers, err := w.requestDetailed(ctx, http.MethodGet, authBase+"/api/accounts/authorize?"+values.Encode(), nil, w.navigateHeaders(platformBase+"/"), true)
		if err != nil {
			return "", err
		}
		if status != http.StatusOK {
			return "", fmt.Errorf("platform_login_authorize_http_%d", status)
		}
		if code := extractOAuthCode(payload, headers); code != "" {
			return code, nil
		}
		return "", nil
	}
	oauthCode, err := authorizeLogin()
	if err != nil {
		return nil, err
	}
	if oauthCode != "" {
		status, tokenPayload, err := w.requestForm(ctx, authBase+"/oauth/token", url.Values{
			"grant_type":    []string{"authorization_code"},
			"code":          []string{oauthCode},
			"redirect_uri":  []string{platformOAuthRedirectURI},
			"client_id":     []string{platformOAuthClientID},
			"code_verifier": []string{codeVerifier},
		})
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("oauth_token_http_%d", status)
		}
		accessToken := Clean(tokenPayload["access_token"])
		refreshToken := Clean(tokenPayload["refresh_token"])
		idToken := Clean(tokenPayload["id_token"])
		if accessToken == "" || refreshToken == "" || idToken == "" {
			return nil, fmt.Errorf("token exchange response missing access_token, refresh_token, or id_token")
		}
		return tokenPayload, nil
	}
	logStep(email, "登录换 token: 提交邮箱 device_id=%s", w.deviceID)
	status, payload, err := w.submitLoginEmail(ctx, email)
	if err != nil {
		logStep(email, "登录换 token: 提交邮箱请求失败 err=%v", err)
		return nil, err
	}
	logStep(email, "登录换 token: 提交邮箱返回 status=%d%s", status, responseDetail(payload))
	if status == http.StatusConflict {
		logStep(email, "登录换 token: 邮箱提交返回 conflict，尝试直接授权")
		if oauthCode, err := authorizeLogin(); err != nil {
			return nil, err
		} else if oauthCode != "" {
			status, tokenPayload, err := w.requestForm(ctx, authBase+"/oauth/token", url.Values{
				"grant_type":    []string{"authorization_code"},
				"code":          []string{oauthCode},
				"redirect_uri":  []string{platformOAuthRedirectURI},
				"client_id":     []string{platformOAuthClientID},
				"code_verifier": []string{codeVerifier},
			})
			if err != nil {
				return nil, err
			}
			if status != http.StatusOK {
				return nil, fmt.Errorf("oauth_token_http_%d", status)
			}
			accessToken := Clean(tokenPayload["access_token"])
			refreshToken := Clean(tokenPayload["refresh_token"])
			idToken := Clean(tokenPayload["id_token"])
			if accessToken == "" || refreshToken == "" || idToken == "" {
				return nil, fmt.Errorf("token exchange response missing access_token, refresh_token, or id_token")
			}
			return tokenPayload, nil
		}
		logStep(email, "登录换 token: 直接授权无 code，重新提交邮箱")
		status, payload, err = w.submitLoginEmail(ctx, email)
		if err != nil {
			logStep(email, "登录换 token: 重新提交邮箱请求失败 err=%v", err)
			return nil, err
		}
		logStep(email, "登录换 token: 重新提交邮箱返回 status=%d%s", status, responseDetail(payload))
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("email_submit_http_%d%s", status, responseDetail(payload))
	}
	continueURL := Clean(payload["continue_url"])
	page := StringMap(payload["page"])
	pagePayload := StringMap(page["payload"])
	pageType := Clean(page["type"])
	passwordlessDisabled := ToBool(pagePayload["passwordless_disabled"])
	logStep(email, "登录换 token: 邮箱提交后页面 page_type=%s continue_url=%s passwordless_disabled=%t", pageType, continueURL, passwordlessDisabled)
	if pageType == "login_password" && !passwordlessDisabled {
		logStep(email, "登录换 token: 当前账号支持邮箱验证码登录，发送登录验证码")
		if err := w.sendLoginOTP(ctx, continueURL); err != nil {
			logStep(email, "登录换 token: 发送登录验证码失败，尝试回退密码校验 err=%v", err)
		} else {
			code, waitErr := w.waitRegisterCode(ctx)
			if waitErr != nil {
				return nil, waitErr
			}
			if code == "" {
				return nil, fmt.Errorf("independent login waiting for verification code timed out")
			}
			logStep(email, "登录换 token: 已获取登录验证码 code=%s，提交校验", code)
			otpPayload, otpErr := w.validateOTPCode(ctx, code)
			if otpErr != nil {
				logStep(email, "登录换 token: 登录验证码校验失败，尝试回退密码校验 err=%v", otpErr)
			} else {
				continueURL = firstNonEmpty(Clean(otpPayload["continue_url"]), continueURL)
				page = StringMap(otpPayload["page"])
				logStep(email, "登录换 token: 登录验证码校验通过 continue_url=%s page_type=%s", continueURL, Clean(page["type"]))
				return w.exchangeTokensFromContinueURL(ctx, continueURL, codeVerifier)
			}
		}
	}
	headers := w.jsonHeaders(authBase + "/log-in/password")
	logStep(email, "登录换 token: 构建 password_verify sentinel token")
	token, err := w.buildSentinelToken(ctx, "password_verify")
	if err != nil {
		logStep(email, "登录换 token: 构建 password_verify sentinel token 失败 err=%v", err)
		return nil, err
	}
	headers["openai-sentinel-token"] = token
	logStep(email, "登录换 token: 提交密码校验 password_len=%d password_hash=%s sentinel_token_len=%d", len(Clean(password)), shortSecretHash(password), len(token))
	status, payload, err = w.request(ctx, http.MethodPost, authBase+"/api/accounts/password/verify", map[string]any{
		"password": password,
	}, headers, false)
	if err != nil {
		logStep(email, "登录换 token: 密码校验请求失败 err=%v", err)
		return nil, err
	}
	logStep(email, "登录换 token: 密码校验返回 status=%d%s", status, responseDetail(payload))
	if status != http.StatusOK {
		return nil, fmt.Errorf("password_verify_http_%d%s", status, responseDetail(payload))
	}
	continueURL = Clean(payload["continue_url"])
	page = StringMap(payload["page"])
	logStep(email, "登录换 token: 密码校验通过 continue_url=%s page_type=%s", continueURL, Clean(page["type"]))
	if Clean(page["type"]) == "email_otp_verification" || strings.Contains(continueURL, "email-verification") || strings.Contains(continueURL, "email-otp") {
		code, waitErr := w.waitRegisterCode(ctx)
		if waitErr != nil {
			return nil, waitErr
		}
		if code == "" {
			return nil, fmt.Errorf("independent login waiting for verification code timed out")
		}
		otpPayload, otpErr := w.validateOTPCode(ctx, code)
		if otpErr != nil {
			return nil, otpErr
		}
		if next := Clean(otpPayload["continue_url"]); next != "" {
			continueURL = next
		}
	}
	if continueURL == "" {
		continueURL = authBase + "/sign-in-with-chatgpt/codex/consent"
	}
	return w.exchangeTokensFromContinueURL(ctx, continueURL, codeVerifier)
}

func (w *worker) exchangeTokensFromContinueURL(ctx context.Context, continueURL, codeVerifier string) (map[string]any, error) {
	if continueURL == "" {
		continueURL = authBase + "/sign-in-with-chatgpt/codex/consent"
	}
	code, err := w.followConsentForCode(ctx, continueURL)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, fmt.Errorf("token exchange callback code not found")
	}
	status, tokenPayload, err := w.requestForm(ctx, authBase+"/oauth/token", url.Values{
		"grant_type":    []string{"authorization_code"},
		"code":          []string{code},
		"redirect_uri":  []string{platformOAuthRedirectURI},
		"client_id":     []string{platformOAuthClientID},
		"code_verifier": []string{codeVerifier},
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("oauth_token_http_%d", status)
	}
	accessToken := Clean(tokenPayload["access_token"])
	refreshToken := Clean(tokenPayload["refresh_token"])
	idToken := Clean(tokenPayload["id_token"])
	if accessToken == "" || refreshToken == "" || idToken == "" {
		return nil, fmt.Errorf("token exchange response missing access_token, refresh_token, or id_token")
	}
	return tokenPayload, nil
}

func (w *worker) waitRegisterCode(ctx context.Context) (string, error) {
	if w != nil && w.otpFetcher != nil {
		return w.otpFetcher(ctx)
	}
	if stdinReader == nil {
		return "", nil
	}
	fmt.Print("请输入登录流程收到的邮箱验证码: ")
	line, err := stdinReader.ReadString('\n')
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (w *worker) submitLoginEmail(ctx context.Context, email string) (int, map[string]any, error) {
	headers := w.jsonHeaders(authBase + "/log-in?usernameKind=email")
	token, err := w.buildSentinelToken(ctx, "authorize_continue")
	if err != nil {
		return 0, nil, err
	}
	headers["openai-sentinel-token"] = token
	return w.request(ctx, http.MethodPost, authBase+"/api/accounts/authorize/continue", map[string]any{
		"username": map[string]any{"kind": "email", "value": email},
	}, headers, false)
}

func (w *worker) sendLoginOTP(ctx context.Context, referer string) error {
	referer = firstNonEmpty(Clean(referer), authBase+"/log-in/password")
	status, payload, err := w.request(ctx, http.MethodGet, referer, nil, w.navigateHeaders(authBase+"/api/accounts/authorize/continue"), true)
	if err != nil {
		return err
	}
	logStep(w.mail, "登录换 token: 进入登录验证码页面返回 status=%d%s", status, responseDetail(payload))
	if status != http.StatusOK || strings.Contains(Clean(payload["_final_url"]), "/error") {
		return fmt.Errorf("login_otp_page_http_%d%s", status, responseDetail(payload))
	}
	if finalURL := Clean(payload["_final_url"]); finalURL != "" {
		referer = finalURL
	}
	status, payload, err = w.request(ctx, http.MethodGet, authBase+"/api/accounts/email-otp/send", nil, w.navigateHeaders(referer), false)
	if err != nil {
		return err
	}
	logStep(w.mail, "登录换 token: 发送登录验证码返回 status=%d%s", status, responseDetail(payload))
	if status == http.StatusFound {
		location := Clean(payload["_location"])
		if location == "" {
			return fmt.Errorf("send_login_otp_http_302_missing_location%s", responseDetail(payload))
		}
		if strings.Contains(location, "/error") {
			return fmt.Errorf("send_login_otp_error_redirect location=%s%s", location, responseDetail(payload))
		}
		logStep(w.mail, "登录换 token: 发送登录验证码 302 location=%s", location)
	}
	if strings.Contains(Clean(payload["_final_url"]), "/error") {
		return fmt.Errorf("send_login_otp_error_redirect%s", responseDetail(payload))
	}
	if status != http.StatusOK && status != http.StatusFound {
		return fmt.Errorf("send_login_otp_http_%d%s", status, responseDetail(payload))
	}
	return nil
}

func (w *worker) followConsentForCode(ctx context.Context, consentURL string) (string, error) {
	current := consentURL
	if strings.HasPrefix(current, "/") {
		current = authBase + current
	}
	noRedirect := *w.client
	noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	for i := 0; i < 10; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return "", err
		}
		for key, value := range w.navigateHeaders(current) {
			req.Header.Set(key, value)
		}
		resp, err := noRedirect.Do(req)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		if code := oauthCode(resp.Request.URL.String()); code != "" {
			return code, nil
		}
		location := strings.TrimSpace(resp.Header.Get("Location"))
		if code := oauthCode(location); code != "" {
			return code, nil
		}
		if location == "" || (resp.StatusCode < 300 || resp.StatusCode >= 400) {
			break
		}
		next, err := resolveLocation(current, location)
		if err != nil {
			return "", err
		}
		current = next
	}
	return w.selectWorkspaceForConsentCode(ctx, consentURL)
}

func (w *worker) selectWorkspaceForConsentCode(ctx context.Context, consentURL string) (string, error) {
	workspaceID := w.authSessionWorkspaceID()
	if workspaceID == "" {
		return "", nil
	}
	if strings.HasPrefix(consentURL, "/") {
		consentURL = authBase + consentURL
	}
	headers := w.jsonHeaders(consentURL)
	status, wsPayload, wsHeaders, err := w.requestDetailed(ctx, http.MethodPost, authBase+"/api/accounts/workspace/select", map[string]any{
		"workspace_id": workspaceID,
	}, headers, false)
	if err != nil {
		return "", err
	}
	if code := oauthCode(wsHeaders.Get("Location")); code != "" {
		return code, nil
	}
	if code := Clean(wsPayload["continue_url"]); oauthCode(code) != "" {
		return oauthCode(code), nil
	}
	if status < 200 || status >= 400 {
		return "", fmt.Errorf("workspace_select_http_%d", status)
	}
	data := StringMap(wsPayload["data"])
	orgs := AsMapSlice(data["orgs"])
	if len(orgs) == 0 {
		return "", nil
	}
	orgID := Clean(orgs[0]["id"])
	if orgID == "" {
		return "", nil
	}
	orgBody := map[string]any{"org_id": orgID}
	if projects := AsMapSlice(orgs[0]["projects"]); len(projects) > 0 {
		if projectID := Clean(projects[0]["id"]); projectID != "" {
			orgBody["project_id"] = projectID
		}
	}
	orgReferer := firstNonEmpty(Clean(wsPayload["continue_url"]), consentURL)
	status, orgPayload, orgHeaders, err := w.requestDetailed(ctx, http.MethodPost, authBase+"/api/accounts/organization/select", orgBody, w.jsonHeaders(orgReferer), false)
	if err != nil {
		return "", err
	}
	if code := oauthCode(orgHeaders.Get("Location")); code != "" {
		return code, nil
	}
	if code := oauthCode(Clean(orgPayload["continue_url"])); code != "" {
		return code, nil
	}
	if status < 200 || status >= 400 {
		return "", fmt.Errorf("organization_select_http_%d", status)
	}
	return "", nil
}

func (w *worker) authSessionWorkspaceID() string {
	if w.client == nil || w.client.Jar == nil {
		return ""
	}
	authURL, err := url.Parse(authBase)
	if err != nil {
		return ""
	}
	var raw string
	for _, cookie := range w.client.Jar.Cookies(authURL) {
		if cookie.Name == "oai-client-auth-session" {
			raw = cookie.Value
			break
		}
	}
	if raw == "" {
		return ""
	}
	firstPart := strings.Split(raw, ".")[0]
	padding := len(firstPart) % 4
	if padding != 0 {
		firstPart += strings.Repeat("=", 4-padding)
	}
	data, err := base64.URLEncoding.DecodeString(firstPart)
	if err != nil {
		return ""
	}
	var payload map[string]any
	if json.Unmarshal(data, &payload) != nil {
		return ""
	}
	workspaces := AsMapSlice(payload["workspaces"])
	if len(workspaces) == 0 {
		return ""
	}
	return Clean(workspaces[0]["id"])
}

func (w *worker) buildSentinelToken(ctx context.Context, flow string) (string, error) {
	generator := newSentinelTokenGenerator(w.deviceID, userAgent)
	reqPayload := map[string]any{"p": generator.generateRequirementsToken(), "id": w.deviceID, "flow": flow}
	body, err := compactJSONBytes(reqPayload)
	if err != nil {
		return "", err
	}
	status, payload, err := w.requestRawJSON(ctx, http.MethodPost, sentinelBase+"/backend-api/sentinel/req", body, sentinelHeaders())
	if err != nil {
		return "", err
	}
	challengeToken := Clean(payload["token"])
	if status != http.StatusOK || challengeToken == "" {
		return "", fmt.Errorf("sentinel_req_failed_%d", status)
	}
	proof := StringMap(payload["proofofwork"])
	var pValue string
	if ToBool(proof["required"]) && Clean(proof["seed"]) != "" {
		pValue = generator.generateToken(Clean(proof["seed"]), firstNonEmpty(Clean(proof["difficulty"]), "0"))
	} else {
		pValue = generator.generateRequirementsToken()
	}
	tokenPayload := map[string]any{"p": pValue, "t": "", "c": challengeToken, "id": w.deviceID, "flow": flow}
	data, err := compactJSONBytes(tokenPayload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (w *worker) requestRawJSON(ctx context.Context, method, target string, body []byte, headers map[string]string) (int, map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(string(body)))
		if err != nil {
			return 0, nil, err
		}
		for key, value := range headers {
			if strings.TrimSpace(value) != "" {
				req.Header.Set(key, value)
			}
		}
		resp, err := w.client.Do(req)
		if err != nil {
			debugRequestError(method, target, attempt, err)
			lastErr = err
			if attempt < 2 {
				time.Sleep(time.Second)
				continue
			}
			return 0, nil, err
		}
		defer resp.Body.Close()
		debugResponse(method, target, attempt, resp)
		w.debugCookies("raw-json")
		payload := map[string]any{}
		_ = DecodeJSON(resp.Body, &payload)
		return resp.StatusCode, payload, nil
	}
	if lastErr != nil {
		return 0, nil, lastErr
	}
	return 0, nil, fmt.Errorf("raw request failed")
}

func (w *worker) request(ctx context.Context, method, target string, payload any, headers map[string]string, followRedirects bool) (int, map[string]any, error) {
	var body io.Reader
	var bodyData []byte
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		bodyData = data
	}
	client := w.client
	if !followRedirects {
		clone := *w.client
		clone.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
		client = &clone
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if payload != nil {
			body = strings.NewReader(string(bodyData))
		}
		req, err := http.NewRequestWithContext(ctx, method, target, body)
		if err != nil {
			return 0, nil, err
		}
		for key, value := range headers {
			if strings.TrimSpace(value) != "" {
				req.Header.Set(key, value)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			debugRequestError(method, target, attempt, err)
			lastErr = err
			if attempt < 2 {
				time.Sleep(time.Second)
				continue
			}
			return 0, nil, err
		}
		defer resp.Body.Close()
		debugResponse(method, target, attempt, resp)
		w.debugCookies("request")
		payloadMap := map[string]any{}
		if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
			_ = DecodeJSON(resp.Body, &payloadMap)
		} else {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			if len(data) > 0 {
				payloadMap["body"] = string(data)
			}
		}
		if resp.Request != nil && resp.Request.URL != nil {
			payloadMap["_final_url"] = resp.Request.URL.String()
		}
		if location := strings.TrimSpace(resp.Header.Get("Location")); location != "" {
			payloadMap["_location"] = location
		}
		return resp.StatusCode, payloadMap, nil
	}
	if lastErr != nil {
		return 0, nil, lastErr
	}
	return 0, nil, fmt.Errorf("request failed")
}

func (w *worker) requestDetailed(ctx context.Context, method, target string, payload any, headers map[string]string, followRedirects bool) (int, map[string]any, http.Header, error) {
	var body io.Reader
	var bodyData []byte
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, nil, err
		}
		bodyData = data
	}
	client := w.client
	if !followRedirects {
		clone := *w.client
		clone.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
		client = &clone
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if payload != nil {
			body = strings.NewReader(string(bodyData))
		}
		req, err := http.NewRequestWithContext(ctx, method, target, body)
		if err != nil {
			return 0, nil, nil, err
		}
		for key, value := range headers {
			if strings.TrimSpace(value) != "" {
				req.Header.Set(key, value)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			debugRequestError(method, target, attempt, err)
			lastErr = err
			if attempt < 2 {
				time.Sleep(time.Second)
				continue
			}
			return 0, nil, nil, err
		}
		defer resp.Body.Close()
		debugResponse(method, target, attempt, resp)
		w.debugCookies("request-detailed")
		payloadMap := map[string]any{}
		if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
			_ = DecodeJSON(resp.Body, &payloadMap)
		} else {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			if len(data) > 0 {
				payloadMap["body"] = string(data)
			}
		}
		if resp.Request != nil && resp.Request.URL != nil {
			payloadMap["_final_url"] = resp.Request.URL.String()
		}
		if location := strings.TrimSpace(resp.Header.Get("Location")); location != "" {
			payloadMap["_location"] = location
		}
		return resp.StatusCode, payloadMap, resp.Header.Clone(), nil
	}
	if lastErr != nil {
		return 0, nil, nil, lastErr
	}
	return 0, nil, nil, fmt.Errorf("request failed")
}

func (w *worker) requestForm(ctx context.Context, target string, form url.Values) (int, map[string]any, error) {
	body := []byte(form.Encode())
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "application/json",
		"User-Agent":   userAgent,
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(string(body)))
		if err != nil {
			return 0, nil, err
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := w.client.Do(req)
		if err != nil {
			debugRequestError(http.MethodPost, target, attempt, err)
			lastErr = err
			if attempt < 2 {
				time.Sleep(time.Second)
				continue
			}
			return 0, nil, err
		}
		defer resp.Body.Close()
		debugResponse(http.MethodPost, target, attempt, resp)
		w.debugCookies("form")
		payload := map[string]any{}
		_ = DecodeJSON(resp.Body, &payload)
		return resp.StatusCode, payload, nil
	}
	if lastErr != nil {
		return 0, nil, lastErr
	}
	return 0, nil, fmt.Errorf("form request failed")
}

func (w *worker) navigateHeaders(referer string) map[string]string {
	headers := map[string]string{
		"Accept":                      "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language":             "en-US,en;q=0.9",
		"Upgrade-Insecure-Requests":   "1",
		"User-Agent":                  userAgent,
		"sec-ch-ua":                   secCHUA,
		"sec-ch-ua-arch":              `"x86_64"`,
		"sec-ch-ua-bitness":           `"64"`,
		"sec-ch-ua-full-version-list": secCHUAFullVersionList,
		"sec-ch-ua-mobile":            "?0",
		"sec-ch-ua-model":             `""`,
		"sec-ch-ua-platform":          `"Windows"`,
		"sec-ch-ua-platform-version":  `"10.0.0"`,
		"sec-fetch-dest":              "document",
		"sec-fetch-mode":              "navigate",
		"sec-fetch-site":              "same-origin",
		"sec-fetch-user":              "?1",
	}
	if referer != "" {
		headers["Referer"] = referer
	}
	return headers
}

func (w *worker) jsonHeaders(referer string) map[string]string {
	headers := map[string]string{
		"Accept":                      "application/json",
		"Accept-Language":             "en-US,en;q=0.9",
		"Content-Type":                "application/json",
		"Origin":                      authBase,
		"priority":                    "u=1, i",
		"User-Agent":                  userAgent,
		"oai-device-id":               w.deviceID,
		"sec-ch-ua":                   secCHUA,
		"sec-ch-ua-arch":              `"x86_64"`,
		"sec-ch-ua-bitness":           `"64"`,
		"sec-ch-ua-full-version-list": secCHUAFullVersionList,
		"sec-ch-ua-mobile":            "?0",
		"sec-ch-ua-model":             `""`,
		"sec-ch-ua-platform":          `"Windows"`,
		"sec-ch-ua-platform-version":  `"10.0.0"`,
		"sec-fetch-dest":              "empty",
		"sec-fetch-mode":              "cors",
		"sec-fetch-site":              "same-origin",
	}
	for key, value := range traceHeaders() {
		headers[key] = value
	}
	if referer != "" {
		headers["Referer"] = referer
	}
	return headers
}

func (w *worker) step(_ string) {}

func authorizeParams(email, deviceID, state, nonce, codeChallenge string) url.Values {
	values := url.Values{}
	values.Set("issuer", authBase)
	values.Set("client_id", platformOAuthClientID)
	values.Set("audience", platformOAuthAudience)
	values.Set("redirect_uri", platformOAuthRedirectURI)
	values.Set("device_id", deviceID)
	values.Set("screen_hint", "login_or_signup")
	values.Set("max_age", "0")
	values.Set("login_hint", email)
	values.Set("scope", "openid profile email offline_access")
	values.Set("response_type", "code")
	values.Set("response_mode", "query")
	values.Set("state", state)
	values.Set("nonce", nonce)
	values.Set("code_challenge", codeChallenge)
	values.Set("code_challenge_method", "S256")
	values.Set("auth0Client", platformAuth0Client)
	return values
}

func extractOAuthCode(payload map[string]any, headers http.Header) string {
	if code := oauthCode(headers.Get("Location")); code != "" {
		return code
	}
	if code := oauthCode(Clean(payload["_final_url"])); code != "" {
		return code
	}
	return ""
}

func oauthCode(target string) string {
	if strings.TrimSpace(target) == "" {
		return ""
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("code"))
}

func resolveLocation(baseURL, location string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	next, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(next).String(), nil
}

func newSentinelTokenGenerator(deviceID, ua string) *sentinelTokenGenerator {
	return &sentinelTokenGenerator{deviceID: deviceID, userAgent: ua, sid: NewUUID()}
}

type sentinelTokenGenerator struct {
	deviceID  string
	userAgent string
	sid       string
}

func (g *sentinelTokenGenerator) config() []any {
	perfNow := 1000 + mathrand.Float64()*49000
	return []any{"1920x1080", time.Now().UTC().Format("Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated Universal Time)"), int64(4294705152), mathrand.Float64(), g.userAgent, sentinelSDK, nil, nil, "en-US", mathrand.Float64(), randomChoice([]string{"vendorSub-undefined", "plugins-undefined", "mimeTypes-undefined", "hardwareConcurrency-undefined"}), randomChoice([]string{"location", "implementation", "URL", "documentURI", "compatMode"}), randomChoice([]string{"Object", "Function", "Array", "Number", "parseFloat", "undefined"}), perfNow, g.sid, "", randomChoiceInt([]int{4, 8, 12, 16}), float64(time.Now().UnixMilli()) - perfNow}
}

func (g *sentinelTokenGenerator) generateRequirementsToken() string {
	data := g.config()
	data[3] = 1
	data[9] = math.Round(5 + mathrand.Float64()*45)
	return "gAAAAAC" + base64JSON(data)
}

func (g *sentinelTokenGenerator) generateToken(seed, difficulty string) string {
	start := time.Now()
	data := g.config()
	if difficulty == "" {
		difficulty = "0"
	}
	for i := 0; i < sentinelMaxAttempts; i++ {
		data[3] = i
		data[9] = math.Round(float64(time.Since(start).Milliseconds()))
		payload := base64JSON(data)
		hash := fnv1a32(seed + payload)
		prefixLen := minInt(len(difficulty), len(hash))
		if hash[:prefixLen] <= difficulty[:prefixLen] {
			return "gAAAAAB" + payload + "~S"
		}
	}
	return "gAAAAAB" + sentinelErrorPrefix + base64JSON("None")
}

func compactJSONBytes(value any) ([]byte, error) {
	var buf strings.Builder
	enc := json.NewEncoder(&builderWriter{b: &buf})
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(buf.String())), nil
}

type builderWriter struct{ b *strings.Builder }

func (w *builderWriter) Write(p []byte) (int, error) { return w.b.Write(p) }

func base64JSON(value any) string {
	data, err := compactJSONBytes(value)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

func fnv1a32(text string) string {
	hash := uint32(2166136261)
	for _, ch := range text {
		hash ^= uint32(ch)
		hash *= 16777619
	}
	hash ^= hash >> 16
	hash *= 2246822507
	hash ^= hash >> 13
	hash *= 3266489909
	hash ^= hash >> 16
	return fmt.Sprintf("%08x", hash)
}

func sentinelHeaders() map[string]string {
	return map[string]string{
		"Content-Type":       "text/plain;charset=UTF-8",
		"Referer":            sentinelBase + "/backend-api/sentinel/frame.html",
		"Origin":             sentinelBase,
		"User-Agent":         userAgent,
		"sec-ch-ua":          secCHUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
	}
}

func traceHeaders() map[string]string {
	traceID := NewHex(32)
	parentID := randomUint64()
	parentHex := fmt.Sprintf("%016x", parentID)
	parentText := fmt.Sprintf("%d", parentID)
	return map[string]string{
		"traceparent":                 "00-" + traceID + "-" + parentHex + "-01",
		"tracestate":                  "dd=s:1;o:rum",
		"x-datadog-origin":            "rum",
		"x-datadog-parent-id":         parentText,
		"x-datadog-sampling-priority": "1",
		"x-datadog-trace-id":          fmt.Sprintf("%d", randomUint64()),
	}
}

func responseDetail(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return ""
	}
	return ", detail=" + string(data)
}

func failedToCreateAccount(payload map[string]any) bool {
	return Clean(payload["message"]) == "Failed to create account. Please try again."
}

func randomPassword(length int) string {
	if length < 8 {
		length = 8
	}
	upper := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lower := "abcdefghijklmnopqrstuvwxyz"
	digits := "0123456789"
	special := "!@#$%"
	all := upper + lower + digits + special
	value := []byte{upper[mathrand.Intn(len(upper))], lower[mathrand.Intn(len(lower))], digits[mathrand.Intn(len(digits))], special[mathrand.Intn(len(special))]}
	for len(value) < length {
		value = append(value, all[mathrand.Intn(len(all))])
	}
	mathrand.Shuffle(len(value), func(i, j int) { value[i], value[j] = value[j], value[i] })
	return string(value)
}

func randomName() string {
	return firstNames[mathrand.Intn(len(firstNames))] + " " + lastNames[mathrand.Intn(len(lastNames))]
}

func randomBirthdate() string {
	year := 1996 + mathrand.Intn(11)
	month := 1 + mathrand.Intn(12)
	day := 1 + mathrand.Intn(28)
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func randomToken() string { return RandomTokenURL(24) }

func pkceChallenge() string {
	_, challenge := generatePKCE()
	return challenge
}

func generatePKCE() (string, string) {
	buf := make([]byte, 64)
	_, _ = rand.Read(buf)
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

func randomChoice(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[mathrand.Intn(len(values))]
}

func randomChoiceInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[mathrand.Intn(len(values))]
}

func randomUint64() uint64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return uint64(mathrand.Int63())
	}
	var value uint64
	for _, b := range buf {
		value = (value << 8) | uint64(b)
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mustReadLine(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		fatalf("读取输入失败: %v", err)
	}
	return strings.TrimSpace(line)
}

func mustReadOptional(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		fatalf("读取输入失败: %v", err)
	}
	return strings.TrimSpace(line)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func randomSeed() { mathrand.Seed(time.Now().UnixNano()) }

func loadConfig() appConfig {
	path := filepath.Join("config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return appConfig{PasswordMode: "random"}
	}
	var cfg appConfig
	if json.Unmarshal(data, &cfg) != nil {
		return appConfig{PasswordMode: "random"}
	}
	cfg.DefaultProxy = strings.TrimSpace(cfg.DefaultProxy)
	cfg.PasswordMode = strings.ToLower(strings.TrimSpace(cfg.PasswordMode))
	if cfg.PasswordMode != "manual" && cfg.PasswordMode != "random" {
		cfg.PasswordMode = "random"
	}
	cfg.DefaultPassword = strings.TrimSpace(cfg.DefaultPassword)
	cfg.DefaultManualPrompt = strings.TrimSpace(cfg.DefaultManualPrompt)
	return cfg
}

func maskPassword(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

func hasFlag(name string) bool {
	for _, arg := range os.Args[1:] {
		if arg == name {
			return true
		}
	}
	return false
}

func debugRequestError(method, target string, attempt int, err error) {
	if !debugMode {
		return
	}
	parsed, _ := url.Parse(target)
	fmt.Fprintf(os.Stderr, "[debug] request_error attempt=%d method=%s host=%s path=%s err=%v\n", attempt+1, method, parsed.Host, parsed.Path, err)
}

func debugResponse(method, target string, attempt int, resp *http.Response) {
	if !debugMode || resp == nil {
		return
	}
	parsed, _ := url.Parse(target)
	setCookie := resp.Header.Values("Set-Cookie")
	redactedCookies := make([]string, 0, len(setCookie))
	for _, item := range setCookie {
		redactedCookies = append(redactedCookies, redactCookie(item))
	}
	fmt.Fprintf(os.Stderr, "[debug] response attempt=%d method=%s host=%s path=%s status=%d final_url=%s content_type=%q location=%q set_cookie=%v\n", attempt+1, method, parsed.Host, parsed.Path, resp.StatusCode, resp.Request.URL.String(), resp.Header.Get("Content-Type"), resp.Header.Get("Location"), redactedCookies)
}

func (w *worker) debugCookies(stage string) {
	if !debugMode || w == nil || w.client == nil || w.client.Jar == nil {
		return
	}
	authURL, err := url.Parse(authBase)
	if err != nil {
		return
	}
	cookies := w.client.Jar.Cookies(authURL)
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name+"="+redactValue(cookie.Value))
	}
	fmt.Fprintf(os.Stderr, "[debug] cookies stage=%s auth=%v\n", stage, names)
}

func redactCookie(raw string) string {
	name := raw
	if idx := strings.Index(raw, "="); idx >= 0 {
		name = raw[:idx]
	}
	return name + "=<redacted>"
}

func redactValue(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "<redacted>"
	}
	return value[:4] + "..." + value[len(value)-4:]
}
