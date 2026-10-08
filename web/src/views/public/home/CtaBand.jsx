import { Link } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { brandName } from 'utils/brand';

// ==============================|| PUBLIC HOME — FINAL CTA BAND ||============================== //
// Closing conversion tile: a neutral solid dark card with dual CTAs. Primary → /panel/token
// (AuthGuard bounces guests to /login, matching Hero's token-page rule); secondary → /price.

export default function CtaBand() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const brand = brandName(siteInfo.system_name);

  return (
    <section>
      <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6 sm:py-20">
        <div className="relative overflow-hidden rounded-3xl bg-primary px-6 py-16 text-center shadow-lg sm:px-10 sm:py-20 dark:bg-popover">
          <h2 className="text-balance text-3xl font-semibold leading-snug text-white sm:text-4xl">{t('home.finalCta.title')}</h2>
          <p className="mx-auto mt-4 max-w-xl text-pretty text-base leading-relaxed text-white/90 sm:text-lg">
            {t('home.finalCta.subtitle', { brand })}
          </p>

          <div className="mt-8 flex w-full flex-col justify-center gap-3 sm:w-auto sm:flex-row">
            <Button asChild size="lg" className="bg-white text-primary hover:bg-white/90 sm:w-48 dark:text-primary-foreground">
              <Link to="/panel/token">
                {t('home.finalCta.primary')}
                <ArrowRight />
              </Link>
            </Button>
            <Button
              asChild
              size="lg"
              variant="outline"
              className="border-white/40 bg-white/15 text-white hover:bg-white/25 hover:text-white sm:w-48"
            >
              <Link to="/price">{t('home.finalCta.secondary')}</Link>
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}
