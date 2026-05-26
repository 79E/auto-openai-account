package legacy

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func Clean(v any) string {
	return strings.TrimSpace(fmt.Sprint(ValueOr(v, "")))
}

func ValueOr(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}

func StringMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func CopyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func AsStringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s := Clean(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func AsMapSlice(v any) []map[string]any {
	switch x := v.(type) {
	case []map[string]any:
		return x
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func ToInt(v any, fallback int) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, err := x.Int64()
		if err == nil {
			return int(n)
		}
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err == nil {
			return n
		}
	}
	return fallback
}

func ToBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "1", "true", "yes", "on":
			return true
		}
		return false
	default:
		return v != nil
	}
}

func DecodeJSON(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(out)
}

func NewUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func NewHex(n int) string {
	if n <= 0 {
		n = 16
	}
	buf := make([]byte, (n+1)/2)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)[:n]
}

func SHA256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func SHA1Short(text string, n int) string {
	sum := sha1.Sum([]byte(text))
	hexed := hex.EncodeToString(sum[:])
	if n > 0 && n < len(hexed) {
		return hexed[:n]
	}
	return hexed
}

func RandomTokenURL(n int) string {
	if n <= 0 {
		n = 24
	}
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func B64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func B64Decode(text string) ([]byte, error) {
	if idx := strings.Index(text, ","); strings.HasPrefix(text, "data:") && idx >= 0 {
		text = text[idx+1:]
	}
	return base64.StdEncoding.DecodeString(strings.TrimSpace(text))
}

func CompactJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return string(data)
	}
	return buf.String()
}

func NowLocal() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

var LogHook func(email, message string)

func logStep(email, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if LogHook != nil {
		LogHook(Clean(email), message)
	}
	if Clean(email) != "" {
		fmt.Printf("[%s] [%s] %s\n", NowLocal(), Clean(email), message)
		return
	}
	fmt.Printf("[%s] %s\n", NowLocal(), message)
}

func shortSecretHash(value string) string {
	value = Clean(value)
	if value == "" {
		return "empty"
	}
	return SHA256Hex(value)[:10]
}

func ExplainError(message string) string {
	message = Clean(message)
	if message == "" {
		return ""
	}
	lower := strings.ToLower(message)
	var reason string
	switch {
	case strings.Contains(lower, "platform_authorize_entered_login_flow") || strings.Contains(lower, "/log-in/password"):
		reason = "注册失败：OpenAI 返回了登录密码页面，而不是新账号创建页面。通常表示该邮箱已经注册过、存在半成品账号，或被上游识别为已有账号/关联登录方式。建议换一个从未注册过的邮箱，或改走登录换 token 流程。"
	case strings.Contains(lower, "otp timeout"):
		reason = "验证码获取超时：在设定时间内没有从邮箱 INBOX 读取到符合条件的 6 位验证码。请检查 IMAP 配置、邮箱 access_token/密码、验证码邮件是否到达、是否进了垃圾箱，或适当增加等待时间。"
	case strings.Contains(lower, "imap command failed") || strings.Contains(lower, "authenticate xoauth2") || strings.Contains(lower, "imap xoauth2 token is empty"):
		reason = "邮箱登录失败：IMAP 认证没有通过。请检查邮箱 access_token 是否有效，或将 IMAP 认证模式改为 password/auto 并确认邮箱密码可用。"
	case strings.Contains(lower, "send_otp_http_"):
		reason = "发送邮箱验证码失败：上游没有接受发送验证码请求，可能是网络、代理、风控或当前注册会话异常。"
	case strings.Contains(lower, "validate_otp_http_"):
		reason = "验证码校验失败：读取到的验证码未被上游接受，可能验证码已过期、读取到了旧验证码，或当前注册会话已失效。"
	case strings.Contains(lower, "user_register_http_"):
		reason = "提交注册密码失败：上游拒绝创建账号，可能是邮箱域名、代理环境、密码规则或风控导致。"
	case strings.Contains(lower, "create_account_http_"):
		reason = "创建账号资料失败：验证码校验后创建账号阶段被上游拒绝，可能是风控、代理环境或邮箱域名限制导致。"
	case strings.Contains(lower, "password_verify_http_"):
		reason = "登录密码校验失败：账号密码没有通过上游校验，请确认保存的注册密码是否正确。"
	case strings.Contains(lower, "oauth_token_http_") || strings.Contains(lower, "token exchange"):
		reason = "换取 token 失败：上游授权成功后没有正常返回 token，可能是授权回调、代理或会话状态异常。"
	default:
		reason = "操作失败：程序收到上游或本地流程错误，暂未匹配到更具体的中文原因。请结合原始错误排查。"
	}
	return reason + "\n原始错误：" + message
}

func AnonymizeToken(token any) string {
	value := Clean(token)
	if value == "" {
		return "token:empty"
	}
	return "token:" + SHA256Hex(value)[:10]
}

func ParseCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
