import { observer } from 'mobx-react-lite';
import type { ReactNode } from 'react';
import { useAuth } from '@/stores/context';
import { Eyebrow } from '@/components/ui/card';
import { Logo } from '@/components/Logo';

export const Layout = observer(function Layout({
  children,
}: {
  children: ReactNode;
}) {
  const auth = useAuth();
  return (
    <div className="flex h-full min-h-screen flex-col bg-void text-frost">
      <header className="flex items-center justify-between border-b border-hairline bg-slate px-4 py-2.5">
        <span className="flex items-center gap-2">
          <Logo className="h-5 w-5" />
          <span className="font-display text-sm font-semibold tracking-tight">
            SAPN<span className="text-ion">·</span>VPN
          </span>
        </span>
        {auth.version && (
          <Eyebrow className="tabnum">v{auth.version}</Eyebrow>
        )}
      </header>
      <main className="flex-1 overflow-y-auto p-4">{children}</main>
    </div>
  );
});
