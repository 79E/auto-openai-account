package legacy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type RegisterInput struct {
	Mailbox        Mailbox
	Settings       Settings
	RegisterPass   string
	OTPFetcher     func(context.Context) (string, error)
	SkipTokenLogin bool
}

type RegisterResult struct {
	Email        string         `json:"email"`
	Password     string         `json:"password"`
	Name         string         `json:"name"`
	Birthdate    string         `json:"birthdate"`
	TokenPayload map[string]any `json:"token_json,omitempty"`
}

func RegisterOne(ctx context.Context, input RegisterInput) (*RegisterResult, error) {
	email := Clean(input.Mailbox.Email)
	logStep(email, "注册流程开始")
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	password := Clean(input.RegisterPass)
	if password == "" {
		password = passwordForSettings(input.Settings)
	}
	w, err := newWorkerWithOTP(input.Settings.Proxy, email, input.OTPFetcher)
	if err != nil {
		return nil, err
	}
	defer w.close()

	logStep(email, "步骤 1/8 platform authorize")
	if err := w.platformAuthorize(ctx, email); err != nil {
		return nil, err
	}
	logStep(email, "步骤 2/8 提交注册密码")
	if err := w.registerUser(ctx, email, password); err != nil {
		return nil, err
	}
	logStep(email, "步骤 3/8 请求发送邮箱验证码")
	if err := w.sendOTP(ctx); err != nil {
		return nil, err
	}
	logStep(email, "步骤 4/8 等待并读取邮箱验证码")
	code, err := input.OTPFetcher(ctx)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, fmt.Errorf("verification code is empty")
	}
	logStep(email, "步骤 5/8 已获取验证码 code=%s，提交校验", code)
	if err := w.validateOTP(ctx, code); err != nil {
		return nil, err
	}

	name := randomName()
	birthdate := randomBirthdate()
	logStep(email, "步骤 6/8 创建账号资料 name=%s birthdate=%s", name, birthdate)
	if err := w.createAccount(ctx, name, birthdate); err != nil {
		return nil, err
	}

	result := &RegisterResult{Email: email, Password: password, Name: name, Birthdate: birthdate}
	if input.SkipTokenLogin {
		logStep(email, "注册流程完成，跳过 token 登录")
		return result, nil
	}
	logStep(email, "步骤 7/8 登录并换取 token")
	tokens, err := w.loginAndExchangeTokens(ctx, email, password)
	if err != nil {
		return nil, err
	}
	tokens["email"] = email
	tokens["password"] = password
	tokens["created_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	result.TokenPayload = tokens
	logStep(email, "步骤 8/8 注册流程完成，token 已获取")
	return result, nil
}

func LoginOne(ctx context.Context, mailbox Mailbox, settings Settings, otpFetcher func(context.Context) (string, error)) (map[string]any, error) {
	email := Clean(mailbox.Email)
	logStep(email, "登录换 token 流程开始")
	password := firstNonEmpty(Clean(mailbox.RegisterPassword), Clean(mailbox.Password))
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}
	w, err := newWorkerWithOTP(settings.Proxy, email, otpFetcher)
	if err != nil {
		return nil, err
	}
	defer w.close()
	tokens, err := w.loginAndExchangeTokens(ctx, email, password)
	if err != nil {
		return nil, err
	}
	tokens["email"] = email
	tokens["password"] = password
	tokens["created_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	logStep(email, "登录换 token 流程完成")
	return tokens, nil
}

func passwordForSettings(settings Settings) string {
	if settings.PasswordMode == "fixed" && Clean(settings.FixedPassword) != "" {
		return Clean(settings.FixedPassword)
	}
	return randomPassword(16)
}

func compactTokenJSON(tokens map[string]any) string {
	if len(tokens) == 0 {
		return ""
	}
	data, err := json.Marshal(tokens)
	if err != nil {
		return ""
	}
	return string(data)
}
