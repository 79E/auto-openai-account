# OpenAI 账户订阅与额度查询逻辑

本文档梳理 cockpit-tools 项目中查询 OpenAI（Codex）账户订阅状态和使用额度的完整逻辑链路。

## 一、整体架构

项目采用 Tauri（Rust 后端 + TypeScript 前端）架构，查询逻辑分为三层：

```
前端 Service 层 (TypeScript)
    ↓ invoke()
Tauri Command 层 (Rust commands/codex.rs)
    ↓
业务模块层 (Rust modules/codex_quota.rs + codex_account.rs)
    ↓ HTTP 请求
OpenAI 后端 API
```

## 二、核心 API 端点

项目调用了三个 OpenAI 官方后端接口来获取账户信息：

| 端点 | 地址 | 用途 |
|------|------|------|
| Usage（配额使用率） | `https://chatgpt.com/backend-api/wham/usage` | 获取实时用量百分比、重置时间 |
| Accounts Check（订阅账号检查） | `https://chatgpt.com/backend-api/accounts/check/v4-2023-04-27` | 获取 plan_type、订阅到期时间、account_id |
| Subscriptions（订阅详情） | `https://chatgpt.com/backend-api/subscriptions` | 补充获取订阅计划类型和到期时间 |

此外，对于 Cockpit Api（第三方代理）账号，使用自定义端点：

| 端点 | 地址 | 用途 |
|------|------|------|
| Token Profile | `{api_base_url}/api/cockpit-tools/token-profile` | 获取代理额度（总量/已用/可用/是否无限） |

## 三、前端调用入口

### 3.1 TypeScript Service 层（`src/services/codexService.ts`）

```typescript
// 刷新单个账号配额
export async function refreshCodexQuota(accountId: string): Promise<CodexQuota> {
  return await invoke('refresh_codex_quota', { accountId });
}

// 强制刷新单个账号的订阅信息
export async function refreshCodexSubscriptionInfo(accountId: string): Promise<CodexAccount> {
  return await invoke('refresh_codex_subscription_info', { accountId });
}

// 刷新所有账号配额
export async function refreshAllCodexQuotas(): Promise<number> {
  return await invoke('refresh_all_codex_quotas');
}
```

### 3.2 TypeScript 类型定义（`src/types/codex.ts`）

配额数据结构：

```typescript
export interface CodexQuota {
  hourly_percentage: number;       // 5小时配额剩余百分比 (0-100)
  hourly_reset_time?: number;      // 5小时配额重置时间 (Unix timestamp)
  hourly_window_minutes?: number;  // 主窗口时长（分钟）
  hourly_window_present?: boolean; // 主窗口是否存在
  weekly_percentage: number;       // 周配额剩余百分比 (0-100)
  weekly_reset_time?: number;      // 周配额重置时间 (Unix timestamp)
  weekly_window_minutes?: number;  // 次窗口时长（分钟）
  weekly_window_present?: boolean; // 次窗口是否存在
  raw_data?: unknown;              // 原始响应数据
}
```

## 四、Tauri Command 层（`src-tauri/src/commands/codex.rs`）

Command 层作为桥梁，调用业务模块并在成功后触发 UI 更新：

```rust
#[tauri::command]
pub async fn refresh_codex_quota(app: AppHandle, account_id: String) -> Result<CodexQuota, String> {
    let result = codex_quota::refresh_account_quota(&account_id).await;
    if result.is_ok() {
        run_codex_post_refresh_checks(&app).await;  // 触发自动切换检查等后处理
        let _ = crate::modules::tray::update_tray_menu(&app);  // 更新托盘菜单
    }
    result
}

#[tauri::command]
pub async fn refresh_all_codex_quotas(app: AppHandle) -> Result<i32, String> {
    let results = codex_quota::refresh_all_quotas().await?;
    let success_count = results.iter().filter(|(_, r)| r.is_ok()).count();
    // ... 返回成功刷新的数量
}

#[tauri::command]
pub async fn refresh_codex_subscription_info(
    app: AppHandle,
    account_id: String,
) -> Result<CodexAccount, String> {
    codex_quota::refresh_account_subscription_info(&account_id, true).await
}
```

## 五、核心业务逻辑（`src-tauri/src/modules/codex_quota.rs`）

### 5.1 刷新账号配额的主流程 `refresh_account_quota_once`

这是最核心的函数，完整流程如下：

```
1. 加载并准备账号数据 (prepare_account_for_injection)
2. 判断账号类型：
   ├─ API Key 账号 + Cockpit Api → 调用 fetch_new_api_quota()
   ├─ 普通 API Key 账号 → 返回不支持
   └─ OAuth 账号 → 继续下面的流程
3. 检查 access_token 是否过期
   ├─ 过期 → 刷新 Token (force_refresh_managed_account)
   └─ 未过期 → 继续
4. 调用 fetch_quota() 获取使用率
   ├─ 成功 → 从响应同步 plan_type
   └─ 失败 → 记录错误，尝试补拉订阅信息
5. 调用 refresh_subscription_state() 刷新订阅状态
6. 保存账号数据到本地
```

