import { useTranslation } from 'react-i18next';

// ==============================|| PUBLIC HOME — FACTS ||============================== //
// Three plain columns under the hero: the native protocols, where models come from, and why the
// numbers can be trusted. Text only, separated by hairlines, so the hero stays the one rich moment.

const FACTS = ['protocols', 'sources', 'ledger'];

export default function FactsSection() {
  const { t } = useTranslation();

  return (
    <section className="border-b border-border">
      <div className="mx-auto grid max-w-6xl grid-cols-1 px-4 sm:px-6 md:grid-cols-3">
        {FACTS.map((key, index) => (
          <div
            key={key}
            className={
              index === 0
                ? 'py-12 md:py-16 md:pr-8'
                : 'border-t border-border py-12 md:border-l md:border-t-0 md:px-8 md:py-16 [&:last-child]:md:pr-0'
            }
          >
            <p className="font-mono text-[12px] text-muted-foreground">{t(`home.facts.${key}.label`)}</p>
            <h2 className="mt-3 text-xl font-semibold leading-snug text-foreground">{t(`home.facts.${key}.title`)}</h2>
            <p className="mt-3 text-[15px] leading-relaxed text-muted-foreground">{t(`home.facts.${key}.desc`)}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
