#!/usr/bin/env bash
# 一键部署 Cloudflare Worker (Linux / macOS)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PASSWORD=""
START_LOGIN=false

while getopts "p:l" opt; do
  case $opt in
    p) PASSWORD="$OPTARG" ;;
    l) START_LOGIN=true ;;
    *) echo "用法: $0 [-p password] [-l]"; exit 1 ;;
  esac
done

cd "$ROOT"

if [ "$START_LOGIN" = true ]; then
  echo "==> wrangler login"
  npx wrangler login
fi

echo "==> npm install"
npm install

if [ -z "$PASSWORD" ]; then
  read -rsp "请输入 PASSWORD: " PASSWORD
  echo ""
fi

echo "==> npm run deploy (PASSWORD via --var)"
npm run deploy -- --var "PASSWORD:$PASSWORD"

echo "WebSocket 端点: https://<your-worker>.workers.dev/ws"
