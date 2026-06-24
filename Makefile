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

# sing-box release (TUN engine). Override: `make singbox SINGBOX_VERSION=1.11.15`.
# NOTE: the asset name uses the version WITHOUT the leading "v".
SINGBOX_VERSION ?= 1.11.15
SINGBOX_ZIP     ?= sing-box-$(SINGBOX_VERSION)-windows-amd64.zip

# wintun (TUN driver DLL) required by sing-box TUN on Windows. From wintun.net.
WINTUN_VERSION ?= 0.14.1
WINTUN_ZIP     ?= wintun-$(WINTUN_VERSION).zip

# Geo databases. xray reads geoip.dat/geosite.dat; sing-box reads .srs rule-sets.
# Sources: Loyalsoldier rules for xray .dat; SagerNet sing-geoip/sing-geosite for
# the ru .srs rule-sets used by the "Russian sites direct" toggle.
GEOIP_DAT_URL   ?= https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
GEOSITE_DAT_URL ?= https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
SINGBOX_GEOIP_RU_URL   ?= https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-ru.srs
SINGBOX_GEOSITE_RU_URL ?= https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ru.srs

# Release version (used in the zip name and embedded via ldflags for the
# auto-updater). Override: `make release VERSION=v0.2.0`.
VERSION ?= dev

# Module path for -X ldflags targeting the embedded build version.
MODULE := github.com/Alexzxcv/vpn-client-windows

# Build flags: GUI subsystem so no console window pops up; strip debug info; embed
# the version so the in-app updater can compare against the latest GitHub release.
LDFLAGS := -s -w -H windowsgui -X $(MODULE)/internal/buildinfo.Version=$(VERSION)

.PHONY: ui build build-gui run xray singbox wintun geo tidy vet test release dist release-staging installer

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

# Download sing-box.exe (TUN engine) into bin/ from the official GitHub release.
# The asset is a zip with sing-box.exe inside a versioned subfolder.
singbox:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "\
		$ErrorActionPreference='Stop'; \
		$ver='$(SINGBOX_VERSION)'; $zip='$(SINGBOX_ZIP)'; \
		$url=\"https://github.com/SagerNet/sing-box/releases/download/v$$ver/$$zip\"; \
		New-Item -ItemType Directory -Force -Path bin | Out-Null; \
		$tmp=Join-Path $$env:TEMP $$zip; \
		Write-Host \"Downloading $$url\"; \
		Invoke-WebRequest -Uri $$url -OutFile $$tmp; \
		Add-Type -AssemblyName System.IO.Compression.FileSystem; \
		$dest=Join-Path (Get-Location) 'bin'; \
		$archive=[System.IO.Compression.ZipFile]::OpenRead($$tmp); \
		foreach ($$e in $$archive.Entries) { if ($$e.Name -eq 'sing-box.exe') { \
			[System.IO.Compression.ZipFileExtensions]::ExtractToFile($$e, (Join-Path $$dest 'sing-box.exe'), $$true) } }; \
		$archive.Dispose(); Remove-Item $$tmp; \
		Write-Host 'sing-box.exe -> bin/sing-box.exe'"

# Download wintun.dll (amd64) into bin/ from wintun.net. Required for TUN.
wintun:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "\
		$ErrorActionPreference='Stop'; \
		$zip='$(WINTUN_ZIP)'; \
		$url=\"https://www.wintun.net/builds/$$zip\"; \
		New-Item -ItemType Directory -Force -Path bin | Out-Null; \
		$tmp=Join-Path $$env:TEMP $$zip; \
		Write-Host \"Downloading $$url\"; \
		Invoke-WebRequest -Uri $$url -OutFile $$tmp; \
		Add-Type -AssemblyName System.IO.Compression.FileSystem; \
		$dest=Join-Path (Get-Location) 'bin'; \
		$archive=[System.IO.Compression.ZipFile]::OpenRead($$tmp); \
		foreach ($$e in $$archive.Entries) { if ($$e.FullName -eq 'wintun/bin/amd64/wintun.dll') { \
			[System.IO.Compression.ZipFileExtensions]::ExtractToFile($$e, (Join-Path $$dest 'wintun.dll'), $$true) } }; \
		$archive.Dispose(); Remove-Item $$tmp; \
		Write-Host 'wintun.dll -> bin/wintun.dll'"

