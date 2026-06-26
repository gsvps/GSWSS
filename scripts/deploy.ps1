# 一键部署脚本（Windows）
# 用法: .\scripts\deploy.ps1
#
# 功能:
#   1. 检查依赖 (Node.js / npm / Go / wrangler)
#   2. 部署 Cloudflare Worker
#   3. 构建或复制 gs.exe
#   4. 生成 config.yaml
#   5. 可选: 立即启动客户端

param(
    [string]$Password = "",
    [string]$WorkerName = "gs-protocol-worker",
    [switch]$SkipWorker,
    [switch]$SkipBuild,
    [switch]$Start
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$WorkerDir = Join-Path $Root "worker"
$ClientDir = Join-Path $Root "client"
$DistExe = Join-Path $Root "dist\windows-amd64\gs.exe"
$ConfigPath = Join-Path $ClientDir "config.yaml"

function Write-Step($msg) {
    Write-Host "`n==> $msg" -ForegroundColor Cyan
}

function Test-Command($name) {
    return $null -ne (Get-Command $name -ErrorAction SilentlyContinue)
}

function Ensure-Command($name, $installHint) {
    if (-not (Test-Command $name)) {
        Write-Host "缺少依赖: $name" -ForegroundColor Red
        Write-Host $installHint
        exit 1
    }
}

Write-Host @"

  GSWSS 一键部署
  ==============

"@ -ForegroundColor Green

# --- 检查依赖 ---
Write-Step "检查依赖"
Ensure-Command "node" "请安装 Node.js 18+: https://nodejs.org/"
Ensure-Command "npm"  "请安装 Node.js (含 npm): https://nodejs.org/"
if (-not $SkipBuild) {
    Ensure-Command "go" "请安装 Go 1.24+: https://go.dev/dl/"
}

$wranglerCmd = "npx"
Write-Host "  node  $(node --version)"
Write-Host "  npm   $(npm --version)"
if (-not $SkipBuild) {
    Write-Host "  go    $(go version)"
}

# --- 获取密码 ---
if (-not $Password) {
    $secure = Read-Host "请输入 Worker 密码 (PASSWORD)" -AsSecureString
    $Password = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
        [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    )
    if (-not $Password) {
        Write-Host "密码不能为空" -ForegroundColor Red
        exit 1
    }
}

$workerUrl = ""

# --- 部署 Worker ---
if (-not $SkipWorker) {
    Write-Step "部署 Cloudflare Worker"
    Push-Location $WorkerDir
    try {
        Write-Host "安装 npm 依赖..."
        npm install --silent

        Write-Host "设置 PASSWORD Secret..."
        $Password | npx wrangler secret put PASSWORD

        Write-Host "部署 Worker..."
        $deployOutput = npx wrangler deploy 2>&1 | Out-String
        Write-Host $deployOutput

        if ($deployOutput -match "https://[^\s]+\.workers\.dev") {
            $workerUrl = ($Matches[0]).TrimEnd("/")
        }
    } finally {
        Pop-Location
    }

    if (-not $workerUrl) {
        $workerUrl = Read-Host "未能自动识别 Worker URL，请手动输入 (例如 https://xxx.workers.dev)"
    }
    $workerUrl = $workerUrl.TrimEnd("/")
    Write-Host "Worker URL: $workerUrl" -ForegroundColor Green
} else {
    $workerUrl = Read-Host "请输入已部署的 Worker URL (例如 https://xxx.workers.dev)"
    $workerUrl = $workerUrl.TrimEnd("/")
}

# --- 准备 gs.exe ---
Write-Step "准备客户端 gs.exe"
$targetExe = Join-Path $ClientDir "gs.exe"

if ($SkipBuild -and (Test-Path $DistExe)) {
    Copy-Item $DistExe $targetExe -Force
    Write-Host "已从 dist 复制: $targetExe"
} elseif ($SkipBuild) {
    Write-Host "未找到 dist\windows-amd64\gs.exe，尝试从 GitHub Release 下载..." -ForegroundColor Yellow
    $releaseUrl = "https://github.com/gsvps/GSWSS/releases/latest/download/gs.exe"
    try {
        Invoke-WebRequest -Uri $releaseUrl -OutFile $targetExe -UseBasicParsing
        Write-Host "已下载: $targetExe"
    } catch {
        Write-Host "下载失败，请手动构建或下载 gs.exe" -ForegroundColor Red
        exit 1
    }
} else {
    Push-Location $ClientDir
    try {
        if (-not $env:GOPROXY) {
            $env:GOPROXY = "https://goproxy.cn,direct"
        }
        go mod tidy
        go build -ldflags "-s -w" -o gs.exe ./cmd/gs
        Write-Host "已构建: $targetExe"

        $distDir = Split-Path $DistExe -Parent
        if (-not (Test-Path $distDir)) {
            New-Item -ItemType Directory -Path $distDir -Force | Out-Null
        }
        Copy-Item $targetExe $DistExe -Force
        Write-Host "已同步到: $DistExe"
    } finally {
        Pop-Location
    }
}

# --- 生成 config.yaml ---
Write-Step "生成配置文件"
$configContent = @"
# GSWSS 客户端配置 (由 deploy.ps1 自动生成)
server: $workerUrl/ws
password: $Password
local_socks: 127.0.0.1:1080
local_http: 127.0.0.1:7890
tls: true
heartbeat: 30
log_level: info
"@
Set-Content -Path $ConfigPath -Value $configContent -Encoding UTF8
Write-Host "已生成: $ConfigPath" -ForegroundColor Green

# --- 完成 ---
Write-Host @"

  部署完成!
  =========

  Worker:  $workerUrl/ws
  客户端:  $targetExe
  配置:    $ConfigPath

  启动命令:
    cd client
    .\gs.exe start -c config.yaml

  代理地址:
    SOCKS5: 127.0.0.1:1080
    HTTP:   127.0.0.1:7890

"@ -ForegroundColor Green

if ($Start) {
    Write-Step "启动客户端"
    Push-Location $ClientDir
    try {
        & ".\gs.exe" start -c config.yaml
    } finally {
        Pop-Location
    }
}
