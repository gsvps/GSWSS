# GSWSS Worker

Cloudflare Worker 端，负责 WebSocket 接入、密码认证与 TCP 中继。

## 环境变量（wrangler.toml [vars]）

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WEBSOCKET_PATH` | WebSocket 入口路径 | `/ws` |
| `PASSWORD` | 客户端认证密码 | `change-me`（部署前请修改） |

## 一键部署

[![Deploy to Cloudflare Workers](https://deploy.workers.cloudflare.com/button)](https://dash.cloudflare.com/?to=/%3Aaccount/workers-and-pages/create/workers/new)

1. **选择一种方法** → **Continue with GitHub**
2. 选择仓库 **`gsvps/GSWSS`**
3. 确认 `[vars]` 中 `WEBSOCKET_PATH` 与 `PASSWORD`
4. Deploy command = `npm run deploy`，根目录留空
5. 点击 **Deploy**

## 手动部署

```bash
# 编辑 wrangler.toml [vars] 后
npm install
npx wrangler login
npm run deploy:cloudflare
```

## 本地开发

```bash
cp .dev.vars.example .dev.vars   # 可选
npm install
npm run dev
```

## 路由

| 路径 | 说明 |
|------|------|
| `GET /` | 健康检查 |
| `GET {WEBSOCKET_PATH}` | WebSocket 中继（默认 `/ws`） |
