# OpenAI Codex OAuth 2.0 PKCE 授权流程 — Go 实现指南

本文档面向 AI 编程助手，详细说明如何用 Go 实现 OpenAI Codex 的 OAuth 2.0 授权码 + PKCE 流程。分为两个核心部分：生成授权链接、处理回调并换取 Token。

---

## 一、生成授权链接

### 1.1 固定参数（常量）

以下参数在所有请求中保持不变，直接硬编码：

```go
const (
    // OAuth 授权端点
    AuthURL     = "https://auth.openai.com/oauth/authorize"
    // Token 交换端点
    TokenURL    = "https://auth.openai.com/oauth/token"
    // OpenAI 分配的客户端 ID（公开参数，无需保密）
    ClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
    // 本地回调地址，端口固定 1455
    RedirectURI = "http://localhost:1455/auth/callback"
    // 请求的权限范围
    Scopes      = "openid profile email offline_access api.connectors.read api.connectors.invoke"
)
```

### 1.2 动态参数（需要用函数生成）

| 参数 | 说明 | 生成方式 |
|------|------|----------|
| `state` | 防 CSRF 的随机令牌 | 16 字节 `crypto/rand` + hex 编码 |
| `code_verifier` | PKCE 验证器（保密，不发送到授权端点） | 96 字节 `crypto/rand` + base64url 编码 |
| `code_challenge` | PKCE 挑战码（发送到授权端点） | 对 `code_verifier` 做 SHA-256 后 base64url 编码 |

### 1.3 完整示例代码

```go
package oauth

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "net/url"
)

// ========== 固定常量 ==========

const (
    AuthURL     = "https://auth.openai.com/oauth/authorize"
    TokenURL    = "https://auth.openai.com/oauth/token"
    ClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
    RedirectURI = "http://localhost:1455/auth/callback"
    Scopes      = "openid profile email offline_access api.connectors.read api.connectors.invoke"
)

// ========== PKCE 相关结构和生成函数 ==========

// PKCECodes 保存 PKCE 流程所需的一对密钥
type PKCECodes struct {
    CodeVerifier  string // 保密，仅在换 Token 时使用
    CodeChallenge string // 公开，放到授权 URL 中
}

// GeneratePKCECodes 生成 PKCE 验证码对
// code_verifier: 96 字节随机数的 base64url 编码（128 字符）
// code_challenge: code_verifier 的 SHA-256 哈希的 base64url 编码
func GeneratePKCECodes() (*PKCECodes, error) {
    // 生成 code_verifier
    verifierBytes := make([]byte, 96)
    if _, err := rand.Read(verifierBytes); err != nil {
        return nil, fmt.Errorf("生成随机字节失败: %w", err)
    }
    codeVerifier := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(verifierBytes)

    // 生成 code_challenge = BASE64URL(SHA256(code_verifier))
    hash := sha256.Sum256([]byte(codeVerifier))
    codeChallenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])

    return &PKCECodes{
        CodeVerifier:  codeVerifier,
        CodeChallenge: codeChallenge,
    }, nil
}

// ========== State 生成 ==========

// GenerateState 生成 16 字节的随机 state 参数（hex 编码后 32 字符）
func GenerateState() (string, error) {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("生成 state 失败: %w", err)
    }
    return hex.EncodeToString(b), nil
}

// ========== 构建授权 URL ==========

// BuildAuthURL 拼装完整的 OAuth 授权链接
// 参数 state 和 pkceCodes 必须在本地保存，后续验证回调和换 Token 时使用
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
```

### 1.4 生成的 URL 结构示例

```
https://auth.openai.com/oauth/authorize
  ?response_type=code
  &client_id=app_EMoamEEZ73f0CkXaXp7hrann
  &redirect_uri=http%3A%2F%2Flocalhost%3A1455%2Fauth%2Fcallback
  &scope=openid+profile+email+offline_access+api.connectors.read+api.connectors.invoke
  &code_challenge=H4He5DFj1f-bKrYFc4Ql_Ma4h3zVT89mQFyZlD7zqpo    ← 动态生成
  &code_challenge_method=S256                                       ← 固定
  &id_token_add_organizations=true                                  ← 固定
  &codex_cli_simplified_flow=true                                   ← 固定
  &state=yL-SdW0zorp9RsjonisneohPKVXNC6qwkw4AYgMTt4w              ← 动态生成
```

