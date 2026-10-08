import { useTranslation } from 'react-i18next';
import { Wallet, CalendarClock, BarChart3, Users } from 'lucide-react';

// ==============================|| PUBLIC HOME — ORG CONTROL ||============================== //
// Left: headline plus the four control capabilities (member budget, team period budget,
// usage analytics, roles & members). Right: a product mock of the member budget panel —
// per-member "remaining this period" bars over an organization-wide period budget bar.
// Monochrome product tokens only; member names are literal, figures are static mock data.

const POINTS = [
  { key: 'memberBudget', Icon: Wallet },
  { key: 'teamBudget', Icon: CalendarClock },
  { key: 'analytics', Icon: BarChart3 },
  { key: 'roles', Icon: Users }
];

// Static mock data — literal names and figures, deliberately not i18n.
const MEMBERS = [
  { name: 'Alice', used: '$32', limit: '$50', pct: 64 },
  { name: 'Bob', used: '$11', limit: '$50', pct: 22 },
  { name: 'Chen', used: '$46', limit: '$50', pct: 92 },
  { name: 'Dana', used: '$8', limit: '$30', pct: 27 }
];

const TEAM_BUDGET = { used: '$97', limit: '$500', pct: 19 };

function BudgetMock() {
  const { t } = useTranslation();

  return (
    <div className="rounded-xl border border-border bg-card p-6">
      <div className="flex items-baseline justify-between">
        <span className="text-sm font-semibold text-foreground">{t('home.control.mock.remaining')}</span>
        <span className="font-mono text-[11.5px] text-muted-foreground">{t('home.control.mock.used')}</span>
      </div>

      <ul className="mt-4 grid gap-3.5">
        {MEMBERS.map(({ name, used, limit, pct }) => (
          <li key={name} className="grid gap-1.5">
            <div className="flex items-baseline justify-between gap-2 text-[13px]">
              <span className="font-medium text-foreground">{name}</span>
              <span className="font-mono text-[11.5px] text-muted-foreground">
                {used} / {limit}
              </span>
            </div>
            <span className="block h-1.5 overflow-hidden rounded-full bg-muted">
              <span className="block h-full rounded-full bg-foreground" style={{ width: `${pct}%` }} />
            </span>
          </li>
        ))}
      </ul>

      <div className="mt-6 border-t border-border pt-5">
        <div className="flex items-baseline justify-between gap-2 text-[13px]">
          <span className="font-semibold text-foreground">{t('home.control.mock.teamBudget')}</span>
          <span className="font-mono text-[11.5px] text-muted-foreground">
            {TEAM_BUDGET.used} / {TEAM_BUDGET.limit}
          </span>
        </div>
        <span className="mt-1.5 block h-2 overflow-hidden rounded-full bg-muted">
          <span className="block h-full rounded-full bg-foreground" style={{ width: `${TEAM_BUDGET.pct}%` }} />
        </span>
      </div>
    </div>
  );
}

export default function OrgControlSection() {
  const { t } = useTranslation();

  return (
    <section className="border-b border-border">
      <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 sm:py-24">
        <div className="grid grid-cols-1 items-start gap-12 lg:grid-cols-2 lg:gap-16">
          <div>
            <h2 className="text-balance text-3xl font-semibold leading-snug sm:text-4xl">{t('home.control.title')}</h2>
            <p className="mt-4 text-pretty text-base leading-relaxed text-muted-foreground">{t('home.control.subtitle')}</p>

            <ul className="mt-10 grid gap-7">
              {POINTS.map(({ key, Icon }) => (
                <li key={key} className="flex gap-4">
                  <span className="inline-flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground">
                    <Icon className="size-5" aria-hidden="true" />
                  </span>
                  <div>
                    <h3 className="text-[15px] font-semibold text-foreground">{t(`home.control.${key}.title`)}</h3>
                    <p className="mt-1 text-[14.5px] leading-relaxed text-muted-foreground">{t(`home.control.${key}.desc`)}</p>
                  </div>
                </li>
              ))}
            </ul>
          </div>

          <BudgetMock />
        </div>
      </div>
    </section>
  );
}
