import { observer } from 'mobx-react-lite';
import { useI18n } from '@/stores/context';
import type { Lang } from '@/i18n/translations';
import { cn } from '@/lib/utils';

const LANGS: { id: Lang; label: string }[] = [
  { id: 'en', label: 'EN' },
  { id: 'ru', label: 'RU' },
];

/** Compact EN | RU segmented toggle. Switches language on the fly. */
export const LangSwitch = observer(function LangSwitch({
  className,
}: {
  className?: string;
}) {
  const i18n = useI18n();
  return (
    <div
      role="radiogroup"
      aria-label="Language"
      className={cn(
        'inline-flex items-center gap-0.5 rounded-sm border border-edge bg-void p-0.5',
        className,
      )}
    >
      {LANGS.map((l) => {
        const active = i18n.lang === l.id;
        return (
          <button
            key={l.id}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => i18n.setLang(l.id)}
            className={cn(
              'rounded-sm px-2 py-0.5 font-mono text-2xs uppercase tracking-eyebrow transition-colors',
              active
                ? 'bg-ion text-void'
                : 'text-ash hover:bg-graphite hover:text-frost',
            )}
          >
            {l.label}
          </button>
        );
      })}
    </div>
  );
});