# Download geo databases into bin/geo/: geoip.dat + geosite.dat (xray) and
# geoip-ru.srs + geosite-ru.srs (sing-box). Used by the "Russian sites direct"
# toggle. xray finds *.dat via XRAY_LOCATION_ASSET / next to xray.exe; sing-box
# loads the .srs by path (internal/singbox.RuleSetPath -> dir/geo).
geo:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "\
		$ErrorActionPreference='Stop'; \
		$dest=Join-Path (Get-Location) 'bin/geo'; \
		New-Item -ItemType Directory -Force -Path $$dest | Out-Null; \
		$files=@{ \
			'geoip.dat'      = '$(GEOIP_DAT_URL)'; \
			'geosite.dat'    = '$(GEOSITE_DAT_URL)'; \
			'geoip-ru.srs'   = '$(SINGBOX_GEOIP_RU_URL)'; \
			'geosite-ru.srs' = '$(SINGBOX_GEOSITE_RU_URL)'; \
		}; \
		foreach ($$k in $$files.Keys) { \
			Write-Host (\"Downloading {0}\" -f $$files[$$k]); \
			Invoke-WebRequest -Uri $$files[$$k] -OutFile (Join-Path $$dest $$k); \
		}; \
		Write-Host 'geo -> bin/geo/'"

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

# release: build UI + GUI binary + fetch xray + sing-box + wintun + geo, then
# package a distributable zip into dist/vpnclient-$(VERSION)-windows-amd64.zip.
# Run on Windows (or a box with PowerShell).
release: ui build-gui xray singbox wintun geo dist

# release-staging: assemble the install payload into dist/stage/ (exe + engines +
# wintun + built UI + geo databases). Shared by `dist` (zip) and `installer`.
release-staging:
	powershell -NoProfile -ExecutionPolicy Bypass -Command "\
		$ErrorActionPreference='Stop'; \
		$stage=Join-Path 'dist' 'stage'; \
		if (Test-Path $$stage) { Remove-Item -Recurse -Force $$stage }; \
		New-Item -ItemType Directory -Force -Path $$stage | Out-Null; \
		Copy-Item 'bin/vpnclient.exe' $$stage -Force; \
		Copy-Item 'bin/xray.exe' $$stage -Force; \
		Copy-Item 'bin/sing-box.exe' $$stage -Force; \
		Copy-Item 'bin/wintun.dll' $$stage -Force; \
		Copy-Item 'docs/RELEASE_README.md' (Join-Path $$stage 'README.md') -Force; \
		New-Item -ItemType Directory -Force -Path (Join-Path $$stage 'ui') | Out-Null; \
		Copy-Item 'frontend/dist/*' (Join-Path $$stage 'ui') -Recurse -Force; \
		if (Test-Path 'bin/geo') { \
			New-Item -ItemType Directory -Force -Path (Join-Path $$stage 'geo') | Out-Null; \
			Copy-Item 'bin/geo/*' (Join-Path $$stage 'geo') -Recurse -Force; \
		}; \
		Write-Host 'staging -> dist/stage'"

# dist: assemble staging then zip it under dist/.
dist: release-staging
	powershell -NoProfile -ExecutionPolicy Bypass -Command "\
		$ErrorActionPreference='Stop'; \
		$ver='$(VERSION)'; \
		$stage=Join-Path 'dist' 'stage'; \
		$zip=Join-Path 'dist' (\"vpnclient-$$ver-windows-amd64.zip\"); \
		if (Test-Path $$zip) { Remove-Item $$zip -Force }; \
		Compress-Archive -Path (Join-Path $$stage '*') -DestinationPath $$zip; \
		Write-Host \"Packaged $$zip\""

# installer: build the single SAPN-VPN-Setup.exe via Inno Setup (iscc must be on
# PATH). Depends on a populated dist/stage (run `make release-staging` or
# `make release` first). Override version: `make installer VERSION=v0.2.0`.
installer:
	iscc installer/vpnclient.iss /DMyAppVersion=$(VERSION)
