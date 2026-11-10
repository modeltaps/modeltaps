import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';

// ==============================|| PUBLIC HOME — ORG CONTROL ||============================== //
// Left: what an organization gets, as a plain list. Right: a product view of the member budget
// panel — each member's spend this period against their own limit, over the organization's period
// budget. A member close to the limit turns amber; the organization bar carries the page accent.
// Names and figures are static mock data, deliberately not translated.

const POINTS = ['memberBudget', 'teamBudget', 'analytics', 'roles'];

const MEMBERS = [
  { name: 'Alice', used: 32.4, limit: 50 },
  { name: 'Bob', used: 11.08, limit: 50 },
  { name: 'Chen', used: 46.2, limit: 50 },
  { name: 'Dana', used: 7.9, limit: 30 }
];

const TEAM = { used: 97.58, limit: 500 };

const usd = (n) => `$${n.toFixed(2)}`;

function Bar({ pct, tone }) {
  return (
    <span className="block h-1.5 overflow-hidden rounded-full bg-muted">
      <span className={cn('block h-full rounded-full', tone)} style={{ width: `${Math.min(pct, 100)}%` }} />
    </span>
  );
}

Bar.propTypes = { pct: PropTypes.number.isRequired, tone: PropTypes.string.isRequired };

function BudgetPanel() {
  const { t } = useTranslation();
  const teamPct = (TEAM.used / TEAM.limit) * 100;

  return (
    <div className="rounded-2xl border border-border bg-card p-6 sm:p-8">
      <div className="flex items-baseline justify-between gap-4">
        <p className="text-sm font-medium text-foreground">{t('home.control.mock.title')}</p>
        <p className="text-[12px] tabular-nums text-muted-foreground">{t('home.control.mock.period')}</p>
      </div>

      <ul className="mt-6 grid gap-5">
        {MEMBERS.map(({ name, used, limit }) => {
          const pct = (used / limit) * 100;
          return (
            <li key={name} className="grid gap-2">
              <div className="flex items-baseline justify-between gap-2 text-[13px]">
                <span className="text-foreground">{name}</span>
                <span className="font-mono tabular-nums text-muted-foreground">
                  {usd(used)} <span className="text-muted-foreground/60">/ {usd(limit)}</span>
                </span>
              </div>
              <Bar pct={pct} tone={pct >= 90 ? 'bg-warning' : 'bg-foreground/80'} />
            </li>
          );
        })}
      </ul>

      <div className="mt-8 border-t border-border pt-6">
        <div className="flex items-baseline justify-between gap-2 text-[13px]">
          <span className="font-medium text-foreground">{t('home.control.mock.teamBudget')}</span>
          <span className="font-mono tabular-nums text-muted-foreground">
            {usd(TEAM.used)} <span className="text-muted-foreground/60">/ {usd(TEAM.limit)}</span>
          </span>
        </div>
        <div className="mt-2">
          <Bar pct={teamPct} tone="bg-info" />
        </div>
      </div>
    </div>
  );
}

export default function OrgControlSection() {
  const { t } = useTranslation();

  return (
    <section className="border-b border-border">
      <div className="mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 px-4 py-20 sm:px-6 lg:grid-cols-2 lg:gap-16 lg:py-28">
        <div>
          <h2 className="text-balance break-keep text-3xl font-semibold leading-tight sm:text-4xl">{t('home.control.title')}</h2>
          <p className="mt-4 max-w-[32rem] text-pretty text-base leading-relaxed text-muted-foreground">{t('home.control.subtitle')}</p>

          <dl className="mt-10 divide-y divide-border border-y border-border">
            {POINTS.map((key) => (
              <div key={key} className="grid gap-1 py-4 sm:grid-cols-[10rem_1fr] sm:gap-6">
                <dt className="text-[15px] font-medium text-foreground">{t(`home.control.${key}.title`)}</dt>
                <dd className="text-[15px] leading-relaxed text-muted-foreground">{t(`home.control.${key}.desc`)}</dd>
              </div>
            ))}
          </dl>
        </div>

        <BudgetPanel />
      </div>
    </section>
  );
}