### 5.2 配额查询 `fetch_quota`

调用 `https://chatgpt.com/backend-api/wham/usage` 获取使用率数据。

请求头：
- `Authorization: Bearer {access_token}`
- `ChatGPT-Account-Id: {account_id}`（从 JWT 中提取或账号记录中获取）
- `Accept: application/json`

响应结构（`UsageResponse`）：

```rust
struct UsageResponse {
    plan_type: Option<String>,          // 订阅类型 (plus/pro/team/enterprise 等)
    rate_limit: Option<RateLimitInfo>,   // 使用率限制
    code_review_rate_limit: Option<RateLimitInfo>,  // 代码审查配额
}

struct RateLimitInfo {
    allowed: Option<bool>,
    limit_reached: Option<bool>,
    primary_window: Option<WindowInfo>,    // 主窗口（5小时配额）
    secondary_window: Option<WindowInfo>,  // 次窗口（周配额）
}

struct WindowInfo {
    used_percent: Option<i32>,           // 已使用百分比
    limit_window_seconds: Option<i64>,   // 窗口时长(秒)
    reset_after_seconds: Option<i64>,    // 距重置剩余秒数
    reset_at: Option<i64>,              // 重置时间点(Unix timestamp)
}
```

百分比计算逻辑：`剩余百分比 = 100 - used_percent`（取值 0-100）

### 5.3 订阅状态查询 `refresh_subscription_state`

采用两步策略获取订阅信息：

**第一步：Accounts Check**

```
GET https://chatgpt.com/backend-api/accounts/check/v4-2023-04-27?timezone_offset_min={offset}
```

请求头包含：
- `Authorization: Bearer {access_token}`
- `Referer: https://chatgpt.com/`
- `User-Agent: Mozilla/5.0 ...`（模拟浏览器）
- `x-openai-target-path` / `x-openai-target-route`

从响应中解析：
- `account_id`：从 `accounts` 数组中匹配当前账号
- `plan_type`：从 `entitlement.subscription_plan` 或 `account.plan_type` 获取
- `subscription_active_until`：从 `entitlement.expires_at` 或 `account.expires_at` 获取

**第二步：Subscriptions（条件触发）**

仅当 Accounts Check 未返回有效订阅到期时间时才执行：

```
GET https://chatgpt.com/backend-api/subscriptions?account_id={account_id}
```

从响应中补充获取 `subscription_plan` 和 `active_until`。

### 5.4 重试与冷却机制

订阅查询失败时有冷却机制：

```rust
const SUBSCRIPTION_RETRY_INTERVAL_SECONDS: i64 = 30 * 60;  // 30分钟冷却

// 失败后标记下次重试时间
account.subscription_query_next_retry_at = Some(now + 30 * 60);
account.subscription_query_last_error = Some(error_message);

// 仅在冷却期过后才允许重新查询（除非 force=true）
fn should_attempt_subscription_refresh(account, options) -> bool {
    if 订阅未过期 && !force → false
    if force → true
    if 下次重试时间 <= 当前时间 → true
}
```

### 5.5 Cockpit Api 额度查询 `fetch_new_api_quota`

针对第三方代理账号的额度查询：

```
GET {api_base_url}/api/cockpit-tools/token-profile
Authorization: Bearer {openai_api_key}
```

响应中提取：
- `usage.total_granted`：总授予额度
- `usage.total_used`：已使用额度
- `usage.total_available`：可用额度
- `usage.unlimited_quota`：是否无限额度
- `usage.expires_at`：到期时间

百分比计算：`percentage = (available / total) * 100`（无限额度返回 100%）

### 5.6 刷新所有账号配额 `refresh_all_quotas`

并发控制：最多 5 个并发请求（`Semaphore`），过滤掉不支持的纯 API Key 账号。

```rust
const MAX_CONCURRENT: usize = 5;
// 过滤条件：非 API Key 账号 或 Cockpit Api 账号
accounts.filter(|a| !a.is_api_key_auth() || is_new_api_account(a))
```

## 六、Token 管理与鉴权

### 6.1 JWT 结构

OpenAI 的 id_token / access_token 是 JWT，其中嵌套的认证数据路径为：

```json
{
  "https://api.openai.com/auth": {
    "chatgpt_user_id": "...",
    "chatgpt_plan_type": "plus|pro|team|enterprise|free",
    "chatgpt_subscription_active_until": "timestamp_or_datetime",
    "account_id": "...",
    "organization_id": "..."
  },
  "https://api.openai.com/profile": {
    "email": "...",
    "email_verified": true
  }
}
```

