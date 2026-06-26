# GSWSS — GS Protocol

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/gsvps/GSWSS/tree/main/worker)

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
├── worker/          # Cloudflare Worker（TypeScript + Hono）
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

部署 Worker 有三种方式，任选其一：

### 方式 A — 浏览器一键部署（推荐，无需本地环境）

点击 README 顶部的 **Deploy to Cloudflare Workers** 按钮，或访问：

```
https://deploy.workers.cloudflare.com/?url=https://github.com/gsvps/GSWSS/tree/main/worker
```

步骤：

1. 登录 Cloudflare 账号
2. 设置 Worker 名称
3. 填写 **PASSWORD** Secret（认证密码，客户端须一致）
4. 点击 Deploy

部署完成后，WebSocket 端点为：

```
https://<your-worker>.workers.dev/ws
```

> 详见 [worker/README.md](worker/README.md) 与 [Cloudflare 部署按钮文档](https://developers.cloudflare.com/workers/platform/deploy-buttons/)。

### 方式 B — 脚本一键部署

**Windows：**

```powershell
git clone https://github.com/gsvps/GSWSS.git
cd GSWSS
.\scripts\deploy-worker.ps1              # 交互式部署
.\scripts\deploy-worker.ps1 -StartLogin  # 首次使用，先登录 Cloudflare
.\scripts\deploy-worker.ps1 -Password "your-secret-password"
```

**Linux / macOS：**

```bash
git clone https://github.com/gsvps/GSWSS.git
cd GSWSS
chmod +x scripts/deploy-worker.sh
./scripts/deploy-worker.sh
./scripts/deploy-worker.sh -l                    # 首次登录
./scripts/deploy-worker.sh -p "your-secret-password"
```

脚本会自动：安装依赖 → 设置 `PASSWORD` Secret → `wrangler deploy` → 输出 WebSocket 地址。

### 方式 C — 全栈一键部署（Worker + 客户端 + 配置）

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

```powershell
# 从 Release 下载
Invoke-WebRequest -Uri "https://github.com/gsvps/GSWSS/releases/latest/download/gs.exe" -OutFile gs.exe
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
cd worker
npm install
npx wrangler secret put PASSWORD
npm run deploy
```

部署成功后，记录 Worker URL，WebSocket 端点为：

```
https://<your-worker>.workers.dev/ws
```

> **说明：** `PASSWORD` 通过 Wrangler Secret 注入，不会写入代码或日志。详见 [Cloudflare TCP Sockets 文档](https://developers.cloudflare.com/workers/runtime-apis/tcp-sockets/)。

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
server: https://your-worker.workers.dev/ws
password: your-secret-password
local_socks: 127.0.0.1:1080
local_http: 127.0.0.1:7890
tls: true
heartbeat: 30
log_level: info
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `server` | Worker WebSocket 地址（必填） | — |
| `password` | 与 Worker Secret 一致的密码（必填） | — |
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
| `gs start [-c config.yaml]` | 启动 SOCKS5 / HTTP 代理 |
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
cd worker
npm run dev      # 本地开发
npm run typecheck
npm run deploy   # 部署到 Cloudflare
```

Worker 路由：

| 路径 | 说明 |
|------|------|
| `GET /` | 健康检查 |
| `GET /ws` | WebSocket 升级与 GSP1 中继 |

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

确认 `server` 地址包含 `/ws` 路径，且 Worker 已成功部署。检查 Cloudflare 控制台中 Worker 的运行状态。

**Q: 认证失败？**

确认客户端 `password` 与 `wrangler secret put PASSWORD` 设置的值完全一致。

**Q: Go 依赖下载超时？**

设置 `$env:GOPROXY = "https://goproxy.cn,direct"` 后重试 `go mod tidy`。

**Q: SOCKS5 可用但浏览器无法访问？**

确认浏览器代理类型与端口匹配（SOCKS5 → `1080`，HTTP → `7890`）。

## 开源协议

[MIT License](LICENSE)

## 相关链接

- 仓库：[github.com/gsvps/GSWSS](https://github.com/gsvps/GSWSS)
- Cloudflare Workers：[developers.cloudflare.com/workers](https://developers.cloudflare.com/workers/)
- TCP Sockets API：[developers.cloudflare.com/workers/runtime-apis/tcp-sockets](https://developers.cloudflare.com/workers/runtime-apis/tcp-sockets/)
