# GSWSS Worker

Cloudflare Worker 端，负责 WebSocket 接入、密码认证与 TCP 中继。

## 一键部署

### 方式 A — 连接 GitHub 仓库（推荐）

连接你 GitHub 上 **已有的 Fork**，不会自动创建新仓库。

1. [Fork GSWSS](https://github.com/gsvps/GSWSS/fork)（若已有可跳过）
2. 打开 [Cloudflare Workers & Pages](https://dash.cloudflare.com/?to=/:account/workers-and-pages)
3. **Create application** → **Import a repository** → **Get started**
4. 连接 GitHub，选择你的 `GSWSS` 仓库
5. 配置：

| 配置项 | 值 |
|--------|-----|
| Root directory | `worker` |
| Build command | （留空） |
| Deploy command | `npx wrangler deploy` |
| Worker name | `gs-protocol-worker` |

6. 添加 Secret：`PASSWORD`
7. **Save and Deploy**

WebSocket 端点：`https://<your-worker>.workers.dev/ws`

> Cloudflare [Deploy 按钮](https://developers.cloudflare.com/workers/platform/deploy-buttons/) 会 Fork 创建新仓库；若你已有 Fork，请用上述 Git 连接方式。

<details>
<summary>备选：Deploy 按钮（自动 Fork 新仓库）</summary>

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/gsvps/GSWSS/tree/main/worker)

</details>

### 方式 B — 脚本部署（Windows）

```powershell
# 仓库根目录
.\scripts\deploy-worker.ps1

# 首次使用，先登录 Cloudflare
.\scripts\deploy-worker.ps1 -StartLogin

# 非交互指定密码
.\scripts\deploy-worker.ps1 -Password "your-secret-password"
```

### 方式 C — 脚本部署（Linux / macOS）

```bash
chmod +x scripts/deploy-worker.sh
./scripts/deploy-worker.sh

# 首次登录
./scripts/deploy-worker.sh -l

# 指定密码
./scripts/deploy-worker.sh -p "your-secret-password"
```

### 方式 D — 手动部署

```bash
cd worker
npm install
npx wrangler login
npx wrangler secret put PASSWORD
npm run deploy
```

## Git 连接构建说明

本仓库为 monorepo，Worker 代码在 `worker/` 子目录。通过 Cloudflare Git 集成部署时，**必须**将 Root directory 设为 `worker`，否则构建会失败。

Worker 名称必须与 `wrangler.toml` 中的 `name = "gs-protocol-worker"` 一致。

## 本地开发

```bash
cd worker
cp .dev.vars.example .dev.vars   # 填入 PASSWORD
npm install
npm run dev
```

## 路由

| 路径 | 说明 |
|------|------|
| `GET /` | 健康检查 |
| `GET /ws` | WebSocket 升级与 GSP1 中继 |

## 环境变量

| 名称 | 类型 | 说明 |
|------|------|------|
| `PASSWORD` | Secret | 客户端认证密码，必填 |

## 部署后配置客户端

```yaml
server: https://your-worker.workers.dev/ws
password: your-secret-password
```
