# auto-openai-account

[English](README-EN.md)

`auto-openai-account` 是一个本地优先的账号自动化控制台，后端使用 Go，前端使用 React。

它的目标是用一个 Go 服务同时提供：

- `/api/*` 下的 JSON API 和 Server-Sent Events 实时事件
- `/` 下的 React 单页应用

这个项目主要用于本地自动化工作流，包括邮箱管理、代理配置、注册/登录任务、运行日志和 token 导出。

## 功能

- 邮箱导入和管理
- 系统设置管理
- 代理池配置和连通性测试
- 注册任务创建、停止、进度、详情和日志
- 登录/token 刷新任务
- 运行日志写入 SQLite，并通过 SSE 实时推送
- 已完成任务的 token 导出
- Go 服务直接提供 React UI

## 技术栈

- 后端：Go
- 存储：SQLite，使用 `modernc.org/sqlite`
- 前端：React、TypeScript、Vite、Tailwind CSS
- UI 交付：Go 服务提供 Vite 构建产物

## 项目结构

```text
apps/
  server/       Go 服务入口
  web/          React + Vite 前端
internal/
  api/          HTTP 路由、接口处理、请求/响应格式化
  domain/       共享业务类型、状态常量、默认配置
  storage/      SQLite 表结构和持久化逻辑
  runner/       任务生命周期、并发、取消、日志、SSE 推送
  legacy/       账号自动化实现，隔离在当前应用边界之后
  proxypool/    代理连通性测试
  webui/        静态 UI 处理器
docs/
  api.md        当前 API 文档
  architecture.md
  design.md
  requirements.md
```

## 环境要求

- Go 1.25 或更新版本，与 `go.mod` 保持一致
- Node.js 和 npm，用于前端开发和构建

## 本地开发

### 只启动后端

```bash
go run ./apps/server
```

默认访问地址：

```text
http://localhost:8080
```

可以通过环境变量修改监听地址：

```bash
AUTO_OPENAI_ACCOUNT_LISTEN=:9090 go run ./apps/server
```

### 启动前端开发服务器

一个终端启动后端：

```bash
go run ./apps/server
```

另一个终端启动 Vite：

```bash
cd apps/web
npm run dev
```

Vite 开发服务器会把 `/api` 请求代理到 `http://localhost:8080`。

## 构建

### 构建前端资源

```bash
cd apps/web
npm run build
```

当前 Vite 配置会把前端构建产物输出到仓库根目录的 `dist/`。

### 构建后端二进制

```bash
go build -o auto-openai-account ./apps/server
```

### 构建完整本地发布包

```bash
cd apps/web
npm run build
cd ../..
go build -o auto-openai-account ./apps/server
```

从仓库根目录运行二进制，这样服务可以读取 `dist/` 并把运行时数据写入 `data/`：

```bash
./auto-openai-account
```

打开：

```text
http://localhost:8080
```

## 部署

单机部署建议流程：

1. 使用 `cd apps/web && npm run build` 构建前端。
2. 使用 `go build -o auto-openai-account ./apps/server` 构建 Go 二进制。
3. 部署二进制文件和生成的 `dist/` 目录。
4. 在包含 `dist/` 的目录中运行二进制。
5. 持久化 `data/` 目录，因为 SQLite 运行时数据库在这里。

示例部署目录：

```text
release/
  auto-openai-account
  dist/
    index.html
    assets/
  data/
    register.db
```

指定端口启动：

```bash
AUTO_OPENAI_ACCOUNT_LISTEN=:8080 ./auto-openai-account
```

## 验证

后端测试：

```bash
go test ./...
```

前端构建检查：

```bash
cd apps/web
npm run build
```

如果一次改动同时影响后端和前端接口，需要同时运行这两个命令，并确认 `docs/api.md` 和 `apps/web/src/types.ts` 保持一致。

## 运行时数据

默认情况下，SQLite 运行时文件保存在 `data/`：

```text
data/register.db
data/register.db-shm
data/register.db-wal
```

除非你明确想删除本地数据，否则不要删除 `data/`。

## API 文档

当前 API 合同见 `docs/api.md`。

主要接口分组：

- 健康检查：`/api/health`
- 设置：`/api/settings`
- 邮箱：`/api/mailboxes`
- 任务：`/api/register-jobs`、`/api/login-jobs`
- 日志和事件：`/api/register-jobs/{id}/logs`、`/api/register-jobs/{id}/events`
- 代理测试：`/api/proxy/test`
- 统计：`/api/stats`

错误响应格式：

```json
{
  "error": "message"
}
```

## 给 Agent 的说明

非简单修改前，请先阅读 `AGENTS.md`。

推荐上下文文件：

- `AGENTS.md`
- `docs/requirements.md`
- `docs/architecture.md`
- `docs/design.md`
- `docs/api.md`

开发规则：

- API handler 放在 `internal/api`。
- 共享 JSON 字段和状态常量放在 `internal/domain`。
- 持久化逻辑放在 `internal/storage`。
- 任务生命周期、取消、运行日志和 SSE 放在 `internal/runner`。
- 前端 API 类型在 `apps/web/src/types.ts`，需要和 Go JSON 类型保持一致。
- 不要删除 `data/`，不要回滚无关工作区改动。

## Git 忽略说明

以下生成文件或运行时文件不应提交：

- `data/`
- `*.db`、`*.db-shm`、`*.db-wal`
- `node_modules/`
- 前端构建产物，除非任务明确要求处理发布包

## 免责声明

本项目代码仅供编程学习与学术研究使用，例如探讨 OAuth2 授权机制、TLS 指纹安全及相关对抗技术。

请勿将本项目用于任何非法用途、大规模恶意注册，或违反平台服务条款（TOS）的商业行为。

OpenAI 的接口风控策略经常变动，本项目不保证代码的永久可用性。因使用本项目带来的任何封号风险、服务限制、法律纠纷或其他后果，均由使用者自行承担，与开发者无关。

## 许可证

见 `LICENSE`。
