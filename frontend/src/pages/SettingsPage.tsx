import { observer } from 'mobx-react-lite';
import { useEffect, useState } from 'react';
import { ArrowLeft, Check, Save } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import type { Settings } from '@/api/types';
import { useSettings } from '@/stores/context';
import { Button } from '@/components/ui/button';
import { Card, Eyebrow } from '@/components/ui/card';
import { Input } from '@/components/ui/input';

/** A simple labelled on/off switch styled to match the mode toggle. */
function Toggle({
  checked,
  onChange,
  disabled,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`inline-flex h-5 w-9 shrink-0 items-center overflow-hidden rounded-full px-0.5 transition-colors disabled:opacity-50 ${
        checked ? 'bg-ion' : 'bg-graphite'
      }`}
    >
      <span
        className={`block h-4 w-4 rounded-full bg-frost transition-transform ${
          checked ? 'translate-x-4' : 'translate-x-0'
        }`}
      />
    </button>
  );
}

export const SettingsPage = observer(function SettingsPage() {
  const settings = useSettings();
  const navigate = useNavigate();

  const [socks, setSocks] = useState('10800');
  const [http, setHttp] = useState('10801');
  const [killSwitch, setKillSwitch] = useState(true);
  const [russiaDirect, setRussiaDirect] = useState(false);
  const [directList, setDirectList] = useState('');

  useEffect(() => {
    void settings.load();
  }, [settings]);

  useEffect(() => {
    const s = settings.current;
    if (!s) return;
    setSocks(String(s.socks_port));
    setHttp(String(s.http_port));
    setKillSwitch(s.kill_switch ?? true);
    setRussiaDirect(s.russia_direct ?? false);
    setDirectList((s.direct_list ?? []).join('\n'));
  }, [settings.current]);

  async function onSave() {
    const next: Settings = {
      socks_port: Number(socks) || 10800,
      http_port: Number(http) || 10801,
      kill_switch: killSwitch,
      russia_direct: russiaDirect,
      direct_list: directList
        .split('\n')
        .map((l) => l.trim())
        .filter((l) => l.length > 0),
    };
    await settings.save(next);
  }

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => navigate('/')}
          className="flex items-center gap-1.5 font-mono text-2xs uppercase tracking-eyebrow text-mute hover:text-frost"
        >
          <ArrowLeft className="h-3.5 w-3.5" strokeWidth={1.5} />
          Back
        </button>
        <Eyebrow>Settings</Eyebrow>
      </div>

      {/* local proxy ports */}
      <Card className="flex flex-col gap-2 p-3">
        <Eyebrow>Local proxy ports</Eyebrow>
        <div className="grid grid-cols-2 gap-2">
          <label className="flex flex-col gap-1">
            <span className="text-2xs text-mute">SOCKS</span>
            <Input
              inputMode="numeric"
              value={socks}
              onChange={(e) => setSocks(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-2xs text-mute">HTTP</span>
            <Input
              inputMode="numeric"
              value={http}
              onChange={(e) => setHttp(e.target.value)}
            />
          </label>
        </div>
        <span className="text-2xs text-mute">
          Applied on the next connect.
        </span>
      </Card>

      {/* kill-switch */}
      <Card className="flex items-center justify-between gap-3 p-3">
        <div className="flex flex-col gap-0.5">
          <span className="text-sm text-frost">Kill switch</span>
          <span className="text-2xs text-mute">
            Block all traffic if the tunnel drops (recommended for full tunnel).
          </span>
        </div>
        <Toggle checked={killSwitch} onChange={setKillSwitch} />
      </Card>

      {/* Russia direct */}
      <Card className="flex items-center justify-between gap-3 p-3">
        <div className="flex flex-col gap-0.5">
          <span className="text-sm text-frost">Russian sites direct</span>
          <span className="text-2xs text-mute">
            Route .ru / Russian IPs outside the tunnel (geosite:ru / geoip:ru).
          </span>
        </div>
        <Toggle checked={russiaDirect} onChange={setRussiaDirect} />
      </Card>

      {/* split-tunnel list */}
      <Card className="flex flex-col gap-2 p-3">
        <Eyebrow>Direct list (split tunnel)</Eyebrow>
        <span className="text-2xs text-mute">
          One entry per line: domains (.ru, example.com) or IP/CIDR
          (10.0.0.0/8). These bypass the tunnel.
        </span>
        <textarea
          value={directList}
          onChange={(e) => setDirectList(e.target.value)}
          rows={5}
          spellCheck={false}
          className="w-full rounded-sm border border-hairline bg-slate px-2.5 py-2 font-mono text-xs text-frost placeholder:text-mute focus-visible:border-ion focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ion/40"
          placeholder={'.ru\nexample.com\n10.0.0.0/8'}
        />
      </Card>

      {settings.error && (
        <div className="rounded-sm border border-alert/40 bg-alert/10 px-2.5 py-1.5 text-xs text-alert">
          {settings.error}
        </div>
      )}

      <div className="mt-auto flex items-center justify-end gap-2 pt-1">
        {settings.saved && !settings.saving && (
          <span className="flex items-center gap-1 text-2xs text-ok">
            <Check className="h-3.5 w-3.5" strokeWidth={1.5} />
            Saved
          </span>
        )}
        <Button onClick={() => void onSave()} disabled={settings.saving}>
          <Save className="mr-1.5 h-4 w-4" strokeWidth={1.5} />
          {settings.saving ? 'Saving…' : 'Save'}
        </Button>
      </div>
    </div>
  );
});
