# Makefile — vpn-client-windows
#
# On Windows install make via `choco install make`. The `xray` target uses
# PowerShell so it works cross-platform on a dev box with PowerShell available.

# Xray release to download. Override: `make xray XRAY_VERSION=v1.8.24`.
XRAY_VERSION ?= v1.8.24

# Target arch ZIP. Go is built for win/386 here, but the official Xray Windows
# 32-bit asset is "Xray-windows-32.zip". The 64-bit one is "Xray-windows-64.zip".
# A 32-bit (386) Go process can launch a 64-bit xray.exe fine, so 64 is the
# safe default on modern Windows. Switch with `make xray XRAY_ZIP=Xray-windows-32.zip`.
XRAY_ZIP ?= Xray-windows-64.zip

.PHONY: ui build run xray tidy vet test

ui:
	cd frontend && npm install && npm run build

build:
	go build -o bin/vpnclient.exe ./cmd/vpnclient

run:
	go run ./cmd/vpnclient

# Download xray.exe into bin/ from the official GitHub release.
# Uses PowerShell (available on Windows and on dev boxes with pwsh installed).
xray:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "\
		$ErrorActionPreference='Stop'; \
		$ver='$(XRAY_VERSION)'; $zip='$(XRAY_ZIP)'; \
		$url=\"https://github.com/XTLS/Xray-core/releases/download/$$ver/$$zip\"; \
		New-Item -ItemType Directory -Force -Path bin | Out-Null; \
		$tmp=Join-Path $$env:TEMP $$zip; \
		Write-Host \"Downloading $$url\"; \
		Invoke-WebRequest -Uri $$url -OutFile $$tmp; \
		Add-Type -AssemblyName System.IO.Compression.FileSystem; \
		$dest=Join-Path (Get-Location) 'bin'; \
		$archive=[System.IO.Compression.ZipFile]::OpenRead($$tmp); \
		foreach ($$e in $$archive.Entries) { if ($$e.Name -eq 'xray.exe') { \
			[System.IO.Compression.ZipFileExtensions]::ExtractToFile($$e, (Join-Path $$dest 'xray.exe'), $$true) } }; \
		$archive.Dispose(); Remove-Item $$tmp; \
		Write-Host 'xray.exe -> bin/xray.exe'"

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...
