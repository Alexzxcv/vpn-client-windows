# Control API — контракт между Go-ядром и React-UI

Go-ядро поднимает локальный HTTP-сервер на `127.0.0.1:<port>` (порт выбирается свободный,
печатается в лог и передаётся в окно). React-UI (WebView2) ходит на него. Сервер
обслуживает и статику UI (`/`), и control-API (`/api/*`).

## Безопасность
- Слушать ТОЛЬКО `127.0.0.1` (не `0.0.0.0`).
- Проверять заголовок `Origin`/`Host` — принимать только локальные.
- При старте ядро генерирует разовый `session_token`; UI получает его из инъекции в
  `index.html` (через подстановку) или из `GET /api/bootstrap`. Каждый `/api/*` запрос
  (кроме /bootstrap) шлёт `Authorization: Bearer <session_token>`.

## Эндпоинты (JSON)

- `GET  /api/bootstrap` -> `{ session_token, api_base, version }` — единственный публичный.
- `GET  /api/status` -> `{ authenticated: bool, connected: bool, state: "disconnected"|"connecting"|"connected"|"error", location?: {id,name}, since?: RFC3339, last_error?: string }`
- `POST /api/auth/login` `{ email, password }` -> `{ ok: true }` (ядро сохраняет токены бэкенда локально, шифрованно/в памяти)
- `POST /api/auth/logout` -> `204`
- `GET  /api/me` -> `{ id, email, is_admin }`
- `GET  /api/locations` -> `[{ id, name, location }]` (прокси бэкенд `/vpn/locations`)
- `POST /api/connect` `{ server_id?: string }` -> `{ state }` — ядро: тянет `/vpn/config`,
  генерит конфиг xray (SOCKS inbound 127.0.0.1:10808 + VLESS Reality outbound), запускает
  xray, ждёт готовности.
- `POST /api/disconnect` -> `{ state }` — останавливает xray.
- `GET  /api/proxy` -> `{ socks: "127.0.0.1:10808", http?: "127.0.0.1:10809" }` — адрес
  локального прокси для проверки (curl --socks5 ...).

## device_id
Стабильный идентификатор машины: SHA-256 от Windows `MachineGuid`
(`HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid`). Ядро шлёт его в бэкенд при
`/vpn/config` и привязке устройства.

## Бэкенд-контракт (vpn_service), используемый ядром
- `POST /auth/login` -> `{ access_token, refresh_token }`; refresh на 401.
- `GET  /vpn/locations` -> `[{ id, name, location }]`
- `POST /vpn/config` `{ device_id, server_id? }` -> `{ server(host), port, uuid, security:"reality", flow, public_key, short_id, sni, fingerprint }`
