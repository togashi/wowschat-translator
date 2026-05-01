@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%..") do set "ROOT_DIR=%%~fI"

pushd "%ROOT_DIR%" >nul
if errorlevel 1 (
  echo Failed to enter repository root.
  exit /b 1
)

if not exist "dist" mkdir "dist"

set "GOOS=windows"
set "GOARCH=amd64"
go build -trimpath -ldflags "-s -w" -o "dist\wowschat-translator-windows-amd64.exe" .
if errorlevel 1 (
  echo Build failed.
  popd >nul
  exit /b 1
)

echo Build succeeded: dist\wowschat-translator-windows-amd64.exe
popd >nul
exit /b 0
