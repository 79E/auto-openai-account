package legacy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var otpCodeRE = regexp.MustCompile(`\b\d{6}\b`)
var otpCodePromptRE = regexp.MustCompile(`(?is)enter\s+this\s+temporary\s+verification\s+code\s+to\s+continue\s*:.*?\b(\d{6})\b`)
var htmlCommentRE = regexp.MustCompile(`(?is)<!--.*?-->`)
var htmlScriptRE = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
var htmlStyleRE = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
var htmlTagRE = regexp.MustCompile(`(?is)<[^>]+>`)
var whitespaceRE = regexp.MustCompile(`\s+`)

const openAIOTPFrom = "noreply@tm.openai.com"

var outlookTokenEndpoints = []string{
	"https://login.microsoftonline.com/common/oauth2/v2.0/token",
	"https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
	"https://login.live.com/oauth20_token.srf",
}

var outlookTokenScopes = []string{
	"https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/POP.AccessAsUser.All offline_access",
	"https://outlook.office.com/IMAP.AccessAsUser.All offline_access",
}

type outlookTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type OTPProvider struct {
	Settings Settings
	Mailbox  Mailbox
	Since    time.Time
}

func (p OTPProvider) Fetch(ctx context.Context) (string, error) {
	settings := normalizeSettings(p.Settings)
	timeout := time.Duration(settings.OTPTimeoutSeconds) * time.Second
	poll := time.Duration(settings.OTPPollIntervalSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	var lastErr error
	attempt := 1
	logStep(p.Mailbox.Email, "开始轮询邮箱验证码 timeout=%s poll=%s imap=%s:%d auth=%s", timeout, poll, settings.IMAPHost, settings.IMAPPort, settings.IMAPAuthMode)
	for time.Now().Before(deadline) {
		logStep(p.Mailbox.Email, "邮箱验证码轮询第 %d 次", attempt)
		code, err := fetchIMAPOTP(ctx, settings, p.Mailbox, p.Since)
		if err == nil && code != "" {
			logStep(p.Mailbox.Email, "邮箱验证码获取成功 code=%s", code)
			return code, nil
		}
		if err != nil {
			lastErr = err
			logStep(p.Mailbox.Email, "邮箱验证码轮询失败 attempt=%d err=%v", attempt, err)
		} else {
			logStep(p.Mailbox.Email, "本次未找到验证码，等待 %s 后重试", poll)
		}
		attempt++
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(poll):
		}
	}
	if lastErr != nil {
		logStep(p.Mailbox.Email, "邮箱验证码超时，最后错误=%v", lastErr)
		return "", fmt.Errorf("otp timeout: %w", lastErr)
	}
	logStep(p.Mailbox.Email, "邮箱验证码超时，未找到验证码邮件")
	return "", fmt.Errorf("otp timeout")
}

func fetchIMAPOTP(ctx context.Context, settings Settings, mailbox Mailbox, since time.Time) (string, error) {
	addr := net.JoinHostPort(settings.IMAPHost, strconv.Itoa(settings.IMAPPort))
	logStep(mailbox.Email, "连接 IMAP %s", addr)
	conn, err := dialIMAPTLS(ctx, settings, addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(45 * time.Second))
	reader := bufio.NewReader(conn)
	if _, err := reader.ReadString('\n'); err != nil {
		return "", err
	}
	client := &rawIMAP{conn: conn, reader: reader, seq: 1}
	logStep(mailbox.Email, "IMAP 登录认证")
	if err := client.authenticate(ctx, settings, mailbox); err != nil {
		return "", err
	}
	logStep(mailbox.Email, "IMAP 选择 INBOX")
	if _, err := client.command("SELECT INBOX"); err != nil {
		return "", err
	}
	logStep(mailbox.Email, "IMAP 搜索全部邮件")
	searchResp, err := client.command("SEARCH ALL")
	if err != nil {
		return "", err
	}
	ids := parseIMAPIDs(searchResp)
	if len(ids) == 0 {
		logStep(mailbox.Email, "INBOX 没有邮件")
		return "", nil
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ids)))
	if len(ids) > 20 {
		ids = ids[:20]
	}
	logStep(mailbox.Email, "准备检查最近 %d 封邮件 ids=%v", len(ids), ids)
	for _, id := range ids {
		logStep(mailbox.Email, "读取邮件 id=%d", id)
		resp, err := client.command(fmt.Sprintf("FETCH %d (INTERNALDATE BODY.PEEK[])", id))
		if err != nil {
			logStep(mailbox.Email, "读取邮件失败 id=%d err=%v", id, err)
			continue
		}
		mailTime := mailReceivedTime(resp)
		if !since.IsZero() && !mailTime.IsZero() && mailTime.Before(since.Add(-5*time.Second)) {
			logStep(mailbox.Email, "邮件 id=%d 跳过：时间 %s 早于本次验证码请求 %s", id, mailTime.Format(time.RFC3339), since.Format(time.RFC3339))
			continue
		}
		if code := extractOTPFromMail(mailbox.Email, id, resp); code != "" {
			return code, nil
		}
	}
	return "", nil
}

