# Windows 构建脚本
# 用法: .\scripts\build-windows.ps1
# 输出: dist/windows-amd64/gs.exe

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$ClientDir = Join-Path $Root "client"
$DistDir = Join-Path $Root "dist\windows-amd64"
$DistExe = Join-Path $DistDir "gs.exe"
$Manifest = Join-Path $ClientDir "cmd\gs\app.manifest"
$Syso = Join-Path $ClientDir "cmd\gs\rsrc.syso"

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

    Write-Host "==> embed Windows manifest (Common Controls v6)"
    go install github.com/akavel/rsrc@v0.10.2
    $rsrc = Join-Path (go env GOPATH) "bin\rsrc.exe"
    if (-not (Test-Path $rsrc)) { $rsrc = "rsrc" }
    & $rsrc -manifest $Manifest -o $Syso -arch amd64

    Write-Host "==> go test ./..."
    go test ./...

    Write-Host "==> building gs.exe (tray GUI)"
    go build -ldflags "-s -w -H windowsgui" -o $DistExe ./cmd/gs

    Copy-Item $DistExe (Join-Path $ClientDir "gs.exe") -Force

    Write-Host "==> done:"
    Write-Host "    $DistExe"
    Write-Host "    $ClientDir\gs.exe"
} finally {
    Pop-Location
}
