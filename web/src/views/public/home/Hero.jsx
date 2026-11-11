import { Link } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { brandName } from 'utils/brand';
import DripFlow from './DripFlow';

// ==============================|| PUBLIC HOME — HERO ||============================== //
// The whole landing page for now: the drip on top as the poster, then the slogan, a short
// catchphrase and one action, centred between the header and the footer. No feature list on purpose.
// The text rises in while the first drop gathers (drip-rise lives in DripFlow's scoped styles).

const RISE = 'motion-safe:animate-[drip-rise_700ms_cubic-bezier(0.22,1,0.36,1)_both]';

export default function Hero() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);

  return (
    <section className="flex min-h-[calc(100svh-7.5rem)] items-center">
      <div className="mx-auto flex w-full max-w-5xl flex-col items-center px-4 py-12 text-center sm:px-6">
        <div className="w-full max-w-2xl">
          <DripFlow />
        </div>

        <h1
          className={`mt-10 text-balance break-keep text-4xl font-semibold leading-tight tracking-[-0.02em] sm:mt-12 sm:text-6xl ${RISE}`}
          style={{ animationDelay: '900ms' }}
        >
          {t('home.hero.tagline')}
        </h1>
        <p className={`mt-4 text-base text-muted-foreground sm:text-lg ${RISE}`} style={{ animationDelay: '1000ms' }}>
          {t('home.hero.catchphrase')}
        </p>
        {/* A plain link with no login state, so the prerendered static home works unchanged: the
            panel's AuthGuard sends visitors through login and back to the API-key page. */}
        <Button asChild size="lg" className={`mt-8 h-12 px-6 ${RISE}`} style={{ animationDelay: '1100ms' }}>
          <Link to="/panel/token">
            {t('home.hero.ctaPrimary')}
            <ArrowRight />
          </Link>
        </Button>

        <p className="sr-only">{brandName(siteInfo.system_name)}</p>
      </div>
    </section>
  );
}
