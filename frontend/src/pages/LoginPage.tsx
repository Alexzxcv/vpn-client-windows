import { observer } from 'mobx-react-lite';
import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/stores/context';

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
    <form className="login" onSubmit={onSubmit}>
      <h1 className="login-title">Вход</h1>

      <label className="field">
        <span className="field-label">Email</span>
        <input
          type="email"
          autoComplete="username"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          autoFocus
        />
      </label>

      <label className="field">
        <span className="field-label">Пароль</span>
        <input
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
      </label>

      {auth.loginError && <div className="error">{auth.loginError}</div>}

      <button type="submit" className="btn-primary" disabled={auth.loggingIn}>
        {auth.loggingIn ? 'Вход…' : 'Войти'}
      </button>
    </form>
  );
});
