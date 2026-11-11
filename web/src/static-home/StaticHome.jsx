import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, Globe, Moon, Sun } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { LogoMark } from '@/components/chrome/Logo';
import i18nList from 'i18n/i18nList';
import Hero from 'views/public/home/Hero';
import { STATIC_TOKENS } from './tokens';

// ==============================|| STATIC HOME (prerendered) ||============================== //
// The page the server returns at `/`: rendered to plain HTML at build time, one file per language,
// and shipped without the app bundle so search engines and first-time visitors get the content
// straight away. It never reads the login state: the header always offers "Console", and the panel
// sends visitors who are not signed in through login.
//
// Nothing here may depend on client-side React. The language menu is a <details> element with
// plain links, and the theme button is wired by a small inline script (see scripts/prerender-home.mjs).

const escapeText = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

// The footer's docs link, between markers the server uses to drop it when no docs link is set.
function docsLinkMarkup(label) {
  const { docsStart, docsEnd, docsUrl } = STATIC_TOKENS;
  return `${docsStart}<a href="${docsUrl}" target="_blank" rel="noopener noreferrer" class="text-[0.8125rem] leading-none hover:text-foreground hover:underline">${escapeText(label)}</a>${docsEnd}`;
}

function LanguageMenu({ lng }) {
  const { t } = useTranslation();
  const current = i18nList.find((item) => item.lng === lng) || i18nList[0];

  return (
    <details className="group relative">
      <summary
        aria-label={t('language.select', { defaultValue: 'Select language' })}
        className="inline-flex h-8 cursor-pointer list-none items-center gap-1.5 rounded-lg px-2 text-sm font-medium hover:bg-muted [&::-webkit-details-marker]:hidden"
      >
        <Globe className="size-5" />
        {current.shortCode}
      </summary>
      <div className="absolute right-0 z-50 mt-1 min-w-40 rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-md">
        {i18nList.map((item) => (
          <a
            key={item.lng}
            href={`/?lang=${item.lng}`}
            hrefLang={item.lng.replace('_', '-')}
            lang={item.lng.replace('_', '-')}
            className={`flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-muted ${item.lng === lng ? 'font-medium' : ''}`}
          >
            <span className="flex w-4 justify-center text-foreground">{item.lng === lng && <Check className="size-4" />}</span>
            {item.name}
          </a>
        ))}
      </div>
    </details>
  );
}

LanguageMenu.propTypes = { lng: PropTypes.string.isRequired };

export default function StaticHome({ lng }) {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="flex h-14 items-center justify-between gap-2 border-b border-border px-4 sm:px-6">
        <a href="/" className="flex items-center gap-2 text-foreground">
          <LogoMark size={20} />
          <span className="text-sm font-semibold leading-none">{STATIC_TOKENS.wordmark}</span>
        </a>
        <div className="flex items-center gap-1">
          <LanguageMenu lng={lng} />
          <Button variant="ghost" size="icon" aria-label="Toggle theme" data-theme-toggle="">
            <Moon className="size-5 dark:hidden" />
            <Sun className="hidden size-5 dark:block" />
          </Button>
          <Button asChild size="sm" className="ml-1">
            <a href="/panel/dashboard">{t('home.cta.console')}</a>
          </Button>
        </div>
      </header>

      <main className="flex-1">
        <div className="bg-background">
          <Hero />
        </div>
      </main>

      <footer className="flex items-center justify-center gap-1.5 px-4 py-5 text-muted-foreground">
        <LogoMark size={12} />
        <span className="text-[0.8125rem] font-medium leading-none">{STATIC_TOKENS.brand}</span>
        <span className="text-[0.8125rem] leading-none">© 2026</span>
        {/* comments cannot be rendered as React nodes; the markup is a constant plus escaped text */}
        <span className="contents" dangerouslySetInnerHTML={{ __html: docsLinkMarkup(t('common.docs')) }} />
      </footer>
    </div>
  );
}

StaticHome.propTypes = { lng: PropTypes.string.isRequired };
