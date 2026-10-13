#!/bin/sh
# Builds NetzScan.exe (Windows x64) from Linux, macOS or Windows (Git Bash).
set -e
cd "$(dirname "$0")"
go run github.com/akavel/rsrc@v0.10.2 -manifest netzscan.manifest -ico app.ico -arch amd64 -o rsrc_windows_amd64.syso
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o NetzScan.exe .
echo "OK: NetzScan.exe"
