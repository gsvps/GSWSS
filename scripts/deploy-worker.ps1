# 一键部署 Cloudflare Worker
# 用法: .\scripts\deploy-worker.ps1 [-Password "your-password"] [-StartLogin]

param(
    [string]$Password = "",
    [switch]$StartLogin
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$WorkerDir = Join-Path $Root "worker"
$DeployInfo = Join-Path $WorkerDir "deploy-info.txt"

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

# --- 检查依赖 ---
Write-Step "检查依赖"
if (-not (Test-Command "node")) {
    Write-Host "缺少 Node.js，请安装 18+: https://nodejs.org/" -ForegroundColor Red
    exit 1
}
if (-not (Test-Command "npm")) {
    Write-Host "缺少 npm，请安装 Node.js: https://nodejs.org/" -ForegroundColor Red
    exit 1
}
Write-Host "  node  $(node --version)"
Write-Host "  npm   $(npm --version)"

Push-Location $WorkerDir
try {
    # --- 可选：Wrangler 登录 ---
    if ($StartLogin) {
        Write-Step "登录 Cloudflare (wrangler login)"
        npx wrangler login
    }

    # --- 安装依赖 ---
    Write-Step "安装 npm 依赖"
    npm install

    # --- 设置密码 ---
    if (-not $Password) {
        Write-Host ""
        Write-Host "请设置 Worker 认证密码 (PASSWORD Secret)" -ForegroundColor Yellow
        Write-Host "客户端 config.yaml 中的 password 须与此一致" -ForegroundColor Yellow
        $secure = Read-Host "请输入 PASSWORD" -AsSecureString
        $Password = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
            [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
        )
        if (-not $Password) {
            Write-Host "密码不能为空" -ForegroundColor Red
            exit 1
        }
    }

    Write-Step "设置 PASSWORD Secret"
    $Password | npx wrangler secret put PASSWORD

    # --- 部署 ---
    Write-Step "部署 Worker (wrangler deploy)"
    $deployOutput = npx wrangler deploy 2>&1 | Out-String
    Write-Host $deployOutput

    # --- 解析 Worker URL ---
    $workerUrl = ""
    if ($deployOutput -match "https://[^\s]+\.workers\.dev") {
        $workerUrl = ($Matches[0]).TrimEnd("/")
    }

    if (-not $workerUrl) {
        Write-Host "未能自动识别 Worker URL" -ForegroundColor Yellow
        $workerUrl = Read-Host "请手动输入 Worker URL (例如 https://xxx.workers.dev)"
        $workerUrl = $workerUrl.TrimEnd("/")
    }

    $wsUrl = "$workerUrl/ws"

    # --- 保存部署信息 ---
    $info = @"
GSWSS Worker 部署信息
=====================
部署时间: $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")
Worker URL: $workerUrl
WebSocket:  $wsUrl

客户端 config.yaml 示例:
  server: $wsUrl
  password: <与 PASSWORD Secret 一致>
"@
    Set-Content -Path $DeployInfo -Value $info -Encoding UTF8

    Write-Host @"

  Worker 部署完成!
  ================

  Worker URL:  $workerUrl
  WebSocket:   $wsUrl
  部署信息:    $DeployInfo

  下一步 — 配置客户端:
    server: $wsUrl
    password: (你刚才设置的 PASSWORD)

  启动客户端:
    cd client
    .\gs.exe start -c config.yaml

"@ -ForegroundColor Green

} catch {
    Write-Host "`n部署失败: $_" -ForegroundColor Red
    Write-Host @"

  若尚未登录 Cloudflare，请先运行:
    cd worker
    npx wrangler login

  或使用 -StartLogin 参数:
    .\scripts\deploy-worker.ps1 -StartLogin

"@ -ForegroundColor Yellow
    exit 1
} finally {
    Pop-Location
}
