import { useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { brandName } from 'utils/brand';
import OrgGatewayAnimation from './OrgGatewayAnimation';

// ==============================|| PUBLIC HOME — HERO ||============================== //
// The whole landing page for now: one line, one action and the gateway animation, centred in the
// space between the header and the footer. No feature list on purpose.

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
        <h1 className="text-balance break-keep text-4xl font-semibold leading-tight tracking-[-0.02em] sm:text-6xl">
          {t('home.hero.tagline')}
        </h1>
        <Button size="lg" className="mt-8 h-12 px-6" onClick={handleGetKey}>
          {t('home.hero.ctaPrimary')}
          <ArrowRight />
        </Button>

        <div className="mt-12 w-full max-w-2xl sm:mt-14">
          <OrgGatewayAnimation />
        </div>

        <p className="sr-only">{brandName(siteInfo.system_name)}</p>
      </div>
    </section>
  );
}
