import { Link, useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { LogoMark } from '@/components/chrome/Logo';
import { brandName } from 'utils/brand';
import OrgGatewayAnimation from './OrgGatewayAnimation';

// ==============================|| PUBLIC HOME — HERO ||============================== //
// Landing-only hero: eyebrow -> H1 -> subline -> dual CTA on the left, "one exit for the
// organization" figure on the right. Two columns at >=1120px, single column below.
// Scoped styling only via product shadcn tokens — no global token changes.

export default function Hero() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const siteInfo = useSelector((state) => state.siteInfo);
  const account = useSelector((state) => state.account);

  // Primary CTA targets the API-key page. Logged-in users go straight there; anonymous
  // visitors detour through login and are returned via ?redirect=.
  const handleGetKey = () => {
    if (account?.user) {
      navigate('/panel/token');
    } else {
      navigate(`/login?redirect=${encodeURIComponent('/panel/token')}`);
    }
  };

  return (
    <section className="relative overflow-hidden border-b border-border">
      <div className="mx-auto grid max-w-6xl grid-cols-1 items-center gap-10 px-4 py-16 sm:px-6 sm:py-20 min-[1120px]:grid-cols-2 min-[1120px]:gap-12">
        <div className="flex flex-col items-center text-center min-[1120px]:items-start min-[1120px]:text-left">
          <span className="inline-flex items-center gap-2 rounded-full border border-border bg-card/60 px-3 py-1 text-sm font-medium text-muted-foreground">
            <LogoMark className="h-3 w-auto shrink-0 text-muted-foreground" />
            {t('home.hero.badge')}
          </span>

          <h1 className="mt-5 max-w-[16ch] text-balance text-4xl font-semibold leading-[1.2] sm:text-5xl">{t('home.hero.tagline')}</h1>

          <p className="mt-5 max-w-xl text-pretty text-base leading-relaxed text-muted-foreground sm:text-lg">{t('home.hero.subline')}</p>

          <div className="mt-8 flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
            <Button size="lg" className="sm:w-48" onClick={handleGetKey}>
              {t('home.hero.ctaPrimary')}
              <ArrowRight />
            </Button>
            <Button asChild size="lg" variant="outline" className="sm:w-48">
              <Link to="/price">{t('home.hero.ctaSecondary')}</Link>
            </Button>
          </div>

          <p className="sr-only">{brandName(siteInfo.system_name)}</p>
        </div>

        <div className="w-full">
          <OrgGatewayAnimation />
        </div>
      </div>
    </section>
  );
}
