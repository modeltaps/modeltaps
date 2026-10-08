import { useTranslation } from 'react-i18next';
import { RefreshCw, RadioTower, ScrollText } from 'lucide-react';

// ==============================|| PUBLIC HOME — TRUST BAND ||============================== //
// Three short reassurance columns before the final CTA: daily price sync, model drift
// detection, and per-call traceability. Icon + title + one line each, no cards, so the
// band reads lighter than the capability sections above it.

const SIGNALS = [
  { key: 'pricing', Icon: RefreshCw },
  { key: 'drift', Icon: RadioTower },
  { key: 'logs', Icon: ScrollText }
];

export default function TrustBand() {
  const { t } = useTranslation();

  return (
    <section className="border-b border-border">
      <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6 sm:py-20">
        <div className="mx-auto max-w-2xl text-center">
          {/* break-keep so CJK titles only wrap after punctuation; break-words is the overflow fallback. */}
          <h2 className="text-balance break-keep break-words text-3xl font-semibold leading-snug sm:text-4xl">{t('home.trust.title')}</h2>
        </div>

        <div className="mt-12 grid grid-cols-1 gap-8 md:grid-cols-3">
          {SIGNALS.map(({ key, Icon }) => (
            <div key={key} className="text-center md:text-left">
              <span className="inline-flex size-10 items-center justify-center rounded-lg bg-muted text-foreground">
                <Icon className="size-5" aria-hidden="true" />
              </span>
              <h3 className="mt-3.5 text-base font-semibold text-foreground">{t(`home.trust.${key}.title`)}</h3>
              <p className="mt-1.5 text-[15px] leading-relaxed text-muted-foreground">{t(`home.trust.${key}.desc`)}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
