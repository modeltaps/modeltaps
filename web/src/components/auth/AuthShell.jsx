import { useContext, useEffect, useState } from 'react';
import { useSelector } from 'react-redux';
import { useNavigate } from 'react-router';
import { Moon, Sun } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import LanguageSwitcher from '@/components/auth/LanguageSwitcher';
import { LogoMark } from '@/components/chrome/Logo';
import { UserContext } from 'contexts/UserContext';
import { useTranslation } from 'react-i18next';

// ==============================|| AUTH SHELL ||============================== //

export default function AuthShell({ title, children }) {
  const { t } = useTranslation();
  const account = useSelector((state) => state.account);
  const { isUserLoaded } = useContext(UserContext);
  const navigate = useNavigate();

  const [dark, setDark] = useState(() => {
    const stored = localStorage.getItem('theme');
    if (stored) return stored === 'dark';
    if (document.documentElement.classList.contains('dark')) return true;
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  });

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle('dark', dark);
    root.classList.toggle('light', !dark);
  }, [dark]);

  const toggleTheme = () => {
    setDark((v) => {
      const next = !v;
      localStorage.setItem('theme', next ? 'dark' : 'light');
      return next;
    });
  };

  // Mirror v1 AuthWrapper: redirect already-authenticated users to the panel.
  useEffect(() => {
    if (isUserLoaded && account.user) {
      navigate('/panel');
    }
  }, [account, isUserLoaded, navigate]);

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="flex items-center justify-end gap-1 px-4 py-4 sm:px-6">
        <LanguageSwitcher />
        <Button variant="ghost" size="icon" aria-label="Toggle theme" onClick={toggleTheme}>
          {dark ? <Sun className="size-5" /> : <Moon className="size-5" />}
        </Button>
      </header>

      <main className="flex flex-1 items-center justify-center px-4 py-6">
        {!isUserLoaded ? (
          <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
        ) : (
          <Card className="w-full max-w-sm shadow-sm">
            <CardHeader className="items-center space-y-3 text-center">
              <LogoMark className="h-9 w-auto text-foreground" />
              <CardTitle className="text-2xl">{title}</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">{children}</CardContent>
          </Card>
        )}
      </main>
    </div>
  );
}
