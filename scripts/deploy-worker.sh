#!/usr/bin/env bash
# 一键部署 Cloudflare Worker (Linux / macOS)
# 用法: ./scripts/deploy-worker.sh [-p password] [-l]

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORKER_DIR="$ROOT/worker"
DEPLOY_INFO="$WORKER_DIR/deploy-info.txt"

PASSWORD=""
START_LOGIN=false

while getopts "p:l" opt; do
  case $opt in
    p) PASSWORD="$OPTARG" ;;
    l) START_LOGIN=true ;;
    *) echo "用法: $0 [-p password] [-l]"; exit 1 ;;
  esac
done

echo ""
echo "  GSWSS Worker 一键部署"
echo "  ====================="
echo ""

# 检查依赖
if ! command -v node >/dev/null 2>&1; then
  echo "缺少 Node.js，请安装 18+: https://nodejs.org/"
  exit 1
fi
if ! command -v npm >/dev/null 2>&1; then
  echo "缺少 npm"
  exit 1
fi

echo "  node  $(node --version)"
echo "  npm   $(npm --version)"

cd "$WORKER_DIR"

if [ "$START_LOGIN" = true ]; then
  echo ""
  echo "==> 登录 Cloudflare (wrangler login)"
  npx wrangler login
fi

echo ""
echo "==> 安装 npm 依赖"
npm install

if [ -z "$PASSWORD" ]; then
  echo ""
  echo "请设置 Worker 认证密码 (PASSWORD Secret)"
  read -rsp "请输入 PASSWORD: " PASSWORD
  echo ""
  if [ -z "$PASSWORD" ]; then
    echo "密码不能为空"
    exit 1
  fi
fi

echo ""
echo "==> 设置 PASSWORD Secret"
printf '%s' "$PASSWORD" | npx wrangler secret put PASSWORD

echo ""
echo "==> 部署 Worker (wrangler deploy)"
DEPLOY_OUTPUT=$(npx wrangler deploy 2>&1 | tee /dev/stderr)

WORKER_URL=$(echo "$DEPLOY_OUTPUT" | grep -oE 'https://[^[:space:]]+\.workers\.dev' | head -1 | sed 's/\/$//')

if [ -z "$WORKER_URL" ]; then
  echo ""
  read -rp "请手动输入 Worker URL (例如 https://xxx.workers.dev): " WORKER_URL
  WORKER_URL="${WORKER_URL%/}"
fi

WS_URL="$WORKER_URL/ws"

cat > "$DEPLOY_INFO" <<EOF
GSWSS Worker 部署信息
=====================
Worker URL: $WORKER_URL
WebSocket:  $WS_URL

客户端 config.yaml 示例:
  server: $WS_URL
  password: <与 PASSWORD Secret 一致>
EOF

echo ""
echo "  Worker 部署完成!"
echo "  ================"
echo ""
echo "  Worker URL:  $WORKER_URL"
echo "  WebSocket:   $WS_URL"
echo "  部署信息:    $DEPLOY_INFO"
echo ""
echo "  客户端 config.yaml:"
echo "    server: $WS_URL"
echo "    password: (你设置的 PASSWORD)"
echo ""
