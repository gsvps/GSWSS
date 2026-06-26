# 仅部署 Cloudflare Worker
# 用法: .\scripts\deploy-worker.ps1 [-Password "your-password"]

param(
    [string]$Password = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$WorkerDir = Join-Path $Root "worker"

if (-not $Password) {
    $secure = Read-Host "请输入 Worker 密码 (PASSWORD)" -AsSecureString
    $Password = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
        [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    )
}

Push-Location $WorkerDir
try {
    Write-Host "==> npm install" -ForegroundColor Cyan
    npm install

    Write-Host "==> wrangler secret put PASSWORD" -ForegroundColor Cyan
    $Password | npx wrangler secret put PASSWORD

    Write-Host "==> wrangler deploy" -ForegroundColor Cyan
    npx wrangler deploy
} finally {
    Pop-Location
}

Write-Host "`nWorker 部署完成。WebSocket 端点: https://<your-worker>.workers.dev/ws" -ForegroundColor Green
