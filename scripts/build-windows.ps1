# Windows 构建脚本
# 用法: .\scripts\build-windows.ps1

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$ClientDir = Join-Path $Root "client"

Push-Location $ClientDir
try {
    Write-Host "==> go mod tidy"
    go mod tidy

    Write-Host "==> go test ./..."
    go test ./...

    Write-Host "==> building gs.exe"
    go build -ldflags "-s -w" -o gs.exe ./cmd/gs

    Write-Host "==> done: $ClientDir\gs.exe"
} finally {
    Pop-Location
}
