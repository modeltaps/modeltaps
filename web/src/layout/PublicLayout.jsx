import { Outlet, Link } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Moon, Sun } from 'lucide-react';

import useTheme from 'hooks/useTheme';
import { Button } from '@/components/ui/button';
import { LogoMark } from '@/components/chrome/Logo';
import UserMenu from '@/components/chrome/UserMenu';
import LanguageSwitcher from '@/components/auth/LanguageSwitcher';
import Footer from '@/components/chrome/Footer';
import { brandName } from 'utils/brand';

// ==============================|| PUBLIC LAYOUT (shadcn) ||============================== //
// Header + footer chrome for unauthenticated/public pages (OAuth callbacks,
// About, 404, Jump, Playground, model price). Mirrors the auth shell tone.

export default function PublicLayout() {
  const { t } = useTranslation();
  const { resolved, cycle } = useTheme();
  const siteInfo = useSelector((state) => state.siteInfo);
  const account = useSelector((state) => state.account);
  const dark = resolved === 'dark';

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="flex items-center justify-between gap-2 border-b border-border px-4 py-3 sm:px-6">
        <Link to="/" className="flex items-center gap-2 text-foreground">
          <LogoMark className="h-5 w-auto shrink-0" />
          <span className="text-sm font-semibold leading-none">{brandName(siteInfo.system_name)}</span>
        </Link>
        <div className="flex items-center gap-1">
          <LanguageSwitcher />
          <Button variant="ghost" size="icon" aria-label="Toggle theme" onClick={cycle}>
            {dark ? <Sun className="size-5" /> : <Moon className="size-5" />}
          </Button>
          {account.user ? (
            <div className="ml-1">
              <UserMenu />
            </div>
          ) : (
            <>
              <Button asChild variant="ghost" size="sm" className="ml-1 hidden sm:inline-flex">
                <Link to="/register">{t('home.cta.signup')}</Link>
              </Button>
              <Button asChild size="sm">
                <Link to="/login">{t('home.cta.login')}</Link>
              </Button>
            </>
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
