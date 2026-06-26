# GSWSS Worker

Cloudflare Worker 端，负责 WebSocket 接入、密码认证与 TCP 中继。

## 一键部署

> [`deploy.workers.cloudflare.com`](https://deploy.workers.cloudflare.com/) 会 **fork 新仓库**到你的 GitHub。若已有本仓库，请用 **Dashboard 连接 GitHub**（方式 A）。

### 方式 A — Dashboard 连接 GitHub（推荐）

[![Connect GitHub & Deploy](https://img.shields.io/badge/Deploy-Connect%20GitHub%20Repo-F38020?logo=cloudflare&logoColor=white)](https://dash.cloudflare.com/?to=/%3Aaccount/workers-and-pages/create/workers/new)

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com/?to=/%3Aaccount/workers-and-pages/create/workers/new)
2. 选择 **Import a repository**
3. 首次使用：[安装 Cloudflare GitHub App](https://github.com/apps/cloudflare-workers-and-pages/installations/new)
4. 选择仓库 **`gsvps/GSWSS`**
5. 配置：

| 配置项 | 值 |
|--------|-----|
| Root directory | `worker` |
| Worker 名称 | `gs-protocol-worker` |
| Deploy command | `npm run deploy` |

6. **Save and Deploy**
7. 在 Worker → Settings → Secrets 添加 **`PASSWORD`**

WebSocket 端点：`https://gs-protocol-worker.<subdomain>.workers.dev/ws`

### 方式 B — GitHub Actions

Fork 仓库并配置 Secrets：`CLOUDFLARE_API_TOKEN`、`CLOUDFLARE_ACCOUNT_ID`、`WORKER_PASSWORD`（可选）。推送 `main` 即自动部署。

### 方式 C — 脚本部署（Windows）

```powershell
# 仓库根目录
.\scripts\deploy-worker.ps1
.\scripts\deploy-worker.ps1 -StartLogin
.\scripts\deploy-worker.ps1 -Password "your-secret-password"
```

### 方式 D — 脚本部署（Linux / macOS）

```bash
chmod +x scripts/deploy-worker.sh
./scripts/deploy-worker.sh -l
./scripts/deploy-worker.sh -p "your-secret-password"
```

### 方式 E — 手动部署

```bash
cd worker
npm install
npx wrangler login
npx wrangler secret put PASSWORD
npm run deploy
```

<details>
<summary>备选：模板部署（Fork 新仓库）</summary>

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/gsvps/GSWSS/tree/main/worker)

</details>

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
