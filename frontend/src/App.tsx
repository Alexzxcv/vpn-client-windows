import { observer } from 'mobx-react-lite';
import { Loader2 } from 'lucide-react';
import { AppRouter } from '@/router/AppRouter';
import { useAuth, useT } from '@/stores/context';
import { Titlebar } from '@/components/Titlebar';

/**
 * Pre-router shell: keeps the custom title bar (so the window is always
 * draggable/closable, even before bootstrap completes) and centers content on
 * the dark background.
 */
function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-full min-h-screen flex-col bg-void text-frost">
      <Titlebar />
      <div className="flex flex-1 items-center justify-center overflow-y-auto p-6">
        <div className="w-full max-w-[440px]">{children}</div>
      </div>
    </div>
  );
}

export const App = observer(function App() {
  const auth = useAuth();
  const t = useT();

  if (auth.bootstrapError) {
    return (
      <Shell>
        <div className="mx-auto max-w-xs rounded-sm border border-alert/40 bg-alert/10 px-3 py-2.5 text-center text-sm text-alert">
          {t('login.coreFailed', { error: auth.bootstrapError })}
        </div>
      </Shell>
    );
  }

  if (!auth.bootstrapped) {
    return (
      <Shell>
        <div className="flex flex-col items-center justify-center gap-2 text-mute">
          <Loader2 className="h-5 w-5 animate-spin text-ion" strokeWidth={1.5} />
          <span className="font-mono text-2xs uppercase tracking-eyebrow">
            {t('common.initializing')}
          </span>
        </div>
      </Shell>
    );
  }

  return <AppRouter />;
});