---

## 二、回调处理：验证参数 & 换取 Token

### 2.1 回调 URL 结构

用户在浏览器完成登录后，OpenAI 会重定向到：

```
http://localhost:1455/auth/callback?code=<授权码>&state=<state>&scope=<授权范围>
```

实际示例：
```
http://localhost:1455/auth/callback
  ?code=ac_emCIOve2Xdw-da0H9oD0Cin8CAvQvwVRUybS5IHsVmk.WUdeLEgyjVp77WzyfdZt-rdeMuP6jmW6CC4BqsGv0l8
  &scope=openid+profile+email+offline_access+api.connectors.read+api.connectors.invoke
  &state=x55PPMBLxEwLoFtueXQf_i5RTx8TT-pl85opqu_J3OU
```

### 2.2 回调参数说明

| 参数 | 说明 | 是否必须验证 |
|------|------|-------------|
| `code` | 授权码，用于换取 Token | 必须非空 |
| `state` | 防 CSRF 令牌 | **必须与发起时生成的 state 完全一致** |
| `scope` | 实际授予的权限范围 | 可选检查 |
| `error` | 如果授权失败会携带此参数 | 有则表示失败 |

### 2.3 验证逻辑 + 换 Token 完整示例代码

```go
package oauth

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "time"
)

// ========== Token 响应结构 ==========

// TokenResponse 是 OpenAI Token 端点返回的 JSON 结构
type TokenResponse struct {
    IDToken      string `json:"id_token"`      // JWT 格式，包含用户信息
    AccessToken  string `json:"access_token"`  // API 访问令牌
    RefreshToken string `json:"refresh_token"` // 刷新令牌（长期有效）
    TokenType    string `json:"token_type"`    // 固定为 "Bearer"
    ExpiresIn    int    `json:"expires_in"`    // access_token 有效期（秒）
}

// OAuthResult 回调服务器收到的结果
type OAuthResult struct {
    Code  string // 授权码
    State string // state 参数
    Error string // 错误信息（如有）
}

// ========== 本地回调 HTTP 服务器 ==========

// CallbackServer 在本地启动 HTTP 服务器监听 OAuth 回调
type CallbackServer struct {
    port       int
    server     *http.Server
    resultCh   chan *OAuthResult
    mu         sync.Mutex
}

// NewCallbackServer 创建回调服务器，端口固定 1455
func NewCallbackServer(port int) *CallbackServer {
    return &CallbackServer{
        port:     port,
        resultCh: make(chan *OAuthResult, 1),
    }
}

// Start 启动回调服务器
func (s *CallbackServer) Start() error {
    // 先检查端口是否可用
    listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
    if err != nil {
        return fmt.Errorf("端口 %d 被占用: %w", s.port, err)
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
            fmt.Printf("回调服务器错误: %v\n", err)
        }
    }()

    time.Sleep(100 * time.Millisecond) // 等待服务器就绪
    return nil
}

// handleCallback 处理 /auth/callback 请求
// 核心验证逻辑在这里：提取 code、state、error 参数
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query()

    // 1. 检查是否有 error 参数（授权被拒绝等情况）
    if errParam := query.Get("error"); errParam != "" {
        s.resultCh <- &OAuthResult{Error: errParam}
        http.Error(w, fmt.Sprintf("授权失败: %s", errParam), http.StatusBadRequest)
        return
    }

    // 2. 提取 code（授权码，必须非空）
    code := query.Get("code")
    if code == "" {
        s.resultCh <- &OAuthResult{Error: "missing_code"}
        http.Error(w, "缺少授权码", http.StatusBadRequest)
        return
    }

    // 3. 提取 state（必须非空，后续需与本地保存的 state 对比）
    state := query.Get("state")
    if state == "" {
        s.resultCh <- &OAuthResult{Error: "missing_state"}
        http.Error(w, "缺少 state 参数", http.StatusBadRequest)
        return
    }

    // 4. 回调验证通过，返回成功页面
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Write([]byte(`<html><body><h1>✅ 授权成功</h1><p>可以关闭此窗口</p></body></html>`))

    // 5. 将结果发送到等待的 channel
    s.resultCh <- &OAuthResult{Code: code, State: state}
}

// WaitForResult 等待回调结果（带超时）
func (s *CallbackServer) WaitForResult(timeout time.Duration) (*OAuthResult, error) {
    select {
    case result := <-s.resultCh:
        return result, nil
    case <-time.After(timeout):
        return nil, fmt.Errorf("等待回调超时（%v）", timeout)
    }
}

// Stop 停止服务器
func (s *CallbackServer) Stop() {
    if s.server != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        s.server.Shutdown(ctx)
    }
}

// ========== 验证 State 并换取 Token ==========

// ExchangeCodeForToken 用授权码换取 Token
//
// 参数说明：
//   - code: 回调 URL 中的 code 参数
//   - state: 回调 URL 中的 state 参数
//   - expectedState: 发起授权时本地生成并保存的 state
//   - pkceCodes: 发起授权时本地生成并保存的 PKCE 密钥对
//
// 验证逻辑：
//   1. state 必须与 expectedState 完全匹配（防 CSRF）
//   2. 用 code + code_verifier 向 Token 端点换取 Token（PKCE 验证）
func ExchangeCodeForToken(
    ctx context.Context,
    code string,
    state string,
    expectedState string,
    pkceCodes *PKCECodes,
) (*TokenResponse, error) {

    // ====== 步骤 1：验证 state ======
    // 如果 state 不匹配，说明可能遭受 CSRF 攻击，必须拒绝
    if state != expectedState {
        return nil, fmt.Errorf("state 不匹配: expected=%s, got=%s", expectedState, state)
    }

    // ====== 步骤 2：构建 Token 请求 ======
    // POST https://auth.openai.com/oauth/token
    // Content-Type: application/x-www-form-urlencoded
    data := url.Values{
        "grant_type":    {"authorization_code"},
        "client_id":     {ClientID},
        "code":          {code},          // 回调拿到的授权码
        "redirect_uri":  {RedirectURI},   // 必须与授权请求中的一致
        "code_verifier": {pkceCodes.CodeVerifier}, // PKCE：服务端用它验证 code_challenge
    }

    req, err := http.NewRequestWithContext(ctx, "POST", TokenURL, strings.NewReader(data.Encode()))
    if err != nil {
        return nil, fmt.Errorf("创建请求失败: %w", err)
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Accept", "application/json")

    // ====== 步骤 3：发送请求 ======
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("Token 请求失败: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("读取响应失败: %w", err)
    }

    // ====== 步骤 4：检查响应状态 ======
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("Token 交换失败: status=%d, body=%s", resp.StatusCode, string(body))
    }

    // ====== 步骤 5：解析 Token 响应 ======
    var tokenResp TokenResponse
    if err := json.Unmarshal(body, &tokenResp); err != nil {
        return nil, fmt.Errorf("解析 Token 响应失败: %w", err)
    }

    return &tokenResp, nil
}
```

