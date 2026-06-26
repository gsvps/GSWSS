# Windows 构建脚本
# 用法: .\scripts\build-windows.ps1
# 输出: dist/windows-amd64/gs.exe

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$ClientDir = Join-Path $Root "client"
$DistDir = Join-Path $Root "dist\windows-amd64"
$DistExe = Join-Path $DistDir "gs.exe"

if (-not (Test-Path $DistDir)) {
    New-Item -ItemType Directory -Path $DistDir -Force | Out-Null
}

Push-Location $ClientDir
try {
    if (-not $env:GOPROXY) {
        $env:GOPROXY = "https://goproxy.cn,direct"
    }

    Write-Host "==> go mod tidy"
    go mod tidy

    Write-Host "==> go test ./..."
    go test ./...

    Write-Host "==> building gs.exe"
    go build -ldflags "-s -w" -o $DistExe ./cmd/gs

    Copy-Item $DistExe (Join-Path $ClientDir "gs.exe") -Force

    Write-Host "==> done:"
    Write-Host "    $DistExe"
    Write-Host "    $ClientDir\gs.exe"
} finally {
    Pop-Location
}
