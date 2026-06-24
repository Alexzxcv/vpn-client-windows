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

- `GET  /api/bootstrap` -> `{ session_token, api_base, version, elevated }` — единственный публичный.
  - `elevated` — запущено ли ядро от администратора. TUN-режим требует `elevated:true`; UI прячет/предупреждает иначе.
- `GET  /api/status` -> `{ authenticated: bool, connected: bool, state: "disconnected"|"connecting"|"connected"|"error", mode: "proxy"|"tun", proxy_address?: string, location?: {id,name}, since?: RFC3339, last_error?: string }`
  - `mode` — фактический активный режим туннелирования: `"proxy"` (xray + системный прокси) или `"tun"` (sing-box, полный туннель на всё устройство).
  - `proxy_address` — адрес локального SOCKS-прокси (`127.0.0.1:<port>`), присутствует только когда системный прокси поднят (proxy-режим).
- `POST /api/auth/login` `{ email, password }` -> `{ ok: true }` (ядро сохраняет токены бэкенда локально, шифрованно/в памяти)
- `POST /api/auth/logout` -> `204`
- `GET  /api/me` -> `{ id, email, is_admin }`
- `GET  /api/locations` -> `[{ id, name, location }]` (прокси бэкенд `/vpn/locations`)
- `POST /api/connect` `{ server_id?: string, mode?: "proxy"|"tun" }` -> `{ state }` —
  `mode` по умолчанию `"proxy"`. Ядро тянет `/vpn/config` и поднимает туннель:
  - `proxy`: генерит конфиг xray (SOCKS+HTTP inbound + VLESS Reality outbound), запускает
    xray, ждёт готовности SOCKS-порта, ставит системный прокси Windows.
  - `tun`: требует прав администратора (иначе `{ state:"error", error:"TUN-режим требует
    запуска от администратора" }`). Генерит конфиг sing-box (TUN inbound `sapn-tun` +
    VLESS Reality outbound), запускает sing-box, ждёт готовности. Системный прокси НЕ ставится.
- `POST /api/disconnect` -> `{ state }` — останавливает активный движок (xray или sing-box)
  и снимает системный прокси, если он был поднят.
- `GET  /api/proxy` -> `{ socks: "127.0.0.1:10808", http?: "127.0.0.1:10809" }` — адрес
  локального прокси для проверки (curl --socks5 ...).
- `GET  /api/settings` -> `{ socks_port, http_port, kill_switch, direct_list[], russia_direct }` —
  локальные настройки клиента.
- `PUT  /api/settings` `{ socks_port, http_port, kill_switch?, direct_list?, russia_direct? }` ->
  сохранённые (нормализованные) настройки. Порты применяются на следующем connect;
  split-tunnel/kill-switch — тоже. `direct_list` — список «напрямую»: домены (`.ru`,
  `example.com`) и/или IP/CIDR (`10.0.0.0/8`).

## Идентичность устройства (криптопривязка)
Ed25519 keypair генерится при первом запуске; приватный ключ хранится рядом с токенами
(`%APPDATA%/sapn-vpn/device_key`), запечатанный через Windows DPAPI (никогда в открытом
виде). Регистрация: `POST /devices { public_key (base64 std, 32 байта), name, platform,
mac? }` -> `{ device_id }` (идемпотентно по public_key). Запрос конфига подписывается:
заголовки `X-Device-Id`, `X-Device-Timestamp` (unix sec), `X-Device-Signature` =
base64(Ed25519_sign(privkey, "<device_id>.<timestamp>")). Приватный ключ, device_id и
подпись никогда не логируются.

## Бэкенд-контракт (vpn_service), используемый ядром
- `POST /auth/login` -> `{ access_token, refresh_token }`; refresh на 401.
- `GET  /vpn/locations` -> `[{ id, name, location }]`
- `POST /devices` `{ public_key, name, platform, mac? }` -> `{ device_id }` (JWT, идемпотентно).
- `POST /vpn/config` `{ server_id? }` + headers `X-Device-Id`/`X-Device-Timestamp`/`X-Device-Signature`
  -> `{ server(host), port, uuid, security:"reality", flow, public_key, short_id, sni, fingerprint, expires_at }`.
  Ядро авто-рефрешит конфиг, когда до `expires_at` остаётся < 12ч (фоновый таймер, без разрыва туннеля).