### 2.4 完整调用流程串联

```go
package main

import (
    "context"
    "fmt"
    "os/exec"
    "runtime"
    "time"

    "your-module/oauth" // 引用上面的包
)

func main() {
    // ===== 第一步：生成动态参数 =====
    pkceCodes, err := oauth.GeneratePKCECodes()
    if err != nil {
        panic(err)
    }

    state, err := oauth.GenerateState()
    if err != nil {
        panic(err)
    }

    // ===== 第二步：构建授权 URL =====
    authURL := oauth.BuildAuthURL(state, pkceCodes)
    fmt.Printf("授权链接: %s\n", authURL)

    // ===== 第三步：启动本地回调服务器 =====
    server := oauth.NewCallbackServer(1455)
    if err := server.Start(); err != nil {
        panic(err)
    }
    defer server.Stop()

    // ===== 第四步：打开浏览器让用户登录 =====
    openBrowser(authURL)

    // ===== 第五步：等待回调 =====
    fmt.Println("等待用户完成授权...")
    result, err := server.WaitForResult(5 * time.Minute)
    if err != nil {
        panic(err)
    }

    // 检查回调是否返回了错误
    if result.Error != "" {
        panic(fmt.Sprintf("OAuth 错误: %s", result.Error))
    }

    // ===== 第六步：验证 state + 用 code 换 Token =====
    tokenResp, err := oauth.ExchangeCodeForToken(
        context.Background(),
        result.Code,     // 回调中的 code
        result.State,    // 回调中的 state
        state,           // 本地保存的 state（用于对比验证）
        pkceCodes,       // 本地保存的 PKCE codes（code_verifier 用于换 Token）
    )
    if err != nil {
        panic(err)
    }

    // ===== 第七步：使用 Token =====
    fmt.Printf("access_token: %s...\n", tokenResp.AccessToken[:20])
    fmt.Printf("refresh_token: %s...\n", tokenResp.RefreshToken[:20])
    fmt.Printf("expires_in: %d 秒\n", tokenResp.ExpiresIn)
}

func openBrowser(url string) {
    switch runtime.GOOS {
    case "darwin":
        exec.Command("open", url).Start()
    case "linux":
        exec.Command("xdg-open", url).Start()
    case "windows":
        exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
    }
}
```

