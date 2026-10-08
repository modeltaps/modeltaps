import { useTranslation } from 'react-i18next';

// ==============================|| PUBLIC HOME — HOW IT WORKS ||============================== //
// Three-step path: add a channel → create a token → call one endpoint. Numbered cards with a
// neutral muted badge; all copy via i18n.

const STEPS = ['step1', 'step2', 'step3'];

export default function HowItWorks() {
  const { t } = useTranslation();

  return (
    <section className="border-b border-border">
      <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 sm:py-24">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-balance text-3xl font-semibold leading-snug sm:text-4xl">{t('home.howItWorks.title')}</h2>
          <p className="mt-4 text-pretty text-base leading-relaxed text-muted-foreground">{t('home.howItWorks.subtitle')}</p>
        </div>

        <ol className="mt-14 grid grid-cols-1 gap-6 lg:grid-cols-3">
          {STEPS.map((key, index) => (
            <li key={key} className="rounded-xl border border-border bg-card p-6 sm:p-7">
              <span className="inline-flex size-11 items-center justify-center rounded-xl bg-muted font-mono text-sm font-bold text-foreground">
                {String(index + 1).padStart(2, '0')}
              </span>
              <h3 className="mt-5 text-xl font-semibold text-foreground">{t(`home.howItWorks.${key}.title`)}</h3>
              <p className="mt-2 text-[15px] leading-7 text-muted-foreground">{t(`home.howItWorks.${key}.desc`)}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
