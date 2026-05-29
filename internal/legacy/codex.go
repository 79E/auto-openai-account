package legacy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	codexOAuthClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexOAuthRedirectURI = "http://localhost:1455/auth/callback"
	codexOAuthScopes      = "openid profile email offline_access api.connectors.read api.connectors.invoke"
)

type CodexSMSActivation struct {
	ID               string
	PhoneNumber      string
	CountryPhoneCode int
}

type CodexSMSProvider interface {
	GetNumber(context.Context) (*CodexSMSActivation, error)
	PollCode(context.Context, string) (string, error)
	Complete(context.Context, string) error
	Cancel(context.Context, string) error
}

type CodexLoginInput struct {
	Email                    string
	Password                 string
	Proxy                    string
	SMSProvider              CodexSMSProvider
	OTPFetcher               func(context.Context) (string, error)
	MaxPhoneAttempts         int
	PasswordVerifyRetries    int
	PasswordVerifyRetryDelay time.Duration
	Progress                 func(step string, index int, total int, message string)
}

type CodexLoginResult struct {
	TokenPayload map[string]any
	PhoneNumber  string
}

func CodexLogin(ctx context.Context, input CodexLoginInput) (*CodexLoginResult, error) {
	email := Clean(input.Email)
	password := Clean(input.Password)
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	w, err := newWorkerWithOTP(input.Proxy, email, input.OTPFetcher)
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			w.close()
		}
	}()

	progress := input.Progress
	if progress == nil {
		progress = func(string, int, int, string) {}
	}
	attempts := input.MaxPhoneAttempts
	if attempts < 1 {
		attempts = 3
	}
	passwordVerifyRetries := input.PasswordVerifyRetries
	if passwordVerifyRetries < 1 {
		passwordVerifyRetries = 1
	}
	passwordVerifyRetryDelay := input.PasswordVerifyRetryDelay
	if passwordVerifyRetryDelay <= 0 {
		passwordVerifyRetryDelay = 10 * time.Second
	}
	canFallbackToEmailOTP := input.OTPFetcher != nil
	if input.OTPFetcher != nil && passwordVerifyRetries > 1 {
		logStep(email, "Codex 授权登录: 新注册账号先执行普通登录预热，建立上游认证会话")
		progress("codex_authorize", 1, 8, "正在预热登录会话")
		if _, err := w.loginAndExchangeTokens(ctx, email, password); err != nil {
			logStep(email, "Codex 授权登录: 普通登录预热失败，继续尝试 Codex 授权 err=%v", err)
		} else {
			logStep(email, "Codex 授权登录: 普通登录预热完成，重建干净 Codex OAuth 会话")
			w.close()
			cleanup = false
			w, err = newWorkerWithOTP(input.Proxy, email, input.OTPFetcher)
			if err != nil {
				return nil, err
			}
		}
	}

	codeVerifier, codeChallenge := generatePKCE()
	state := randomToken()
	logStep(email, "Codex 授权登录: 创建 OAuth 会话 client_id=%s redirect_uri=%s scopes=%q code_challenge_len=%d state_hash=%s", codexOAuthClientID, codexOAuthRedirectURI, codexOAuthScopes, len(codeChallenge), shortSecretHash(state))
	progress("codex_authorize", 1, 8, "正在创建 Codex OAuth 授权会话")
	status, payload, err := w.request(ctx, http.MethodGet, codexAuthorizeURL(state, codeChallenge), nil, w.navigateHeaders(authBase+"/"), true)
	if err != nil {
		logStep(email, "Codex 授权登录: OAuth 会话请求失败 err=%v", err)
		return nil, fmt.Errorf("codex_authorize_request_failed: %w", err)
	}
	logStep(email, "Codex 授权登录: OAuth 会话返回 status=%d%s", status, responseDetail(payload))
	if status >= 400 {
		return nil, fmt.Errorf("codex_authorize_http_%d%s", status, responseDetail(payload))
	}

	logStep(email, "Codex 授权登录: 提交邮箱 device_id=%s", w.deviceID)
	progress("submit_email", 2, 8, "正在提交邮箱并确认登录方式")
	submitRetries := 3
	for submitAttempt := 1; submitAttempt <= submitRetries; submitAttempt++ {
		status, payload, err = w.submitLoginEmail(ctx, email)
		if err != nil {
			logStep(email, "Codex 授权登录: 提交邮箱请求失败 err=%v", err)
			return nil, fmt.Errorf("submit_email_failed: %w", err)
		}
		logStep(email, "Codex 授权登录: 提交邮箱返回 status=%d%s", status, responseDetail(payload))
		if status == http.StatusTooManyRequests && submitAttempt < submitRetries {
			backoff := time.Duration(submitAttempt*15) * time.Second
			logStep(email, "Codex 授权登录: 提交邮箱触发限流 429，等待 %s 后重试 (%d/%d)", backoff, submitAttempt+1, submitRetries)
			progress("submit_email", 2, 8, fmt.Sprintf("提交邮箱触发限流，等待 %s 后重试 (%d/%d)", backoff, submitAttempt+1, submitRetries))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
		break
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("submit_email_http_%d%s", status, responseDetail(payload))
	}

	continueURL, pageType := pageState(payload)
	progress("submit_email", 2, 8, fmt.Sprintf("邮箱提交成功，下一步 %s", firstNonEmpty(pageType, continueURL)))

	var phoneNumber string
	if isPasswordStep(continueURL, pageType) {
		passwordVerified := false
		for attempt := 1; attempt <= passwordVerifyRetries; attempt++ {
			logStep(email, "Codex 授权登录: 构建并提交密码校验 attempt=%d/%d password_len=%d password_hash=%s", attempt, passwordVerifyRetries, len(Clean(password)), shortSecretHash(password))
			progress("password_login", 3, 8, "正在提交 OpenAI 密码")
			status, payload, err = w.codexVerifyPassword(ctx, password)
			if err != nil {
				logStep(email, "Codex 授权登录: 密码校验请求失败 attempt=%d/%d err=%v", attempt, passwordVerifyRetries, err)
				return nil, fmt.Errorf("password_verify_failed: %w", err)
			}
			logStep(email, "Codex 授权登录: 密码校验返回 attempt=%d/%d status=%d%s", attempt, passwordVerifyRetries, status, responseDetail(payload))
			if status == http.StatusOK {
				passwordVerified = true
				break
			}
			if status == http.StatusUnauthorized {
				if canFallbackToEmailOTP {
					logStep(email, "Codex 授权登录: 密码校验失败，准备回退邮箱验证码登录 status=%d attempt=%d/%d", status, attempt, passwordVerifyRetries)
					break
				}
				return nil, fmt.Errorf("password_verify_http_%d%s", status, responseDetail(payload))
			}
			if attempt == passwordVerifyRetries {
				return nil, fmt.Errorf("password_verify_http_%d%s", status, responseDetail(payload))
			}
			progress("password_login", 3, 8, fmt.Sprintf("密码校验暂未通过，等待 %s 后重试（%d/%d）", passwordVerifyRetryDelay, attempt+1, passwordVerifyRetries))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(passwordVerifyRetryDelay):
			}
		}
		if passwordVerified {
			continueURL, pageType = pageState(payload)
			logStep(email, "Codex 授权登录: 密码校验通过 continue_url=%s page_type=%s", continueURL, pageType)
			progress("password_login", 3, 8, fmt.Sprintf("密码校验通过，下一步 %s", firstNonEmpty(pageType, continueURL)))
			if pageType == "email_otp_verification" || strings.Contains(continueURL, "email-verification") || strings.Contains(continueURL, "email-otp") {
				if !canFallbackToEmailOTP {
					return nil, fmt.Errorf("codex login requires email otp verification but mailbox is missing required email auth data")
				}
				logStep(email, "Codex 授权登录: 密码校验通过，上游要求邮箱验证码验证 (login_challenge)")
				progress("password_login", 3, 8, "密码校验通过，正在等待邮箱验证码")
				code, waitErr := w.waitRegisterCode(ctx)
				if waitErr != nil {
					return nil, waitErr
				}
				if Clean(code) == "" {
					return nil, fmt.Errorf("codex login waiting for email verification code timed out after password verify")
				}
				logStep(email, "Codex 授权登录: 已获取邮箱验证码 code=%s，提交校验", Clean(code))
				progress("password_login", 3, 8, "已读取邮箱验证码，正在提交校验")
				otpPayload, otpErr := w.validateOTPCode(ctx, Clean(code))
				if otpErr != nil {
					return nil, fmt.Errorf("codex email otp verify failed after password: %w", otpErr)
				}
				continueURL, pageType = pageState(otpPayload)
				logStep(email, "Codex 授权登录: 邮箱验证码校验通过 continue_url=%s page_type=%s", continueURL, pageType)
				progress("password_login", 3, 8, fmt.Sprintf("邮箱验证码校验通过，下一步 %s", firstNonEmpty(pageType, continueURL)))
			}
		} else {
			logStep(email, "Codex 授权登录: 密码校验失败，回退邮箱验证码登录")
			progress("password_login", 3, 8, "密码校验失败，正在回退邮箱验证码登录")
			if err := w.sendLoginOTP(ctx, continueURL); err != nil {
				return nil, fmt.Errorf("codex password login failed and email otp fallback unavailable: %w", err)
			}
			code, waitErr := w.waitRegisterCode(ctx)
			if waitErr != nil {
				return nil, waitErr
			}
			if Clean(code) == "" {
				return nil, fmt.Errorf("codex email otp fallback timed out after password verify failure")
			}
			logStep(email, "Codex 授权登录: 密码失败后已获取邮箱验证码 code=%s，提交校验", Clean(code))
			progress("password_login", 3, 8, "已读取邮箱验证码，正在提交校验")
			otpPayload, otpErr := w.validateOTPCode(ctx, Clean(code))
			if otpErr != nil {
				return nil, fmt.Errorf("codex email otp fallback verify failed: %w", otpErr)
			}
			continueURL, pageType = pageState(otpPayload)
			logStep(email, "Codex 授权登录: 邮箱验证码回退通过 continue_url=%s page_type=%s", continueURL, pageType)
			progress("password_login", 3, 8, fmt.Sprintf("邮箱验证码登录通过，下一步 %s", firstNonEmpty(pageType, continueURL)))
		}
	}

	if isAddPhoneStep(continueURL, pageType) {
		nextURL, nextType, boundPhone, err := w.codexBindPhone(ctx, input.SMSProvider, attempts, progress)
		if err != nil {
			return nil, err
		}
		continueURL, pageType = nextURL, nextType
		phoneNumber = boundPhone
	}

	progress("exchange_token", 8, 8, "正在选择 workspace 并换取 Codex token")
	tokenPayload, err := w.exchangeCodexTokensFromContinueURL(ctx, continueURL, codeVerifier)
	if err != nil {
		return nil, err
	}
	tokenPayload["email"] = email
	tokenPayload["password"] = password
	tokenPayload["created_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	if phoneNumber != "" {
		tokenPayload["phone_number"] = phoneNumber
	}
	progress("codex_complete", 8, 8, "Codex 授权登录流程完成")
	return &CodexLoginResult{TokenPayload: tokenPayload, PhoneNumber: phoneNumber}, nil
}

