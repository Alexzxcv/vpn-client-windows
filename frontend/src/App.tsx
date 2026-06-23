import { observer } from 'mobx-react-lite';
import { Loader2 } from 'lucide-react';
import { AppRouter } from '@/router/AppRouter';
import { useAuth } from '@/stores/context';

export const App = observer(function App() {
  const auth = useAuth();

  if (auth.bootstrapError) {
    return (
      <div className="flex h-full min-h-screen items-center justify-center bg-void p-6">
        <div className="max-w-xs rounded-sm border border-alert/40 bg-alert/10 px-3 py-2.5 text-sm text-alert">
          Core connection failed: {auth.bootstrapError}
        </div>
      </div>
    );
  }

  if (!auth.bootstrapped) {
    return (
      <div className="flex h-full min-h-screen flex-col items-center justify-center gap-2 bg-void text-mute">
        <Loader2 className="h-5 w-5 animate-spin text-ion" strokeWidth={1.5} />
        <span className="font-mono text-2xs uppercase tracking-eyebrow">
          Initializing
        </span>
      </div>
    );
  }

  return <AppRouter />;
});
