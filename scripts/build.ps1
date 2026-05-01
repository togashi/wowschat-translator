param(
    [string]$Output = "dist/wowschat-translator-windows-amd64.exe",
    [string]$GoArch = "amd64"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$rootDir = Resolve-Path (Join-Path $scriptDir "..")
Set-Location $rootDir

New-Item -ItemType Directory -Path "dist" -Force | Out-Null

$env:GOOS = "windows"
$env:GOARCH = $GoArch

go build -trimpath -ldflags "-s -w" -o $Output .

Write-Host "Build succeeded: $Output"
