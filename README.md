# GSWSS — GS Protocol

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://dash.cloudflare.com/?to=/%3Aaccount/workers-and-pages/create)

面向 [Cloudflare Workers](https://developers.cloudflare.com/workers/) 设计的轻量级安全传输协议（**GSP1**）。

**GSWSS** 不是公共 VPN，不提供任何公共节点。你需要自行部署 Worker，并在本地运行客户端，通过 WSS 加密隧道转发 TCP 流量。

## 特性

| 功能 | 状态 |
|------|------|
| SOCKS5 代理（CONNECT） | ✅ |
| HTTP 代理（CONNECT / GET / POST） | ✅ |
| WebSocket 二进制传输（WSS） | ✅ |
| 密码认证 | ✅ |
| TCP Relay（Worker `connect()`） | ✅ |
| 心跳保活（PING / PONG，30s） | ✅ |
| 连接速率限制 | ✅ |
| 目标地址校验 | ✅ |
| Mux 多路复用 | 🔜 |
| 自动重连 | 🔜 |
| GUI（Wails） | 🔜 |
| UDP / QUIC | 🔜 |

## 架构

```
┌─────────────┐     SOCKS5 / HTTP      ┌──────────────┐
│  Browser /  │ ─────────────────────► │  GS Client   │
│  Application│                        │  (Go CLI)    │
└─────────────┘                        └──────┬───────┘
                                              │ WSS + GSP1
                                              ▼
                                       ┌──────────────┐
                                       │  Cloudflare  │
                                       │    Worker    │
                                       └──────┬───────┘
                                              │ TCP connect()
                                              ▼
                                       ┌──────────────┐
                                       │   Internet   │
                                       └──────────────┘
```

## 仓库结构

```
GSWSS/
├── protocol/          # GSP1 协议定义（Go 帧编解码）
├── client/          # Go 客户端（CLI）
│   ├── cmd/gs/      # 入口
│   └── internal/    # transport / proxy / config / log
├── worker/          # Cloudflare Worker 源码（TypeScript + Hono）
├── wrangler.toml    # Worker 一键部署配置（根目录）
├── package.json     # Worker 构建/部署脚本
├── examples/        # 配置示例
├── dist/            # 预编译客户端 (windows-amd64/gs.exe)
├── scripts/         # 构建与一键部署脚本
│   ├── deploy.ps1           # 一键部署（Worker + 客户端 + 配置）
│   ├── deploy-worker.ps1    # Worker 一键部署 (Windows)
│   ├── deploy-worker.sh     # Worker 一键部署 (Linux/macOS)
│   └── build-windows.ps1    # 构建 gs.exe
├── product.md       # 产品设计文档
└── README.md
```

## 环境要求

| 组件 | 要求 |
|------|------|
| 客户端 | Go 1.24+（或使用预编译 `gs.exe`） |
| Worker | Node.js 18+、Cloudflare 账号、[Wrangler](https://developers.cloudflare.com/workers/wrangler/) |
| 平台 | Windows（首版）、macOS / Linux（Go 交叉编译） |

## Worker 一键部署

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://dash.cloudflare.com/?to=/%3Aaccount/workers-and-pages/create)

点击按钮进入 Cloudflare **「选择一种方法」** 页面，按以下步骤操作：

1. **选择一种方法** → 点击 **Continue with GitHub**（继续使用 GitHub）
2. 授权并选择仓库 **`gsvps/GSWSS`**（或你的 fork）
3. **创建和部署** → 确认根目录 **`wrangler.toml` → `[vars]`** 中的变量：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WEBSOCKET_PATH` | WebSocket 入口路径 | `/ws`（可改为 `/proxy`） |
| `PASSWORD` | 认证密码 | `change-me`（**部署前请修改**） |

4. 确认 **Deploy command** 为 `npm run deploy`，**根目录（路径）** 留空
5. 点击 **Deploy（部署）**

> 此方式**连接你已有的 GitHub 仓库**，不会在 GitHub 下新建 fork。

直接链接（与按钮相同）：

```
https://dash.cloudflare.com/?to=/%3Aaccount/workers-and-pages/create
```

> **注意：** 不要使用 `.../create/workers/new`，该链接会跳过「选择一种方法」直接进入 Hello World 部署页。

部署完成后 WebSocket 端点（路径由 `WEBSOCKET_PATH` 决定）：

```
https://<your-worker>.workers.dev/ws
# 若 WEBSOCKET_PATH = "/proxy" 则为：
# https://<your-worker>.workers.dev/proxy
```

### 高级设置默认值

| 参数 | 默认值 | 来源 |
|------|--------|------|
| 构建命令 | 留空 | 无需构建 |
| 部署命令 | `npm run deploy` | `package.json` |
| 预览部署命令 | `npx wrangler versions upload` | Cloudflare 默认 |
| Node.js 版本 | `22` | `.nvmrc` |
| 生产分支 | `main` | 仓库默认分支 |
| 路径（根目录） | **留空** | 根目录 `wrangler.toml` 指向 `worker/src/index.ts` |

<details>
<summary>备选：模板一键部署（会在 GitHub 新建 fork 仓库）</summary>

若希望 Cloudflare 自动 fork 模板仓库并预填配置，可使用：

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/gsvps/GSWSS/tree/main)

</details>

### 手动部署

```powershell
git clone https://github.com/gsvps/GSWSS.git
cd GSWSS
# 编辑 wrangler.toml [vars] 中的 WEBSOCKET_PATH 与 PASSWORD
npm install
npx wrangler login
npm run deploy:cloudflare
```

### 脚本部署

**Windows：** `.\scripts\deploy-worker.ps1`  
**Linux / macOS：** `./scripts/deploy-worker.sh`

### GitHub Actions 自动部署

推送 `main` 且改动 Worker 相关文件时，会自动运行 [Deploy Worker](.github/workflows/deploy-worker.yml)。

**首次使用前，必须在 GitHub 仓库配置 Secrets**（否则 CI 会跳过部署，不会报错中断）：

| Secret | 必填 | 说明 |
|--------|------|------|
| `CLOUDFLARE_API_TOKEN` | 是 | Cloudflare API Token |
| `CLOUDFLARE_ACCOUNT_ID` | 是 | Cloudflare 账户 ID |
| `WORKER_PASSWORD` | 否 | 覆盖 `wrangler.toml` 中的 `PASSWORD`；不填则使用仓库默认值 |

**配置步骤：**

1. **获取 Account ID**  
   打开 [Cloudflare Dashboard](https://dash.cloudflare.com/) → 右侧 **Account ID** 复制。

2. **创建 API Token**  
   [Create Token](https://dash.cloudflare.com/profile/api-tokens) → **Create Custom Token**：
   - Permissions：`Account` → `Cloudflare Workers Scripts` → **Edit**
   - Account Resources：选择你的账户
   - 创建后复制 Token（只显示一次）

3. **写入 GitHub Secrets**  
   仓库 → **Settings** → **Secrets and variables** → **Actions** → **New repository secret**，分别添加：
   - `CLOUDFLARE_API_TOKEN` = 上一步 Token
   - `CLOUDFLARE_ACCOUNT_ID` = Account ID
   - `WORKER_PASSWORD` = 与客户端一致的密码（建议设置）

4. **手动触发一次部署（可选）**  
   **Actions** → **Deploy Worker** → **Run workflow**

本地也可手动部署：`npx wrangler login` 后执行 `npm run deploy`。

### 全栈部署（Worker + 客户端）

```powershell
.\scripts\deploy.ps1
```

同时部署 Worker、构建/复制 `gs.exe`、生成 `client/config.yaml`。详见下方「一键部署（全栈）」。

## 一键部署（全栈）

克隆仓库后，一条命令完成 Worker 部署 + 客户端构建 + 配置生成：

```powershell
git clone https://github.com/gsvps/GSWSS.git
cd GSWSS
.\scripts\deploy.ps1
```

脚本会自动：

1. 检查 Node.js / Go / wrangler 依赖
2. 部署 Cloudflare Worker 并设置 `PASSWORD` Secret
3. 构建 `gs.exe`（或从 `dist/` / GitHub Release 获取）
4. 生成 `client/config.yaml`

常用参数：

```powershell
# 部署完成后立即启动
.\scripts\deploy.ps1 -Start

# 跳过 Worker 部署（已有 Worker 时）
.\scripts\deploy.ps1 -SkipWorker

# 跳过构建，使用仓库内预编译 gs.exe
.\scripts\deploy.ps1 -SkipBuild

# 指定密码（非交互）
.\scripts\deploy.ps1 -Password "your-secret-password"
```

## 下载预编译客户端

无需安装 Go，直接下载 `gs.exe`：

| 来源 | 链接 |
|------|------|
| 仓库内 | [`dist/windows-amd64/gs.exe`](dist/windows-amd64/gs.exe) |
| GitHub Release | [Latest Release](https://github.com/gsvps/GSWSS/releases/latest) |

**Windows 托盘模式：** 双击 `gs.exe` 或在资源管理器运行，图标出现在右下角系统托盘。

| 托盘操作 | 说明 |
|----------|------|
| **启动代理** | 按已保存配置启动 SOCKS5 / HTTP |
| **停止代理** | 停止本地代理 |
| **参数设置...** | 打开参数框，填写 Worker 地址、密码等，可保存或连接测试 |
| **连接测试** | 测试 Worker 连通性与密码认证 |
| **退出** | 关闭托盘程序 |

配置文件默认路径：`%APPDATA%\gs-protocol\config.yaml`

```powershell
# 从 Release 下载
Invoke-WebRequest -Uri "https://github.com/gsvps/GSWSS/releases/latest/download/gs.exe" -OutFile gs.exe
.\gs.exe          # 托盘模式（Windows 默认）
.\gs.exe start    # CLI 模式
```

## 快速开始

### 1. 克隆仓库

```powershell
git clone https://github.com/gsvps/GSWSS.git
cd GSWSS
```

### 2. 部署 Cloudflare Worker

推荐使用 [Worker 一键部署](#worker-一键部署)（浏览器按钮或 `deploy-worker.ps1`）。

手动部署：

```powershell
npm install
npx wrangler login
# 编辑 wrangler.toml [vars] 中的 PASSWORD 与 WEBSOCKET_PATH
npm run deploy:cloudflare
```

部署成功后，记录 Worker URL，WebSocket 端点为：

```
https://<your-worker>.workers.dev/ws
```

> **说明：** `PASSWORD` 与 `WEBSOCKET_PATH` 在根目录 `wrangler.toml` 的 `[vars]` 中配置。详见 [Cloudflare TCP Sockets 文档](https://developers.cloudflare.com/workers/runtime-apis/tcp-sockets/)。

### 3. 构建 Windows 客户端

**方式 A — 使用构建脚本：**

```powershell
cd ..
.\scripts\build-windows.ps1
```

**方式 B — 手动构建：**

```powershell
cd client
go mod tidy
go build -ldflags "-s -w" -o gs.exe ./cmd/gs
```

若 Go 模块下载较慢，可设置国内代理：

```powershell
$env:GOPROXY = "https://goproxy.cn,direct"
go mod tidy
```

### 4. 配置客户端

```powershell
copy ..\examples\config.yaml config.yaml
notepad config.yaml
```

配置示例：

```yaml
# server 路径须与 wrangler.toml [vars] WEBSOCKET_PATH 一致
server: https://your-worker.workers.dev/ws
# 须与 wrangler.toml [vars] PASSWORD 一致
password: your-secret-password
local_socks: 127.0.0.1:1080
local_http: 127.0.0.1:7890
tls: true
heartbeat: 30
log_level: info
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `server` | Worker WebSocket 地址（必填，路径须与 Worker `WEBSOCKET_PATH` 一致） | — |
| `password` | 与 `wrangler.toml` `[vars]` 中 `PASSWORD` 一致（必填） | — |
| `local_socks` | 本地 SOCKS5 监听地址 | `127.0.0.1:1080` |
| `local_http` | 本地 HTTP 代理监听地址 | `127.0.0.1:7890` |
| `tls` | 是否使用 WSS | `true` |
| `heartbeat` | 心跳间隔（秒） | `30` |
| `log_level` | 日志级别：`debug` / `info` / `warn` / `error` | `info` |

> **安全提示：** 请勿将含真实密码的 `config.yaml` 提交到 Git。

### 5. 启动与验证

```powershell
# 启动代理
.\gs.exe start -c config.yaml

# 查看运行状态
.\gs.exe status

# 查看版本
.\gs.exe version

# SOCKS5 连通性测试
curl.exe -x socks5://127.0.0.1:1080 https://example.com

# HTTP 代理测试
curl.exe -x http://127.0.0.1:7890 https://example.com
```

启动后本地代理地址：

- **SOCKS5：** `127.0.0.1:1080`
- **HTTP Proxy：** `127.0.0.1:7890`

浏览器可将 SOCKS5 或 HTTP 代理指向上述地址。

## CLI 命令

| 命令 | 说明 |
|------|------|
| `gs` / `gs tray` | 启动系统托盘（Windows 默认） |
| `gs start [-c config.yaml]` | CLI 启动 SOCKS5 / HTTP 代理 |
| `gs status` | 查看客户端是否在运行 |
| `gs version` | 显示版本与构建信息 |

## GSP1 协议概览

| 字段 | 值 |
|------|-----|
| Magic | `GSP1`（`0x47535031`） |
| Version | `1` |
| 传输层 | WebSocket Binary Frame |
| 加密 | TLS（WSS） |
| 认证 | Password（CONNECT 帧携带） |

**帧结构：**

```
Magic (4) + Version (1) + Type (1) + Flags (2) + Length (4) + Payload
```

**帧类型：**

| Type | 名称 | 说明 |
|------|------|------|
| 1 | CONNECT | 建立中继，携带目标地址与密码 |
| 2 | DATA | 二进制数据 |
| 3 | PING | 心跳请求 |
| 4 | PONG | 心跳响应 |
| 5 | CLOSE | 关闭连接 |
| 6 | ERROR | 错误（含错误码与消息） |

**错误码：**

| 码 | 含义 |
|----|------|
| 1001 | 认证失败 |
| 1002 | 目标地址无效 |
| 1003 | 连接目标失败 |
| 1004 | 速率限制 |
| 1005 | 无效帧 |
| 1006 | 内部错误 |

协议层、传输层、代理层完全解耦，可用任意语言实现兼容客户端。

## Worker 开发

```powershell
npm install
npm run dev        # 本地开发（根目录 wrangler.toml）
npm run typecheck
npm run deploy     # 部署到 Cloudflare
```

Worker 路由：

| 路径 | 说明 |
|------|------|
| `GET /` | 健康检查（显示当前 `WEBSOCKET_PATH`） |
| `GET {WEBSOCKET_PATH}` | WebSocket 升级与 GSP1 中继（默认 `/ws`） |

## 路线图

- **V1（当前）** — SOCKS5、HTTP Proxy、Worker、密码认证
- **V2** — Mux 多路复用、GUI、D1 管理后台
- **V3** — Android、macOS、Linux 客户端
- **V4** — UDP、QUIC、AI Gateway

详细设计见 [product.md](product.md)。

## 安全说明

- **无私有节点：** 所有 Worker 由用户自行部署与管控
- **密码不入日志：** 客户端与 Worker 均不会记录认证信息
- **默认 TLS：** 客户端默认启用 WSS
- **速率限制：** Worker 按 IP 限制连接频率，防止暴力破解
- **地址校验：** 禁止 relay 到内网/本地地址及高风险端口
- **无调试后门：** 不存在绕过认证的隐藏通道

## 常见问题

**Q: 连接 Worker 失败？**

确认 `server` 地址路径与 Worker `WEBSOCKET_PATH` 一致（默认 `/ws`），且 Worker 已成功部署。

**Q: 认证失败？**

确认客户端 `password` 与 `wrangler.toml` `[vars]` 中 `PASSWORD` 完全一致。

**Q: Go 依赖下载超时？**

设置 `$env:GOPROXY = "https://goproxy.cn,direct"` 后重试 `go mod tidy`。

**Q: 连接测试成功，但浏览器仍无法访问 Google？**

「连接测试」只验证 Worker 可达且密码正确，**不会自动开启本地代理**。请按顺序操作：

1. 托盘右键 → **启动代理**（提示「运行中」）
2. 浏览器配置代理（二选一）：
   - **推荐 HTTP 代理：** `127.0.0.1:7890`
   - **SOCKS5：** `127.0.0.1:1080`（须启用**远程 DNS**，Firefox 勾选「代理 DNS」；Chrome 系统代理对 SOCKS5 支持较差，建议用 HTTP 或 SwitchyOmega）
3. 命令行测试：

```powershell
# HTTP 代理（推荐）
curl.exe -x http://127.0.0.1:7890 https://www.google.com

# SOCKS5 远程 DNS（注意是 socks5-hostname / socks5h）
curl.exe --socks5-hostname 127.0.0.1:1080 https://www.google.com
```

**Q: SOCKS5 可用但浏览器无法访问？**

确认浏览器代理类型与端口匹配（SOCKS5 → `1080`，HTTP → `7890`）。

## 开源协议

[MIT License](LICENSE)

## 相关链接

- 仓库：[github.com/gsvps/GSWSS](https://github.com/gsvps/GSWSS)
- Cloudflare Workers：[developers.cloudflare.com/workers](https://developers.cloudflare.com/workers/)
- TCP Sockets API：[developers.cloudflare.com/workers/runtime-apis/tcp-sockets](https://developers.cloudflare.com/workers/runtime-apis/tcp-sockets/)