### 6.2 Token 过期检测与自动刷新

在查询配额前，会检测 access_token 是否过期：
- 过期 → 调用 `force_refresh_managed_account` 使用 refresh_token 换取新 token
- 刷新成功后从新 id_token 同步订阅信息
- 刷新失败则记录错误并中止配额查询

### 6.3 订阅信息同步 `sync_subscription_from_token`

多个来源都可能更新账号的订阅信息：

1. JWT 中的 `chatgpt_plan_type` 和 `chatgpt_subscription_active_until`
2. Usage API 响应中的 `plan_type`
3. Accounts Check / Subscriptions API 的专门查询结果

更新时会同步写入账号索引（`CodexAccountIndex`），保持一致性。

## 七、错误处理

### 7.1 配额错误记录

查询失败时写入 `quota_error` 字段：

```rust
struct CodexQuotaErrorInfo {
    code: Option<String>,    // 错误码（从响应体或消息中提取）
    message: String,         // 错误描述
    timestamp: i64,          // 发生时间
}
```

错误码提取逻辑支持多种格式：
- `{"detail": {"code": "xxx"}}`
- `{"error": {"code": "xxx"}}`
- `[error_code:xxx]` 标记
- `error_code=xxx` 标记

### 7.2 HTTP 响应日志

每次请求都会记录详细日志，包含：url、status、request-id、x-request-id、cf-ray、body_len

## 八、数据持久化

查询结果持久化到本地文件系统：

```
{data_dir}/
├── codex/
│   ├── accounts.json          # 账号索引（CodexAccountIndex）
│   └── accounts/
│       └── {account_id}.json  # 单个账号详情（含 quota/quota_error/usage_updated_at）
└── cache/
    └── quota_api_v1_desktop/  # API 响应缓存
        └── authorized/
            └── {hash(email)}.json
```

## 九、前端展示逻辑

### 9.1 配额窗口展示

前端将配额数据转换为可展示的窗口列表（`getCodexQuotaWindows`），包含：
- `id`：primary（5小时）或 secondary（周）
- `label`：根据 `window_minutes` 动态计算（如 "5h"、"Weekly"、"3d"）
- `percentage`：剩余百分比
- `resetTime`：重置时间

特殊逻辑：当周配额为 0% 时，强制将小时配额也显示为 0%（`weeklyBlocksHourly`）。

### 9.2 订阅状态展示

`getCodexSubscriptionPresentation` 将到期时间转换为分级状态：

| 状态 | 条件 | 展示色调 |
|------|------|----------|
| missing | 无到期时间 | missing |
| expired | 已过期 | expired |
| within_24h | 24小时内到期 | warning |
| within_7d | 7天内到期 | warning |
| within_30d | 30天内到期 | active |
| active | 30天以上 | active |

### 9.3 Plan 类型展示

`getCodexPlanDisplayName` 映射：

| 原始值 | 展示 |
|--------|------|
| 含 "team" | TEAM |
| 含 "enterprise" | ENTERPRISE |
| 含 "plus" | PLUS |
| 含 "pro" | PRO / PRO 5x / PRO 20x |
| 空/free | FREE |
| Cockpit Api | Cockpit Api |

Pro 账号细分：通过 `auth_file_plan_type` 区分 PRO 5x（prolite）和 PRO 20x（promax，默认）。

## 十、账号类型判断

| 类型 | 判断条件 | 配额查询方式 |
|------|----------|-------------|
| OAuth 账号 | `auth_mode != "apikey"` | 调用 OpenAI 官方 API |
| 普通 API Key | `auth_mode == "apikey"` 且非 Cockpit Api | 不支持配额查询 |
| Cockpit Api | `api_provider_id` 为 "cockpit_api"/"new_api"，或 base_url 匹配 `chongcodex.cn` | 调用自定义 token-profile 接口 |

## 十一、调用时序总结

```
用户点击"刷新配额"
    ↓
refreshCodexQuota(accountId)           [前端 TypeScript]
    ↓ invoke
refresh_codex_quota(app, account_id)   [Tauri Command]
    ↓
refresh_account_quota(account_id)      [codex_quota 模块]
    ↓
prepare_account_for_injection          → 加载账号、检查 Token 新鲜度
    ↓
[Token 过期?] → force_refresh_managed_account → 刷新 Token
    ↓
fetch_quota(account)                   → GET /backend-api/wham/usage
    ↓
refresh_subscription_state(account)    → GET /backend-api/accounts/check
                                       → GET /backend-api/subscriptions (条件)
    ↓
save_account(account)                  → 持久化到本地 JSON
    ↓
返回 CodexQuota → 前端更新 UI
```
