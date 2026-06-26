# 一键部署 Cloudflare Worker
# 用法: .\scripts\deploy-worker.ps1 [-Password "your-password"] [-StartLogin]

param(
    [string]$Password = "",
    [switch]$StartLogin
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$DeployInfo = Join-Path $Root "worker\deploy-info.txt"

function Write-Step($msg) {
    Write-Host "`n==> $msg" -ForegroundColor Cyan
}

function Test-Command($name) {
    return $null -ne (Get-Command $name -ErrorAction SilentlyContinue)
}

Write-Host @"

  GSWSS Worker 一键部署
  =====================

"@ -ForegroundColor Green

Write-Step "检查依赖"
if (-not (Test-Command "node")) { Write-Host "缺少 Node.js 18+: https://nodejs.org/" -ForegroundColor Red; exit 1 }
if (-not (Test-Command "npm")) { Write-Host "缺少 npm" -ForegroundColor Red; exit 1 }
Write-Host "  node  $(node --version)"
Write-Host "  npm   $(npm --version)"

Push-Location $Root
try {
    if ($StartLogin) {
        Write-Step "登录 Cloudflare (wrangler login)"
        npx wrangler login
    }

    Write-Step "安装 npm 依赖"
    npm install

    if (-not $Password) {
        Write-Host ""
        Write-Host "请设置 Worker 认证密码 (PASSWORD Secret)" -ForegroundColor Yellow
        $secure = Read-Host "请输入 PASSWORD" -AsSecureString
        $Password = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
            [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
        )
        if (-not $Password) { Write-Host "密码不能为空" -ForegroundColor Red; exit 1 }
    }

    Write-Step "部署 Worker（PASSWORD 写入 Worker 变量）"
    $deployOutput = npm run deploy -- --var "PASSWORD:$Password" 2>&1 | Out-String
    Write-Host $deployOutput

    $wsPath = "/ws"
    $tomlPath = Join-Path $Root "wrangler.toml"
    if (Test-Path $tomlPath) {
        $toml = Get-Content $tomlPath -Raw
        if ($toml -match 'WEBSOCKET_PATH\s*=\s*"([^"]+)"') {
            $wsPath = $Matches[1]
        }
    }

    $workerUrl = ""
    if ($deployOutput -match "https://[^\s]+\.workers\.dev") {
        $workerUrl = ($Matches[0]).TrimEnd("/")
    }
    if (-not $workerUrl) {
        $workerUrl = Read-Host "请手动输入 Worker URL (例如 https://xxx.workers.dev)"
        $workerUrl = $workerUrl.TrimEnd("/")
    }

    $wsUrl = "$workerUrl$wsPath"
    $info = @"
GSWSS Worker 部署信息
=====================
Worker URL: $workerUrl
WebSocket:  $wsUrl

客户端 config.yaml:
  server: $wsUrl
  password: <与 wrangler.toml [vars] PASSWORD 一致>
"@
    Set-Content -Path $DeployInfo -Value $info -Encoding UTF8

    Write-Host @"

  Worker 部署完成!
  Worker URL:  $workerUrl
  WebSocket:   $wsUrl

"@ -ForegroundColor Green

} catch {
    Write-Host "`n部署失败: $_" -ForegroundColor Red
    Write-Host "首次使用请运行: .\scripts\deploy-worker.ps1 -StartLogin" -ForegroundColor Yellow
    exit 1
} finally {
    Pop-Location
}