func dialIMAPTLS(ctx context.Context, settings Settings, addr string) (*tls.Conn, error) {
	dialer := &net.Dialer{Timeout: 20 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: settings.IMAPHost, MinVersion: tls.VersionTLS12})
	if err == nil || strings.TrimSpace(settings.Proxy) == "" {
		return conn, err
	}
	rawConn, proxyErr := dialerForProxy(settings.Proxy)(ctx, "tcp", addr)
	if proxyErr != nil {
		return nil, fmt.Errorf("direct imap failed: %w; proxy imap failed: %v", err, proxyErr)
	}
	proxyConn := tls.Client(rawConn, &tls.Config{ServerName: settings.IMAPHost, MinVersion: tls.VersionTLS12})
	if proxyErr := proxyConn.HandshakeContext(ctx); proxyErr != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("direct imap failed: %w; proxy imap failed: %v", err, proxyErr)
	}
	return proxyConn, nil
}

type rawIMAP struct {
	conn   net.Conn
	reader *bufio.Reader
	seq    int
}

func (c *rawIMAP) authenticate(ctx context.Context, settings Settings, mailbox Mailbox) error {
	mode := settings.IMAPAuthMode
	if mode == "xoauth2" || mode == "auto" {
		accessToken, refreshErr := outlookIMAPAccessToken(ctx, mailbox, settings.Proxy)
		if accessToken != "" {
			if _, err := c.authenticateXOAUTH2(mailbox, accessToken); err == nil {
				return nil
			} else if mode == "xoauth2" {
				return err
			}
		} else if refreshErr != nil && mode == "xoauth2" {
			return refreshErr
		}
		if token := Clean(mailbox.AccessToken); token != "" && token != accessToken {
			if _, err := c.authenticateXOAUTH2(mailbox, token); err == nil {
				return nil
			} else if mode == "xoauth2" {
				return err
			}
		}
	}
	if mode == "password" || mode == "auto" {
		return c.loginPassword(mailbox)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("imap xoauth2 token is empty")
	}
}

func (c *rawIMAP) authenticateXOAUTH2(mailbox Mailbox, accessToken string) (string, error) {
	payload := base64.StdEncoding.EncodeToString([]byte("user=" + Clean(mailbox.Email) + "\x01auth=Bearer " + Clean(accessToken) + "\x01\x01"))
	return c.command("AUTHENTICATE XOAUTH2 " + payload)
}

func (c *rawIMAP) loginPassword(mailbox Mailbox) error {
	email := strings.ReplaceAll(Clean(mailbox.Email), `"`, `\"`)
	password := strings.ReplaceAll(Clean(mailbox.Password), `"`, `\"`)
	_, err := c.command(fmt.Sprintf("LOGIN \"%s\" \"%s\"", email, password))
	return err
}

func outlookIMAPAccessToken(ctx context.Context, mailbox Mailbox, proxy string) (string, error) {
	clientID := Clean(mailbox.ClientID)
	refreshToken := Clean(mailbox.AccessToken)
	if clientID == "" || refreshToken == "" {
		return "", nil
	}

	var lastErr error
	for _, endpoint := range outlookTokenEndpoints {
		scopes := outlookTokenScopes
		if strings.Contains(endpoint, "live.com") {
			scopes = []string{""}
		}
		for _, scope := range scopes {
			token, err := requestOutlookAccessToken(ctx, endpoint, clientID, refreshToken, scope, "")
			if err != nil && strings.TrimSpace(proxy) != "" {
				token, err = requestOutlookAccessToken(ctx, endpoint, clientID, refreshToken, scope, proxy)
			}
			if err == nil && token != "" {
				logStep(mailbox.Email, "Outlook access token 刷新成功 endpoint=%s", endpoint)
				return token, nil
			}
			if err != nil {
				lastErr = err
			}
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("outlook access token refresh failed")
}

func requestOutlookAccessToken(ctx context.Context, endpoint, clientID, refreshToken, scope, proxy string) (string, error) {
	body := url.Values{
		"client_id":     []string{clientID},
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{refreshToken},
	}
	if scope != "" {
		body.Set("scope", scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 30 * time.Second, Transport: transportForProxy(proxy)}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var payload outlookTokenResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("%s: token response decode failed: %w", endpoint, err)
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	if payload.Error != "" {
		detail := payload.Error
		if payload.ErrorDescription != "" {
			detail += ": " + payload.ErrorDescription
		}
		return "", fmt.Errorf("%s: %s", endpoint, detail)
	}
	return "", fmt.Errorf("%s: token http %d", endpoint, resp.StatusCode)
}

func (c *rawIMAP) command(command string) (string, error) {
	_ = c.conn.SetDeadline(time.Now().Add(45 * time.Second))
	tag := fmt.Sprintf("A%04d", c.seq)
	c.seq++
	if _, err := fmt.Fprintf(c.conn, "%s %s\r\n", tag, command); err != nil {
		return "", err
	}
	var builder strings.Builder
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return builder.String(), err
		}
		builder.WriteString(line)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, tag+" ") {
			if strings.Contains(trimmed, " OK") {
				return builder.String(), nil
			}
			return builder.String(), fmt.Errorf("imap command failed: %s", trimmed)
		}
	}
}

