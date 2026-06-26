import { observer } from 'mobx-react-lite';
import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertTriangle, LogIn, UserPlus } from 'lucide-react';
import { useAuth, useT } from '@/stores/context';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Eyebrow } from '@/components/ui/card';
import { LangSwitch } from '@/components/LangSwitch';

export const LoginPage = observer(function LoginPage() {
  const auth = useAuth();
  const t = useT();
  const navigate = useNavigate();
  const [loginId, setLoginId] = useState('');
  const [password, setPassword] = useState('');
  const [otp, setOtp] = useState('');

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const ok = await auth.login(
      loginId.trim(),
      password,
      auth.mfaRequired ? otp.trim() : undefined,
    );
    if (ok) {
      navigate('/', { replace: true });
    }
  }

  return (
    <div className="flex h-full flex-col justify-center">
      <div className="mb-6 flex items-start justify-between gap-3">
        <div>
          <Eyebrow>{t('login.eyebrow')}</Eyebrow>
          <h1 className="mt-1 font-display text-xl font-semibold tracking-tight text-frost">
            {t('login.signIn')}
          </h1>
        </div>
        <LangSwitch />
      </div>

      <form className="flex flex-col gap-4" onSubmit={onSubmit}>
        <label className="flex flex-col gap-1.5">
          <Eyebrow>{t('login.identifier')}</Eyebrow>
          <Input
            type="text"
            autoComplete="username"
            value={loginId}
            onChange={(e) => setLoginId(e.target.value)}
            required
            autoFocus
          />
        </label>

        <label className="flex flex-col gap-1.5">
          <Eyebrow>{t('login.password')}</Eyebrow>
          <Input
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </label>

        {auth.mfaRequired && (
          <label className="flex flex-col gap-1.5">
            <Eyebrow>{t('login.otp')}</Eyebrow>
            <Input
              type="text"
              inputMode="numeric"
              autoComplete="one-time-code"
              placeholder="123456"
              maxLength={6}
              value={otp}
              onChange={(e) =>
                setOtp(e.target.value.replace(/\D/g, '').slice(0, 6))
              }
              required
              autoFocus
            />
            <span className="text-2xs text-mute">{t('login.otpHint')}</span>
          </label>
        )}

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
          {auth.loggingIn
            ? t('login.signingIn')
            : auth.mfaRequired
              ? t('login.confirm')
              : t('login.signIn')}
        </Button>
      </form>

      {auth.dashboardUrl && (
        <div className="mt-6 flex flex-col items-center gap-2 border-t border-hairline pt-5">
          <span className="text-2xs text-mute">{t('login.noAccount')}</span>
          <button
            type="button"
            onClick={() => void auth.openRegister()}
            className="flex items-center gap-1.5 text-sm text-ion hover:text-frost"
          >
            <UserPlus className="h-4 w-4" strokeWidth={1.5} />
            {t('login.createAccount')}
          </button>
        </div>
      )}
    </div>
  );
});
