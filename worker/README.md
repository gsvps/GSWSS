# GSWSS Worker

Cloudflare Worker 端，负责 WebSocket 接入、密码认证与 TCP 中继。

## 一键部署

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/gsvps/GSWSS/tree/main)

> 一键部署读取**仓库根目录**的 `wrangler.toml` 和 `package.json`（与 CloudDesk / CloudDisk 相同模式）。

部署时在配置页：

1. 填写 **Worker 名称**（仓库未预填，自行指定）
2. 填写 **PASSWORD** Secret（客户端认证密码）
3. 点击 Deploy

WebSocket 端点：`https://<your-worker>.workers.dev/ws`

## 手动部署

```bash
# 仓库根目录
npm install
npx wrangler login
npx wrangler secret put PASSWORD
npm run deploy:cloudflare
```

## 本地开发

```bash
# 方式 A：根目录（与生产一致）
cp .dev.vars.example .dev.vars
npm install
npm run dev

# 方式 B：worker 子目录
cd worker
cp .dev.vars.example .dev.vars
npm install
npm run dev
```

## 配置文件

| 文件 | 用途 |
|------|------|
| `/wrangler.toml` | 生产 / 一键部署 |
| `/package.json` | 部署脚本与 Cloudflare 绑定说明 |
| `/worker/wrangler.toml` | 本地开发 |

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
