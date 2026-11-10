import { useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { brandName } from 'utils/brand';
import GatewayFlow from './GatewayFlow';

// ==============================|| PUBLIC HOME — HERO ||============================== //
// The whole landing page for now: the gateway flow on top as the poster, then the slogan, a short
// catchphrase and one action, centred between the header and the footer. No feature list on purpose.
// The text rises in once the flow has drawn itself (gf-rise lives in GatewayFlow's scoped styles).

const RISE = 'motion-safe:animate-[gf-rise_700ms_cubic-bezier(0.22,1,0.36,1)_both]';

export default function Hero() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const siteInfo = useSelector((state) => state.siteInfo);
  const account = useSelector((state) => state.account);

  // Signed-in users go straight to the API-key page; visitors detour through login and come back
  // via ?redirect=.
  const handleGetKey = () => {
    navigate(account?.user ? '/panel/token' : `/login?redirect=${encodeURIComponent('/panel/token')}`);
  };

  return (
    <section className="flex min-h-[calc(100svh-7.5rem)] items-center">
      <div className="mx-auto flex w-full max-w-5xl flex-col items-center px-4 py-12 text-center sm:px-6">
        <div className="w-full max-w-2xl">
          <GatewayFlow />
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
        <Button size="lg" className={`mt-8 h-12 px-6 ${RISE}`} style={{ animationDelay: '1100ms' }} onClick={handleGetKey}>
          {t('home.hero.ctaPrimary')}
          <ArrowRight />
        </Button>

        <p className="sr-only">{brandName(siteInfo.system_name)}</p>
      </div>
    </section>
  );
}
