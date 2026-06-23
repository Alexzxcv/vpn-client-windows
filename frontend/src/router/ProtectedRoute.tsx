import { observer } from 'mobx-react-lite';
import { Navigate } from 'react-router-dom';
import type { ReactNode } from 'react';
import { useAuth } from '@/stores/context';

export const ProtectedRoute = observer(function ProtectedRoute({
  children,
}: {
  children: ReactNode;
}) {
  const auth = useAuth();
  if (!auth.authenticated) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
});
