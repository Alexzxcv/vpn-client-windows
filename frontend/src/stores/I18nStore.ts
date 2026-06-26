import { makeAutoObservable } from 'mobx';
import { translations, type Lang } from '@/i18n/translations';

const STORAGE_KEY = 'sapn_lang';

function detectDefault(): Lang {
  return (navigator.language || '').toLowerCase().startsWith('ru') ? 'ru' : 'en';
}

export class I18nStore {
  lang: Lang;

  constructor() {
    const saved = localStorage.getItem(STORAGE_KEY) as Lang | null;
    this.lang = saved === 'en' || saved === 'ru' ? saved : detectDefault();
    if (typeof document !== 'undefined') {
      document.documentElement.lang = this.lang;
    }
    makeAutoObservable(this, {}, { autoBind: true });
  }

  setLang(l: Lang): void {
    this.lang = l;
    localStorage.setItem(STORAGE_KEY, l);
    if (typeof document !== 'undefined') {
      document.documentElement.lang = l;
    }
  }

  /** Translate a key, with optional `{name}` interpolation. */
  t(key: string, vars?: Record<string, string | number>): string {
    const dict = translations[this.lang];
    let s = dict[key] ?? translations.en[key] ?? key;
    if (vars) {
      for (const [k, v] of Object.entries(vars)) {
        // split/join performs a global replace without requiring ES2021's
        // String.prototype.replaceAll (the project targets ES2020).
        s = s.split(`{${k}}`).join(String(v));
      }
    }
    return s;
  }
}

/** Singleton so both components and stores share one language state. */
export const i18n = new I18nStore();
