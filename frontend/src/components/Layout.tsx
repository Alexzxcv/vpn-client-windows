import { observer } from 'mobx-react-lite';
import type { ReactNode } from 'react';
import { useAuth } from '@/stores/context';
import { Eyebrow } from '@/components/ui/card';
import { Titlebar } from '@/components/Titlebar';

/**
 * App chrome: custom frameless title bar + a width-capped, centered content
 * column. The client is a compact control panel — content never stretches into
 * a wide dashboard on large/maximized windows; it stays centered on the dark
 * background.
 */
export const Layout = observer(function Layout({
  children,
}: {
  children: ReactNode;
}) {
  const auth = useAuth();
  return (
    <div className="flex h-screen flex-col overflow-hidden bg-void text-frost">
      <Titlebar />
      <div className="flex min-h-0 flex-1 justify-center overflow-y-auto">
        <div className="flex w-full max-w-[440px] flex-col">
          {auth.version && (
            <div className="flex justify-end px-4 pt-2">
              <Eyebrow className="tabnum">v{auth.version}</Eyebrow>
            </div>
          )}
          <main className="flex-1 p-4 pt-2">{children}</main>
        </div>
      </div>
    </div>
  );
});
