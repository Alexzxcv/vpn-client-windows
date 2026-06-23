import { observer } from 'mobx-react-lite';
import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertTriangle, LogIn } from 'lucide-react';
import { useAuth } from '@/stores/context';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Eyebrow } from '@/components/ui/card';

export const LoginPage = observer(function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const ok = await auth.login(email.trim(), password);
    if (ok) {
      navigate('/', { replace: true });
    }
  }

  return (
    <div className="flex h-full flex-col justify-center">
      <div className="mb-6">
        <Eyebrow>Secure access</Eyebrow>
        <h1 className="mt-1 font-display text-xl font-semibold tracking-tight text-frost">
          Sign in
        </h1>
      </div>

      <form className="flex flex-col gap-4" onSubmit={onSubmit}>
        <label className="flex flex-col gap-1.5">
          <Eyebrow>Email</Eyebrow>
          <Input
            type="email"
            autoComplete="username"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoFocus
          />
        </label>

        <label className="flex flex-col gap-1.5">
          <Eyebrow>Password</Eyebrow>
          <Input
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </label>

        {auth.loginError && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-sm border border-alert/40 bg-alert/10 px-2.5 py-2 text-sm text-alert"
          >
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" strokeWidth={1.5} />
            <span className="break-words">{auth.loginError}</span>
          </div>
        )}

        <Button type="submit" disabled={auth.loggingIn} className="mt-1">
          <LogIn className="h-4 w-4" strokeWidth={1.5} />
          {auth.loggingIn ? 'Signing in…' : 'Sign in'}
        </Button>
      </form>
    </div>
  );
});
