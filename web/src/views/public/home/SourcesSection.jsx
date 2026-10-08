import { useTranslation } from 'react-i18next';
import { Cloud, KeyRound, Laptop } from 'lucide-react';

// ==============================|| PUBLIC HOME — MODEL SOURCES ||============================== //
// Three equal-width cards describing where models can come from: platform-paid cloud
// providers, the organization's own keys, and member-hosted local nodes. Monochrome
// product tokens + Lucide icons only; no vendor logos. The local-node card is marked
// "coming soon" so the section never claims unshipped capability.

const SOURCES = [
  { key: 'cloud', Icon: Cloud },
  { key: 'byok', Icon: KeyRound },
  { key: 'node', Icon: Laptop, upcoming: true }
];

export default function SourcesSection() {
  const { t } = useTranslation();

  return (
    <section className="border-b border-border">
      <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 sm:py-24">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-balance text-3xl font-semibold leading-snug sm:text-4xl">{t('home.sources.title')}</h2>
        </div>

        <div className="mt-14 grid grid-cols-1 gap-4 md:grid-cols-3">
          {SOURCES.map(({ key, Icon, upcoming }) => (
            <article
              key={key}
              className="relative flex flex-col rounded-xl border border-border bg-card p-6 transition-colors hover:border-foreground/30 motion-reduce:transition-none"
            >
              {upcoming ? (
                <span className="absolute right-4 top-4 rounded-full border border-border px-2.5 py-0.5 text-[11.5px] font-semibold text-muted-foreground">
                  {t('home.sources.node.badge')}
                </span>
              ) : null}
              <span className="mb-3.5 inline-flex size-11 items-center justify-center rounded-lg bg-muted text-foreground">
                <Icon className="size-5" aria-hidden="true" />
              </span>
              <h3 className="text-lg font-semibold text-foreground">{t(`home.sources.${key}.title`)}</h3>
              <p className="mt-2 text-[15px] leading-relaxed text-muted-foreground">{t(`home.sources.${key}.desc`)}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
