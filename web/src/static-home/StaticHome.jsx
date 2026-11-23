import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, Globe, Moon, Sun, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { BrandLockup } from '@/components/chrome/Logo';
import i18nList from 'i18n/i18nList';
import Hero from 'views/public/home/Hero';
import { STATIC_TOKENS } from './tokens';
import { workbench } from 'routes/paths';

// ==============================|| STATIC HOME (prerendered) ||============================== //
// The page the server returns at `/`: rendered to plain HTML at build time, one file per language,
// and shipped without the app bundle so search engines and first-time visitors get the content
// straight away. It never reads the login state: the header always offers "Console", and the panel
// sends visitors who are not signed in through login.
//
// Nothing here may depend on client-side React. The language picker is a native popover with plain
// links, and the theme button is wired by a small inline script (see scripts/prerender-home.mjs).

const escapeText = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

// The footer's docs link, between markers the server uses to drop it when no docs link is set.
function docsLinkMarkup(label) {
  const { docsStart, docsEnd, docsUrl } = STATIC_TOKENS;
  return `${docsStart}<a href="${docsUrl}" target="_blank" rel="noopener noreferrer" class="text-[0.8125rem] leading-none hover:text-foreground hover:underline">${escapeText(label)}</a>${docsEnd}`;
}

// The footer's language button and the dialog it opens. A native popover needs no script: the
// button toggles it, Esc and a click outside close it, and each language is a plain link.
function LanguageMenu({ lng }) {
  const { t } = useTranslation();
  const current = i18nList.find((item) => item.lng === lng) || i18nList[0];
  const title = t('language.select', { defaultValue: 'Language' });

  return (
    <>
      <button
        type="button"
        popoverTarget="language-picker"
        aria-label={title}
        className="inline-flex cursor-pointer items-center gap-1.5 rounded-md px-2 py-1 text-[0.8125rem] leading-none text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
      >
        <Globe className="size-4" />
        {current.name}
      </button>
      <div
        id="language-picker"
        popover="auto"
        role="dialog"
        aria-labelledby="language-picker-title"
        className="m-auto w-80 max-w-[calc(100%-2rem)] rounded-2xl border border-border bg-card p-0 text-card-foreground shadow-xl backdrop:bg-black/50 backdrop:backdrop-blur-sm"
      >
        <div className="flex items-center justify-between px-5 pb-2 pt-4">
          <h2 id="language-picker-title" className="text-base font-semibold">
            {title}
          </h2>
          <button
            type="button"
            popoverTarget="language-picker"
            popoverTargetAction="hide"
            aria-label={t('common.close')}
            className="inline-flex size-8 cursor-pointer items-center justify-center rounded-lg hover:bg-muted"
          >
            <X className="size-4" />
          </button>
        </div>
        <ul className="flex flex-col gap-0.5 px-2 pb-3">
          {i18nList.map((item) => (
            <li key={item.lng}>
              <a
                href={`/?lang=${item.lng}`}
                hrefLang={item.lng.replace('_', '-')}
                lang={item.lng.replace('_', '-')}
                aria-current={item.lng === lng ? 'true' : undefined}
                className={`flex items-center justify-between rounded-lg px-3 py-2.5 text-sm hover:bg-muted ${item.lng === lng ? 'font-medium' : ''}`}
              >
                {item.name}
                {item.lng === lng && <Check className="size-4" />}
              </a>
            </li>
          ))}
        </ul>
      </div>
    </>
  );
}

LanguageMenu.propTypes = { lng: PropTypes.string.isRequired };

export default function StaticHome({ lng }) {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="flex h-14 items-center justify-between gap-2 border-b border-border px-4 sm:px-6">
        <a href="/" className="flex items-center">
          <BrandLockup text={STATIC_TOKENS.wordmark} fontClass={STATIC_TOKENS.wordmarkClass} />
        </a>
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon" aria-label="Toggle theme" data-theme-toggle="">
            <Moon className="size-5 dark:hidden" />
            <Sun className="hidden size-5 dark:block" />
          </Button>
          <Button asChild size="sm" className="ml-1">
            <a href={workbench.dashboard()}>{t('home.cta.console')}</a>
          </Button>
        </div>
      </header>

      <main className="flex-1">
        <div className="bg-background">
          <Hero />
        </div>
      </main>

      <footer className="grid grid-cols-[1fr_auto_1fr] items-center gap-2 px-4 py-5 text-muted-foreground sm:px-6">
        <div className="col-start-2 flex items-center gap-1.5">
          <BrandLockup size={12} text={STATIC_TOKENS.wordmark} fontClass={STATIC_TOKENS.wordmarkClass} />
          <span className="text-[0.8125rem] leading-none">© 2026</span>
          {/* comments cannot be rendered as React nodes; the markup is a constant plus escaped text */}
          <span className="contents" dangerouslySetInnerHTML={{ __html: docsLinkMarkup(t('common.docs')) }} />
        </div>
        <div className="flex justify-end">
          <LanguageMenu lng={lng} />
        </div>
      </footer>
    </div>
  );
}

StaticHome.propTypes = { lng: PropTypes.string.isRequired };
