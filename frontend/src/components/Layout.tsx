import { observer } from 'mobx-react-lite';
import type { ReactNode } from 'react';
import { useAuth } from '@/stores/context';

export const Layout = observer(function Layout({
  children,
}: {
  children: ReactNode;
}) {
  const auth = useAuth();
  return (
    <div className="app">
      <header className="app-header">
        <span className="app-title">VPN Client</span>
        {auth.version && <span className="app-version">v{auth.version}</span>}
      </header>
      <main className="app-main">{children}</main>
    </div>
  );
});
