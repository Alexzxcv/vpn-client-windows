# CLAUDE.md — vpn-client-windows

Windows-клиент VPN-сервиса **для тестирования** подключения. **Public** репозиторий
(доверие к VPN: код открыт, видно что нет телеметрии/бэкдоров) — поэтому в коде НЕ должно
быть секретов, всё тянется с бэкенда в рантайме.

## Что это
Десктоп-приложение: **Go-ядро** + **React-UI**. Ядро управляет `xray-core` (субпроцесс),
ходит в бэкенд (`vpn_service`) за конфигом VLESS Reality, поднимает локальный control-API и
окно WebView2 с React-интерфейсом.

Первый режим — **SOCKS-прокси** (xray поднимает локальный SOCKS, трафик через VLESS
Reality; без драйверов и прав админа). Полный TUN (WinTUN) — позже.

## Стек
- **Ядро:** Go (последняя стабильная), целевая ОС Windows.
  - `github.com/jchv/go-webview2` — окно WebView2 (есть в Windows 11).
  - `chi/v5` — локальный control-сервер. `google/uuid`, `golang.org/x/sys/windows/registry`.
  - Управление `xray-core` как субпроцессом (генерация config.json, start/stop, health).
- **UI:** React 18 + TypeScript + **rsbuild** + **MobX** (`mobx-react-lite`, `observer`) +
  **react-router-dom v6**. Сборка в `frontend/dist`, ядро отдаёт её как статику.

## Структура
```
cmd/vpnclient/      main: single-instance + control-сервер + трей + окно;
                    tray.go (меню трея), ui_windows.go (WebView2 window manager),
                    assets/*.ico (иконки трея, go:embed)
internal/
  control/          локальный HTTP control-API (см. docs/CONTROL_API.md)
  backend/          клиент к API vpn_service (auth + /vpn/config), refresh на 401
  xray/             менеджер xray-core: генерация конфига, процесс, health,
                    SetExitHandler (детект краша процесса для авто-реконнекта)
  sysproxy/         системный прокси Windows: Set/Clear + ClearIfOurs
                    (crash-safe снятие нашего прокси по портам 10800/10801)
  singleinstance/   именованный мьютекс — запрет второго экземпляра
  device/           стабильный device_id (MachineGuid)
  app/              склейка: состояние подключения, авто-реконнект с backoff,
                    crash-safe снятие прокси (CleanupStaleProxy/ForceClearProxy)
frontend/           React-UI (rsbuild); билд -> frontend/dist
docs/CONTROL_API.md контракт Go-ядро <-> React-UI (источник истины)
docs/RELEASE_README.md README, кладётся в релизный zip
.github/workflows/release.yml  релиз по тегу (сборка + xray + zip + GitHub Release)
bin/                xray.exe кладётся сюда (см. Makefile)
```

## Десктоп-поведение (как у настоящего VPN-клиента)
- **Трей** (`fyne.io/systray`): Connect/Disconnect/Status/Open/Quit; закрытие окна
  сворачивает в трей, процесс живёт; Quit — полный выход со снятием прокси.
- **Single-instance**: именованный мьютекс `Global\vpn-client-windows-singleton`.
- **Crash-safe прокси**: снятие в shutdown/сигналах(SIGINT/SIGTERM)/панике, и на
  старте `CleanupStaleProxy` снимает наш прокси, оставшийся от прошлого запуска.
- **Авто-реконнект**: при падении xray в connected — до 5 попыток с backoff.
- TUN-режим (полный туннель) — отдельная фаза; в `/api/status` есть `mode`
  (`proxy`|`tun`) с текущим значением `proxy`.

## Правила
- Контракт control-API и бэкенда — в `docs/CONTROL_API.md`. Держи код в синхроне с ним.
- Control-сервер слушает ТОЛЬКО `127.0.0.1`, с проверкой Origin и session-token.
- Никаких секретов в репозитории. Токены бэкенда хранить локально у пользователя
  (память/защищённое хранилище ОС), не коммитить, не логировать. НЕ логировать vless uuid,
  reality-ключи, готовую VLESS-ссылку.
- Ошибки оборачивай `%w`, внешние вызовы с context+таймаутами, логи через slog.
- `xray.exe` НЕ коммитим (в .gitignore) — качается через Makefile-таргет из релизов Xray.
- Дистрибуция: статические сборки через **GitHub Releases**.
- Язык общения — русский.

## Команды (Makefile; на Windows нужен `choco install make`)
- `make ui`        — собрать React UI (`frontend/dist`).
- `make build`     — собрать `bin/vpnclient.exe` (после `make ui`).
- `make build-gui` — релизная сборка без консольного окна (`-H windowsgui`).
- `make xray`      — скачать `xray.exe` в `bin/`.
- `make run`       — запустить ядро локально.
- `make release VERSION=vX.Y.Z` — ui + build-gui + xray + zip в `dist/`.

## Связанные репозитории
- `vpn_service` — бэкенд (Go) + node-agent + VPN-ноды. Источник истины по API/конфигу VLESS.
- `vpn-admin-web` — веб админка/кабинет (React+rsbuild+mobx).