---

## 三、安全要点总结

| 要点 | 说明 |
|------|------|
| `state` 必须验证 | 回调中的 `state` 必须与发起时的一致，否则拒绝（防 CSRF） |
| `code_verifier` 绝不暴露到浏览器 | 它只在本地生成、本地保存、发给 Token 端点（server-to-server） |
| `code_challenge` 是单向的 | 即使被截获也无法反推出 `code_verifier` |
| `code` 一次性使用 | 授权码只能用一次，且有短有效期（通常几分钟） |
| Token 端点无需 client_secret | 因为使用了 PKCE，`code_verifier` 替代了 client_secret 的角色 |
| 回调端口固定 `1455` | 必须与 OpenAI 注册的 redirect_uri 一致 |
| 超时控制 | 建议 5 分钟超时，超时后清理本地状态 |

---

## 四、Token 刷新（可选）

当 `access_token` 过期后，使用 `refresh_token` 获取新的 Token：

```go
// RefreshToken 使用 refresh_token 换取新的 access_token
func RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
    data := url.Values{
        "grant_type":    {"refresh_token"},
        "client_id":     {ClientID},
        "refresh_token": {refreshToken},
    }

    req, err := http.NewRequestWithContext(ctx, "POST", TokenURL, strings.NewReader(data.Encode()))
    if err != nil {
        return nil, fmt.Errorf("创建刷新请求失败: %w", err)
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Accept", "application/json")

    client := &http.Client{Timeout: 25 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("刷新请求失败: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("读取响应失败: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("Token 刷新失败: status=%d, body=%s", resp.StatusCode, string(body))
    }

    var tokenResp TokenResponse
    if err := json.Unmarshal(body, &tokenResp); err != nil {
        return nil, fmt.Errorf("解析刷新响应失败: %w", err)
    }

    return &tokenResp, nil
}
```

---

## 五、流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                        本地应用                                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  1. 生成 state (随机 16 字节 hex)                                  │
│  2. 生成 code_verifier (随机 96 字节 base64url)                    │
│  3. 计算 code_challenge = BASE64URL(SHA256(code_verifier))         │
│  4. 拼装授权 URL                                                   │
│  5. 启动本地 HTTP 服务器监听 127.0.0.1:1455                        │
│  6. 打开浏览器访问授权 URL                                          │
│                                                                   │
│         ┌──────── 用户在浏览器完成登录 ────────┐                    │
│         ▼                                      │                  │
│  7. 浏览器重定向到 localhost:1455/auth/callback │                  │
│     携带参数: code, state, scope               │                  │
│                                                │                  │
│  8. 验证 state == 本地保存的 state              │                  │
│     (不匹配则拒绝 → 防 CSRF)                   │                  │
│                                                │                  │
│  9. POST https://auth.openai.com/oauth/token   │                  │
│     Body:                                      │                  │
│       grant_type=authorization_code            │                  │
│       client_id=app_EMoamEEZ73f0CkXaXp7hrann   │                  │
│       code=<回调中的code>                       │                  │
│       redirect_uri=http://localhost:1455/auth/callback            │
│       code_verifier=<本地保存的code_verifier>    │                  │
│                                                │                  │
│ 10. 解析响应获取:                               │                  │
│       id_token (JWT, 含用户信息)                │                  │
│       access_token (API 访问令牌)              │                  │
│       refresh_token (刷新令牌)                  │                  │
│       expires_in (过期时间/秒)                  │                  │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```
