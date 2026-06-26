# GSWSS Worker

Cloudflare Worker 端，负责 WebSocket 接入、密码认证与 TCP 中继。

## 一键部署

### 方式 A — 浏览器部署（无需本地环境）

点击下方按钮，登录 Cloudflare 账号即可完成部署：

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/gsvps/GSWSS/tree/main/worker)

部署时在控制台：

1. 设置 Worker 名称（可自定义）
2. 填写 **PASSWORD** Secret（认证密码，客户端须一致）
3. 点击 Deploy

部署成功后，WebSocket 端点为：

```
https://<your-worker>.workers.dev/ws
```

> 参考：[Cloudflare Deploy to Cloudflare buttons 文档](https://developers.cloudflare.com/workers/platform/deploy-buttons/)

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