func codexAuthorizeURL(state, codeChallenge string) string {
	params := url.Values{
		"response_type":              {"code"},
		"client_id":                  {codexOAuthClientID},
		"redirect_uri":               {codexOAuthRedirectURI},
		"scope":                      {codexOAuthScopes},
		"state":                      {state},
		"code_challenge":             {codeChallenge},
		"code_challenge_method":      {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
	}
	return authBase + "/oauth/authorize?" + params.Encode()
}

func pageState(payload map[string]any) (string, string) {
	page := StringMap(payload["page"])
	return Clean(payload["continue_url"]), Clean(page["type"])
}

func isPasswordStep(continueURL, pageType string) bool {
	return pageType == "login_password" || strings.Contains(continueURL, "/log-in/password")
}

func isAddPhoneStep(continueURL, pageType string) bool {
	return pageType == "add_phone" || strings.Contains(continueURL, "/add-phone")
}

func (w *worker) codexVerifyPassword(ctx context.Context, password string) (int, map[string]any, error) {
	headers := w.jsonHeaders(authBase + "/log-in/password")
	token, err := w.buildSentinelToken(ctx, "password_verify")
	if err != nil {
		return 0, nil, err
	}
	headers["openai-sentinel-token"] = token
	logStep(w.mail, "Codex 授权登录: password_verify sentinel token 已生成 token_len=%d", len(token))
	return w.request(ctx, http.MethodPost, authBase+"/api/accounts/password/verify", map[string]any{
		"password": password,
	}, headers, false)
}

func (w *worker) codexBindPhone(ctx context.Context, provider CodexSMSProvider, maxAttempts int, progress func(string, int, int, string)) (string, string, string, error) {
	if provider == nil {
		return "", "", "", fmt.Errorf("sms provider is required for codex phone verification")
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		progress("add_phone", 4, 8, fmt.Sprintf("正在获取手机号，第 %d/%d 次", attempt, maxAttempts))
		logStep(w.mail, "Codex 授权登录: 正在从短信平台获取手机号 attempt=%d/%d", attempt, maxAttempts)
		activation, err := provider.GetNumber(ctx)
		if err != nil {
			lastErr = err
			logStep(w.mail, "Codex 授权登录: 获取手机号失败 attempt=%d/%d err=%v", attempt, maxAttempts, err)
			progress("add_phone", 4, 8, phoneRetryMessage(attempt, maxAttempts, fmt.Sprintf("获取手机号失败：%v", err)))
			continue
		}
		if activation == nil || Clean(activation.ID) == "" || Clean(activation.PhoneNumber) == "" {
			lastErr = fmt.Errorf("sms provider returned empty activation")
			logStep(w.mail, "Codex 授权登录: 短信平台返回空手机号或空激活ID attempt=%d/%d", attempt, maxAttempts)
			progress("add_phone", 4, 8, phoneRetryMessage(attempt, maxAttempts, "短信平台返回空手机号或空激活 ID"))
			continue
		}
		phoneNumber := normalizeCodexPhoneNumber(activation.PhoneNumber, activation.CountryPhoneCode)
		if phoneNumber == "" {
			_ = provider.Cancel(ctx, activation.ID)
			lastErr = fmt.Errorf("sms provider returned invalid phone number")
			logStep(w.mail, "Codex 授权登录: 短信平台返回的手机号格式无效 attempt=%d/%d raw=%s", attempt, maxAttempts, activation.PhoneNumber)
			progress("add_phone", 4, 8, phoneRetryMessage(attempt, maxAttempts, fmt.Sprintf("短信平台返回的手机号格式无效：%s", activation.PhoneNumber)))
			continue
		}

		progress("add_phone", 4, 8, fmt.Sprintf("已获取手机号 %s，正在提交给 OpenAI", phoneNumber))
		status, payload, err := w.codexSubmitPhone(ctx, phoneNumber)
		if err != nil {
			_ = provider.Cancel(ctx, activation.ID)
			lastErr = fmt.Errorf("submit_phone_failed: %w", err)
			progress("add_phone", 4, 8, phoneRetryMessage(attempt, maxAttempts, fmt.Sprintf("提交手机号请求失败：%v", err)))
			continue
		}
		if status != http.StatusOK {
			_ = provider.Cancel(ctx, activation.ID)
			lastErr = fmt.Errorf("submit_phone_http_%d%s", status, responseDetail(payload))
			progress("add_phone", 4, 8, phoneRetryMessage(attempt, maxAttempts, "手机号被 OpenAI 拒绝："+phoneSubmitFailureReason(status, payload)))
			continue
		}

		progress("phone_verification", 5, 8, "手机号已提交，正在等待短信验证码")
		code, err := provider.PollCode(ctx, activation.ID)
		if err != nil {
			_ = provider.Cancel(ctx, activation.ID)
			lastErr = err
			progress("phone_verification", 5, 8, phoneRetryMessage(attempt, maxAttempts, fmt.Sprintf("短信验证码获取失败：%v", err)))
			continue
		}
		if Clean(code) == "" {
			_ = provider.Cancel(ctx, activation.ID)
			lastErr = fmt.Errorf("sms code is empty")
			progress("phone_verification", 5, 8, phoneRetryMessage(attempt, maxAttempts, "短信平台返回空验证码"))
			continue
		}

		progress("phone_verification", 6, 8, "已获取短信验证码，正在提交校验")
		status, payload, err = w.codexSubmitPhoneOTP(ctx, Clean(code))
		if err != nil {
			_ = provider.Cancel(ctx, activation.ID)
			lastErr = err
			progress("phone_verification", 6, 8, phoneRetryMessage(attempt, maxAttempts, fmt.Sprintf("提交短信验证码请求失败：%v", err)))
			continue
		}
		if status != http.StatusOK {
			_ = provider.Cancel(ctx, activation.ID)
			lastErr = fmt.Errorf("phone_otp_http_%d%s", status, responseDetail(payload))
			progress("phone_verification", 6, 8, phoneRetryMessage(attempt, maxAttempts, fmt.Sprintf("短信验证码被 OpenAI 拒绝：HTTP %d%s", status, responseDetail(payload))))
			continue
		}
		_ = provider.Complete(ctx, activation.ID)
		nextURL, pageType := pageState(payload)
		progress("phone_verification", 7, 8, "手机号验证成功")
		return nextURL, pageType, phoneNumber, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("phone verification failed")
	}
	return "", "", "", fmt.Errorf("failed to verify phone after %d attempts: %w", maxAttempts, lastErr)
}

func (w *worker) codexSubmitPhone(ctx context.Context, phoneNumber string) (int, map[string]any, error) {
	return w.request(ctx, http.MethodPost, authBase+"/api/accounts/add-phone/send", map[string]any{
		"phone_number": phoneNumber,
	}, w.jsonHeaders(authBase+"/add-phone"), false)
}

func phoneRetryMessage(attempt, maxAttempts int, reason string) string {
	if attempt < maxAttempts {
		return fmt.Sprintf("%s。已取消当前手机号，准备更换手机号（下一次 %d/%d）", reason, attempt+1, maxAttempts)
	}
	return fmt.Sprintf("%s。已取消当前手机号，已达到最大手机号尝试次数（%d/%d）", reason, attempt, maxAttempts)
}

func phoneSubmitFailureReason(status int, payload map[string]any) string {
	errPayload := StringMap(payload["error"])
	code := Clean(errPayload["code"])
	message := Clean(errPayload["message"])
	switch {
	case code != "" && message != "":
		return fmt.Sprintf("HTTP %d，%s：%s", status, code, message)
	case code != "":
		return fmt.Sprintf("HTTP %d，%s", status, code)
	case message != "":
		return fmt.Sprintf("HTTP %d，%s", status, message)
	default:
		return fmt.Sprintf("HTTP %d%s", status, responseDetail(payload))
	}
}

func (w *worker) codexSubmitPhoneOTP(ctx context.Context, code string) (int, map[string]any, error) {
	return w.request(ctx, http.MethodPost, authBase+"/api/accounts/phone-otp/validate", map[string]any{
		"code": code,
	}, w.jsonHeaders(authBase+"/phone-verification"), false)
}

func (w *worker) exchangeCodexTokensFromContinueURL(ctx context.Context, continueURL, codeVerifier string) (map[string]any, error) {
	if continueURL == "" {
		continueURL = authBase + "/sign-in-with-chatgpt/codex/consent"
	}
	logStep(w.mail, "Codex 授权登录: 跟随授权同意页 continue_url=%s code_verifier_len=%d", continueURL, len(codeVerifier))
	code, err := w.followConsentForCode(ctx, continueURL)
	if err != nil {
		logStep(w.mail, "Codex 授权登录: 授权同意页跳转失败 err=%v", err)
		return nil, err
	}
	if code == "" {
		logStep(w.mail, "Codex 授权登录: consent/workspace 全链路执行后仍未拿到 OAuth code")
		return nil, fmt.Errorf("codex token exchange callback code not found")
	}
	logStep(w.mail, "Codex 授权登录: 已获取 OAuth code code_hash=%s，准备换取 token", shortSecretHash(code))
	status, tokenPayload, err := w.requestForm(ctx, authBase+"/oauth/token", url.Values{
		"grant_type":    []string{"authorization_code"},
		"code":          []string{code},
		"redirect_uri":  []string{codexOAuthRedirectURI},
		"client_id":     []string{codexOAuthClientID},
		"code_verifier": []string{codeVerifier},
	})
	if err != nil {
		logStep(w.mail, "Codex 授权登录: token 请求失败 err=%v", err)
		return nil, err
	}
	logStep(w.mail, "Codex 授权登录: token 接口返回 status=%d access=%s refresh=%s id=%s", status, AnonymizeToken(tokenPayload["access_token"]), AnonymizeToken(tokenPayload["refresh_token"]), AnonymizeToken(tokenPayload["id_token"]))
	if status != http.StatusOK {
		logStep(w.mail, "Codex 授权登录: token 非 200 原始响应%s", responseDetail(tokenPayload))
		return nil, fmt.Errorf("codex_oauth_token_http_%d", status)
	}
	accessToken := Clean(tokenPayload["access_token"])
	refreshToken := Clean(tokenPayload["refresh_token"])
	idToken := Clean(tokenPayload["id_token"])
	if accessToken == "" || refreshToken == "" || idToken == "" {
		logStep(w.mail, "Codex 授权登录: token 返回缺字段 payload=%s", detailSnippet(tokenPayload))
		return nil, fmt.Errorf("codex token exchange response missing access_token, refresh_token, or id_token")
	}
	return tokenPayload, nil
}

func normalizeCodexPhoneNumber(phone string, countryPhoneCode int) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	value := digits.String()
	if value == "" {
		return ""
	}
	if countryPhoneCode > 0 {
		prefix := strconv.Itoa(countryPhoneCode)
		if !strings.HasPrefix(value, prefix) {
			value = prefix + value
		}
	}
	return "+" + value
}
