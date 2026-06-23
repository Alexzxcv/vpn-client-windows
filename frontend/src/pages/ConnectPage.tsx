import { observer } from 'mobx-react-lite';
import { useState } from 'react';
import { useConnection, useAuth } from '@/stores/context';
import { StatusBadge } from '@/components/StatusBadge';
import { ConnectButton } from '@/components/ConnectButton';

export const ConnectPage = observer(function ConnectPage() {
  const conn = useConnection();
  const auth = useAuth();
  const [copied, setCopied] = useState(false);

  const socks = conn.proxy?.socks ?? '127.0.0.1:10808';
  const curlHint = `curl --socks5 ${socks} https://ifconfig.me`;

  async function copyCurl() {
    try {
      await navigator.clipboard.writeText(curlHint);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // буфер обмена недоступен — игнорируем
    }
  }

  return (
    <div className="connect">
      <div className="connect-status-row">
        <StatusBadge state={conn.state} />
        {auth.me && <span className="connect-user">{auth.me.email}</span>}
      </div>

      <div className="field">
        <span className="field-label">Локация</span>
        <select
          value={conn.selectedServerId ?? ''}
          onChange={(e) => conn.setSelectedServer(e.target.value)}
          disabled={conn.state === 'connected' || conn.state === 'connecting'}
        >
          {conn.locations.length === 0 && (
            <option value="">Загрузка…</option>
          )}
          {conn.locations.map((loc) => (
            <option key={loc.id} value={loc.id}>
              {loc.name} — {loc.location}
            </option>
          ))}
        </select>
      </div>

      <ConnectButton
        state={conn.state}
        busy={conn.busy}
        onConnect={() => void conn.connect()}
        onDisconnect={() => void conn.disconnect()}
      />

      {(conn.actionError || (conn.state === 'error' && conn.lastError)) && (
        <div className="error">
          {conn.actionError ?? conn.lastError}
        </div>
      )}

      {conn.connected && (
        <div className="proxy-box">
          <div className="proxy-row">
            <span className="field-label">SOCKS-прокси</span>
            <code className="proxy-addr">{socks}</code>
          </div>
          {conn.proxy?.http && (
            <div className="proxy-row">
              <span className="field-label">HTTP-прокси</span>
              <code className="proxy-addr">{conn.proxy.http}</code>
            </div>
          )}
          <div className="proxy-hint">
            <span className="field-label">Проверка</span>
            <code className="proxy-curl">{curlHint}</code>
            <button type="button" className="btn-link" onClick={copyCurl}>
              {copied ? 'Скопировано' : 'Копировать'}
            </button>
          </div>
        </div>
      )}

      <button
        type="button"
        className="btn-logout"
        onClick={() => void auth.logout()}
      >
        Выйти
      </button>
    </div>
  );
});
