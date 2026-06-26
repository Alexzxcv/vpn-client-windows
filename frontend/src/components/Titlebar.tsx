import { observer } from 'mobx-react-lite';
import { Minus, Square, X } from 'lucide-react';
import type { MouseEvent } from 'react';
import { Logo } from '@/components/Logo';
import { api } from '@/api/control';
import { i18n } from '@/stores/I18nStore';

/**
 * Native bridge functions injected by the Go host (go-webview2 Bind). They are
 * the reliable path for drag (mouse capture must run on the UI thread) and a
 * fallback for the window controls if the HTTP endpoints are unavailable.
 */
declare global {
  interface Window {
    windowStartDrag?: () => void;
    windowMinimize?: () => void;
    windowMaximize?: () => void;
    windowClose?: () => void;
  }
}

function startDrag(e: MouseEvent) {
  // Only left-button drags; ignore clicks that originate on the controls.
  if (e.button !== 0) return;
  if ((e.target as HTMLElement).closest('[data-no-drag]')) return;
  // -webkit-app-region is unsupported in WebView2, so ask the native host to
  // begin an OS move-drag.
  window.windowStartDrag?.();
}

/**
 * Custom frameless title bar: SAPN mark + name on the left, window controls on
 * the right. The whole bar (minus the buttons) is the drag region.
 */
export const Titlebar = observer(function Titlebar() {
  const minimize = () => {
    void api.windowMinimize().catch(() => window.windowMinimize?.());
  };
  const maximize = () => {
    void api.windowMaximize().catch(() => window.windowMaximize?.());
  };
  const close = () => {
    void api.windowClose().catch(() => window.windowClose?.());
  };

  return (
    <header
      onMouseDown={startDrag}
      className="flex h-9 shrink-0 select-none items-center justify-between border-b border-hairline bg-slate pl-3 pr-1"
    >
      <span className="flex items-center gap-2">
        <Logo className="h-4 w-4" />
        <span className="font-display text-xs font-semibold tracking-tight text-frost">
          SAPN<span className="text-ion">·</span>VPN
        </span>
      </span>

      <div data-no-drag className="flex items-center">
        <button
          type="button"
          onClick={minimize}
          aria-label={i18n.t('nav.minimize')}
          className="flex h-9 w-10 items-center justify-center text-ash transition-colors hover:bg-graphite hover:text-frost"
        >
          <Minus className="h-3.5 w-3.5" strokeWidth={1.5} />
        </button>
        <button
          type="button"
          onClick={maximize}
          aria-label={i18n.t('nav.maximize')}
          className="flex h-9 w-10 items-center justify-center text-ash transition-colors hover:bg-graphite hover:text-frost"
        >
          <Square className="h-3 w-3" strokeWidth={1.5} />
        </button>
        <button
          type="button"
          onClick={close}
          aria-label={i18n.t('nav.close')}
          className="flex h-9 w-10 items-center justify-center text-ash transition-colors hover:bg-alert hover:text-void"
        >
          <X className="h-4 w-4" strokeWidth={1.5} />
        </button>
      </div>
    </header>
  );
});

export default Titlebar;
