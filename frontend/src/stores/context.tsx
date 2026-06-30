import { createContext, useContext, type ReactNode } from 'react';
import type { RootStore } from './RootStore';

const StoreContext = createContext<RootStore | null>(null);

export function StoreProvider({
  store,
  children,
}: {
  store: RootStore;
  children: ReactNode;
}) {
  return (
    <StoreContext.Provider value={store}>{children}</StoreContext.Provider>
  );
}

export function useStores(): RootStore {
  const store = useContext(StoreContext);
  if (!store) {
    throw new Error('useStores must be used within a StoreProvider');
  }
  return store;
}

export function useAuth() {
  return useStores().auth;
}

export function useConnection() {
  return useStores().connection;
}

export function useMultiProxy() {
  return useStores().multiProxy;
}

export function useSettings() {
  return useStores().settings;
}

export function useUpdate() {
  return useStores().update;
}

export function useI18n() {
  return useStores().i18n;
}

/**
 * Reactive translate function. Must be called inside an `observer` component:
 * `t()` reads `i18n.lang` (observable), so the component re-renders on language
 * change.
 */
export function useT() {
  const s = useI18n();
  return (k: string, vars?: Record<string, string | number>) => s.t(k, vars);
}
