import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ArrowRight } from 'lucide-react';

import { Button } from '@/components/ui/button';

// ==============================|| PUBLIC HOME — CLOSING ROW ||============================== //
// One line and the same two actions as the hero. Primary → /panel/token (AuthGuard bounces guests to
// /login), secondary → /price.

export default function CtaBand() {
  const { t } = useTranslation();

  return (
    <section className="border-b border-border">
      <div className="mx-auto flex max-w-6xl flex-col items-start justify-between gap-8 px-4 py-20 sm:px-6 md:flex-row md:items-center lg:py-24">
        <h2 className="max-w-[20ch] text-balance break-keep text-3xl font-semibold leading-tight sm:text-4xl">
          {t('home.finalCta.title')}
        </h2>
        <div className="flex flex-col gap-3 sm:flex-row">
          <Button asChild size="lg" className="h-12 px-6">
            <Link to="/panel/token">
              {t('home.finalCta.primary')}
              <ArrowRight />
            </Link>
          </Button>
          <Button asChild size="lg" variant="ghost" className="h-12 px-5 text-muted-foreground hover:text-foreground">
            <Link to="/price">{t('home.finalCta.secondary')}</Link>
          </Button>
        </div>
      </div>
    </section>
  );
}
