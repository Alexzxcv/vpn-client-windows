export type Lang = 'en' | 'ru';

/**
 * Flat, dot-namespaced translation keys. Both languages MUST carry the same set
 * of keys. Simple `{name}` interpolation is handled by the I18nStore.
 *
 * Areas: login.* connect.* settings.* nav.* status.* custom.* update.* common.*
 */
export const translations: Record<Lang, Record<string, string>> = {
  en: {
    // common
    'common.save': 'Save',
    'common.saving': 'Saving…',
    'common.saved': 'Saved',
    'common.back': 'Back',
    'common.add': 'Add',
    'common.dismiss': 'Dismiss',
    'common.copy': 'Copy',
    'common.copied': 'Copied',
    'common.initializing': 'Initializing',
    'common.loading': 'Loading…',

    // login
    'login.eyebrow': 'Secure access',
    'login.signIn': 'Sign in',
    'login.identifier': 'Email or username',
    'login.password': 'Password',
    'login.otp': 'Verification code (2FA)',
    'login.otpHint': 'Enter the 6-digit code from your authenticator app.',
    'login.signingIn': 'Signing in…',
    'login.confirm': 'Confirm',
    'login.noAccount': "Don't have an account?",
    'login.createAccount': 'Create account',
    'login.failed': 'Login failed',
    'login.badOtp': 'Invalid verification code',
    'login.bootstrapFailed': 'Bootstrap failed',
    'login.coreFailed': 'Core connection failed: {error}',

    // nav / footer actions
    'nav.settings': 'Settings',
    'nav.logout': 'Log out',
    'nav.minimize': 'Minimize',
    'nav.maximize': 'Maximize',
    'nav.close': 'Close',

    // connect page
    'connect.mode': 'Mode',
    'connect.modeProxy': 'Proxy',
    'connect.modeTun': 'Full tunnel',
    'connect.tunNeedsAdmin':
      'Full tunnel requires administrator rights. Run the app as administrator.',
    'connect.location': 'Location',
    'connect.locationLoading': 'Loading…',
    'connect.auto': 'Auto (best) — lowest latency',
    'connect.autoName': 'Auto (best)',
    'connect.autoSub': 'lowest latency',
    'connect.connectFailed': 'Connect failed',
    'connect.disconnectFailed': 'Disconnect failed',
    'connect.socksProxy': 'SOCKS proxy',
    'connect.copyCurl': 'Copy curl test',
    'connect.copyCurlAria': 'Copy curl test command',

    // metrics
    'connect.ping': 'Ping',
    'connect.traffic': 'Traffic',

    // custom servers
    'custom.title': 'My servers',
    'custom.placeholder': 'vless://… or https://… (subscription)',
    'custom.adding': 'Adding…',
    'custom.remove': 'Remove {name}',
    'custom.copyLink': 'Copy vless link for {name}',
    'custom.note':
      'Traffic over your own servers is not counted and not limited by the subscription.',
    'custom.addFailed': 'Failed to add server',
    'custom.removeFailed': 'Failed to remove server',

    // status badge / connect button
    'status.offline': 'OFFLINE',
    'status.linking': 'LINKING',
    'status.secured': 'SECURED',
    'status.error': 'ERROR',
    'status.connect': 'CONNECT',
    'status.disconnect': 'DISCONNECT',
    'status.retry': 'RETRY',

    // update banner
    'update.available': 'Update available {version}',
    'update.downloading': 'Downloading…',
    'update.apply': 'Update',
    'update.dismiss': 'Dismiss update',

    // settings page
    'settings.title': 'Settings',
    'settings.ports': 'Local proxy ports',
    'settings.portsHint': 'Applied on the next connect.',
    'settings.autostart': 'Start with Windows',
    'settings.autostartHint':
      'Launch automatically at login (starts minimized to the tray).',
    'settings.killSwitch': 'Kill switch',
    'settings.killSwitchHint':
      'Block all traffic if the tunnel drops (recommended for full tunnel).',
    'settings.russiaDirect': 'Russian sites direct',
    'settings.russiaDirectHint':
      'Route .ru / Russian IPs outside the tunnel (geosite:ru / geoip:ru).',
    'settings.directList': 'Direct list (split tunnel)',
    'settings.directListHint':
      'One entry per line: domains (.ru, example.com) or IP/CIDR (10.0.0.0/8). These bypass the tunnel.',
    'settings.language': 'Language',
  },
  ru: {
    // common
    'common.save': 'Сохранить',
    'common.saving': 'Сохранение…',
    'common.saved': 'Сохранено',
    'common.back': 'Назад',
    'common.add': 'Добавить',
    'common.dismiss': 'Закрыть',
    'common.copy': 'Копировать',
    'common.copied': 'Скопировано',
    'common.initializing': 'Инициализация',
    'common.loading': 'Загрузка…',

    // login
    'login.eyebrow': 'Защищённый вход',
    'login.signIn': 'Войти',
    'login.identifier': 'Email или логин',
    'login.password': 'Пароль',
    'login.otp': 'Код подтверждения (2FA)',
    'login.otpHint': 'Введите 6-значный код из приложения-аутентификатора.',
    'login.signingIn': 'Вход…',
    'login.confirm': 'Подтвердить',
    'login.noAccount': 'Нет аккаунта?',
    'login.createAccount': 'Создать аккаунт',
    'login.failed': 'Не удалось войти',
    'login.badOtp': 'Неверный код подтверждения',
    'login.bootstrapFailed': 'Ошибка инициализации',
    'login.coreFailed': 'Не удалось подключиться к ядру: {error}',

    // nav / footer actions
    'nav.settings': 'Настройки',
    'nav.logout': 'Выйти',
    'nav.minimize': 'Свернуть',
    'nav.maximize': 'Развернуть',
    'nav.close': 'Закрыть',

    // connect page
    'connect.mode': 'Режим',
    'connect.modeProxy': 'Прокси',
    'connect.modeTun': 'Полный туннель',
    'connect.tunNeedsAdmin':
      'Полный туннель требует прав администратора. Запустите приложение от имени администратора.',
    'connect.location': 'Локация',
    'connect.locationLoading': 'Загрузка…',
    'connect.auto': 'Авто (лучший) — минимальная задержка',
    'connect.autoName': 'Авто (лучший)',
    'connect.autoSub': 'минимальная задержка',
    'connect.connectFailed': 'Не удалось подключиться',
    'connect.disconnectFailed': 'Не удалось отключиться',
    'connect.socksProxy': 'SOCKS-прокси',
    'connect.copyCurl': 'Скопировать curl-тест',
    'connect.copyCurlAria': 'Скопировать команду curl для проверки',

    // metrics
    'connect.ping': 'Пинг',
    'connect.traffic': 'Трафик',

    // custom servers
    'custom.title': 'Свои серверы',
    'custom.placeholder': 'vless://… или https://… (подписка)',
    'custom.adding': 'Добавление…',
    'custom.remove': 'Удалить {name}',
    'custom.copyLink': 'Скопировать vless-ссылку для {name}',
    'custom.note':
      'Трафик по своим серверам не учитывается и не ограничен подпиской.',
    'custom.addFailed': 'Не удалось добавить сервер',
    'custom.removeFailed': 'Не удалось удалить сервер',

    // status badge / connect button
    'status.offline': 'ОФФЛАЙН',
    'status.linking': 'ПОДКЛЮЧЕНИЕ',
    'status.secured': 'ЗАЩИЩЕНО',
    'status.error': 'ОШИБКА',
    'status.connect': 'ПОДКЛЮЧИТЬ',
    'status.disconnect': 'ОТКЛЮЧИТЬ',
    'status.retry': 'ПОВТОРИТЬ',

    // update banner
    'update.available': 'Доступно обновление {version}',
    'update.downloading': 'Загрузка…',
    'update.apply': 'Обновить',
    'update.dismiss': 'Закрыть уведомление об обновлении',

    // settings page
    'settings.title': 'Настройки',
    'settings.ports': 'Локальные порты прокси',
    'settings.portsHint': 'Применится при следующем подключении.',
    'settings.autostart': 'Запуск вместе с Windows',
    'settings.autostartHint':
      'Запускать автоматически при входе (стартует свёрнутым в трей).',
    'settings.killSwitch': 'Аварийное отключение (kill switch)',
    'settings.killSwitchHint':
      'Блокировать весь трафик при обрыве туннеля (рекомендуется для полного туннеля).',
    'settings.russiaDirect': 'Российские сайты напрямую',
    'settings.russiaDirectHint':
      'Пускать .ru / российские IP мимо туннеля (geosite:ru / geoip:ru).',
    'settings.directList': 'Список «напрямую» (split tunnel)',
    'settings.directListHint':
      'По одной записи в строке: домены (.ru, example.com) или IP/CIDR (10.0.0.0/8). Они идут мимо туннеля.',
    'settings.language': 'Язык',
  },
};