func parseIMAPIDs(resp string) []int {
	ids := []int{}
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "* SEARCH") {
			continue
		}
		for _, part := range strings.Fields(strings.TrimPrefix(line, "* SEARCH")) {
			id, err := strconv.Atoi(part)
			if err == nil && id > 0 {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func extractOTPFromMail(email string, id int, raw string) string {
	lower := strings.ToLower(raw)
	if !strings.Contains(lower, openAIOTPFrom) {
		logStep(email, "邮件 id=%d 跳过：发件人不是 %s", id, openAIOTPFrom)
		return ""
	}
	body := imapMailBody(raw)
	visibleText := visibleMailText(body)
	code := ""
	if matches := otpCodePromptRE.FindStringSubmatch(visibleText); len(matches) > 1 {
		code = matches[1]
	}
	if code == "" {
		code = otpCodeRE.FindString(visibleText)
	}
	if code == "" {
		logStep(email, "邮件 id=%d 来自 %s，但可见正文未匹配到 6 位数字", id, openAIOTPFrom)
		return ""
	}
	logStep(email, "邮件 id=%d 来自 %s，可见正文匹配到 6 位验证码 code=%s context=%q", id, openAIOTPFrom, code, otpContext(visibleText, code))
	return code
}

func imapMailBody(raw string) string {
	body := raw
	if idx := strings.Index(body, "\r\n\r\n"); idx >= 0 {
		body = body[idx+4:]
	} else if idx := strings.Index(body, "\n\n"); idx >= 0 {
		body = body[idx+2:]
	}
	if decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body))); err == nil && len(decoded) > 0 {
		body = string(decoded)
	}
	return body
}

func mailReceivedTime(raw string) time.Time {
	if t := internalDateTime(raw); !t.IsZero() {
		return t
	}
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		return time.Time{}
	}
	date := strings.TrimSpace(msg.Header.Get("Date"))
	if date == "" {
		return time.Time{}
	}
	if t, err := mail.ParseDate(date); err == nil {
		return t
	}
	return time.Time{}
}

func internalDateTime(raw string) time.Time {
	idx := strings.Index(strings.ToUpper(raw), "INTERNALDATE ")
	if idx < 0 {
		return time.Time{}
	}
	value := raw[idx+len("INTERNALDATE "):]
	start := strings.Index(value, `"`)
	if start < 0 {
		return time.Time{}
	}
	value = value[start+1:]
	end := strings.Index(value, `"`)
	if end < 0 {
		return time.Time{}
	}
	parsed, err := time.Parse("02-Jan-2006 15:04:05 -0700", value[:end])
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func visibleMailText(body string) string {
	body = strings.ReplaceAll(body, "\r", "\n")
	body = htmlCommentRE.ReplaceAllString(body, " ")
	body = htmlScriptRE.ReplaceAllString(body, " ")
	body = htmlStyleRE.ReplaceAllString(body, " ")
	body = strings.ReplaceAll(body, "<br>", " ")
	body = strings.ReplaceAll(body, "<br/>", " ")
	body = strings.ReplaceAll(body, "<br />", " ")
	body = htmlTagRE.ReplaceAllString(body, " ")
	body = html.UnescapeString(body)
	body = whitespaceRE.ReplaceAllString(body, " ")
	return strings.TrimSpace(body)
}

func otpContext(text, code string) string {
	idx := strings.Index(text, code)
	if idx < 0 {
		return ""
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(code) + 80
	if end > len(text) {
		end = len(text)
	}
	return strings.TrimSpace(text[start:end])
}
