import { Outlet, Link } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Moon, Sun } from 'lucide-react';

import useTheme from 'hooks/useTheme';
import { Button } from '@/components/ui/button';
import { BrandLockup } from '@/components/chrome/Logo';
import UserMenu from '@/components/chrome/UserMenu';
import LanguageSwitcher from '@/components/auth/LanguageSwitcher';
import Footer from '@/components/chrome/Footer';
import { brandWordmark, isDefaultBrand } from 'utils/brand';
import { workbench } from 'routes/paths';

// ==============================|| PUBLIC LAYOUT (shadcn) ||============================== //
// Header + footer chrome for unauthenticated/public pages (OAuth callbacks,
// About, 404, Jump, Playground, model price). Mirrors the auth shell tone.

export default function PublicLayout() {
  const { t } = useTranslation();
  const { resolved, cycle } = useTheme();
  const siteInfo = useSelector((state) => state.siteInfo);
  const account = useSelector((state) => state.account);
  const dark = resolved === 'dark';
  // Docs: the configured external link, else the in-app API docs for signed-in users.
  const docsExternal = Boolean(siteInfo.docs_link);
  const docsHref = siteInfo.docs_link || (account.user ? workbench.docs() : '');

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="flex h-14 items-center justify-between gap-4 border-b border-border px-4 sm:px-6">
        <div className="flex items-center gap-6">
          <Link to="/" reloadDocument className="flex items-center">
            <BrandLockup text={brandWordmark(siteInfo.system_name)} wordmark={isDefaultBrand(siteInfo.system_name)} />
          </Link>
          <nav className="hidden items-center gap-5 text-sm text-muted-foreground sm:flex">
            <Link to="/pricing" className="transition-colors hover:text-foreground">
              {t('pricing')}
            </Link>
            {docsHref &&
              (docsExternal ? (
                <a href={docsHref} target="_blank" rel="noopener noreferrer" className="transition-colors hover:text-foreground">
                  {t('common.docs')}
                </a>
              ) : (
                <Link to={docsHref} className="transition-colors hover:text-foreground">
                  {t('common.docs')}
                </Link>
              ))}
          </nav>
        </div>
        <div className="flex items-center gap-0.5">
          <LanguageSwitcher iconOnly />
          <Button
            variant="ghost"
            size="icon"
            aria-label="Toggle theme"
            className="text-muted-foreground hover:text-foreground"
            onClick={cycle}
          >
            {dark ? <Sun className="size-[18px]" /> : <Moon className="size-[18px]" />}
          </Button>
          <span aria-hidden className="mx-2 h-5 w-px bg-border" />
          {account.user ? (
            <div className="flex items-center gap-3">
              <Button asChild size="sm" className="hidden h-8 px-3 sm:inline-flex">
                <Link to={workbench.dashboard()}>{t('home.cta.console')}</Link>
              </Button>
              <UserMenu />
            </div>
          ) : (
            <Button asChild size="sm" className="h-8 px-3">
              <Link to="/login">{t('home.cta.login')}</Link>
            </Button>
          )}
        </div>
      </header>

      <main className="flex-1">
        <Outlet />
      </main>

      <Footer />
    </div>
  );
}
