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

# Release version (used in the zip name). Override: `make release VERSION=v0.2.0`.
VERSION ?= dev

# Build flags: GUI subsystem so no console window pops up; strip debug info.
LDFLAGS := -s -w -H windowsgui

.PHONY: ui build build-gui run xray tidy vet test release dist

ui:
	cd frontend && npm install && npm run build

build:
	go build -o bin/vpnclient.exe ./cmd/vpnclient

# build-gui: release-style build — no console window, stripped binary.
build-gui:
	go build -ldflags "$(LDFLAGS)" -o bin/vpnclient.exe ./cmd/vpnclient

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

# release: build UI + GUI binary + fetch xray, then package a distributable zip
# into dist/vpnclient-$(VERSION)-windows-amd64.zip containing vpnclient.exe,
# xray.exe and a short README. Run on Windows (or a box with PowerShell).
release: ui build-gui xray dist

# dist: assemble bin/ artefacts (built by `release`) into a zip under dist/.
dist:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "\
		$ErrorActionPreference='Stop'; \
		$ver='$(VERSION)'; \
		$stage=Join-Path 'dist' 'stage'; \
		New-Item -ItemType Directory -Force -Path $$stage | Out-Null; \
		Copy-Item 'bin/vpnclient.exe' $$stage -Force; \
		Copy-Item 'bin/xray.exe' $$stage -Force; \
		Copy-Item 'docs/RELEASE_README.md' (Join-Path $$stage 'README.md') -Force; \
		$zip=Join-Path 'dist' (\"vpnclient-$$ver-windows-amd64.zip\"); \
		if (Test-Path $$zip) { Remove-Item $$zip -Force }; \
		Compress-Archive -Path (Join-Path $$stage '*') -DestinationPath $$zip; \
		Remove-Item -Recurse -Force $$stage; \
		Write-Host \"Packaged $$zip\""
