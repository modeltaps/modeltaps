import { useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { RotateCcw } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { ActionCard } from '@/components/settings/cards';
import { catalogEntries, settingEnumLabel, settingLabel } from '@/components/settings/catalog';
import { settingHref } from '@/components/settings/SettingHelp';
import { cardText } from './OptionCard';
import { cardTitleKey, groupLabelKey, visibleGroups } from '../registry';
import { isModified } from '../parts';
import { useIsRoot } from 'utils/common';

// ==============================|| PLATFORM SETTINGS — MODIFIED SETTINGS ||============================== //
// Every platform setting whose stored value differs from its default, with both values, a link to
// the field and a button that puts the default back (saved at once). Secrets are left out: their
// values never reach the browser. The default is the value the running instance started with, so
// a setting the deployment configuration changed counts as unmodified.

const MAX_SHOWN = 60;

// A value as a short line: enum labels from the catalog, long text cut.
function shown(t, entry, value) {
  const text = String(value ?? '');
  if (text === '') return t('settingsUi.catalog.empty');
  if (entry.type === 'enum') return settingEnumLabel(t, entry.key, text);
  if (entry.type === 'bool') return text === 'true' ? t('settingsUi.catalog.on') : t('settingsUi.catalog.off');
  return text.length > 60 ? `${text.slice(0, 57)}…` : text;
}

export function ModifiedSettingsCard({ ctx }) {
  const { t } = useTranslation();
  const isRoot = useIsRoot();
  const [busy, setBusy] = useState('');
  const groups = new Set(visibleGroups(isRoot).map((g) => g.id));
  const meta = { defaults: ctx.defaults, origin: ctx.origin };
  const rows = catalogEntries().filter((e) => e.scope === 'site' && e.type !== 'secret' && groups.has(e.group) && isModified(meta, e.key));

  const restore = async (key) => {
    setBusy(key);
    try {
      await ctx.saveKeys([key], { values: { [key]: ctx.defaults[key] ?? '' } });
    } finally {
      setBusy('');
    }
  };

  return (
    <ActionCard
      title={cardText(t, 'modified-settings', 'title')}
      description={t('settingsUi.catalog.modifiedCount', { count: rows.length })}
      highlight="modified-settings"
    >
      {rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('settingsUi.catalog.noneModified')}</p>
      ) : (
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border">
          {rows.slice(0, MAX_SHOWN).map((entry) => (
            <li key={entry.key} className="flex items-center gap-3 px-3 py-2 text-sm" data-testid="modified-setting">
              <div className="min-w-0 flex-1 space-y-0.5">
                <Link to={settingHref(entry.key)} className="font-medium text-foreground hover:underline">
                  {settingLabel(t, entry.key)}
                </Link>
                <p className="truncate text-xs text-muted-foreground">
                  {t(groupLabelKey(entry.group))} › {t(cardTitleKey(entry.card), { defaultValue: entry.card })} ·{' '}
                  <span className="font-mono">{entry.key}</span>
                </p>
                <p className="truncate text-xs text-muted-foreground">
                  {t('settingsUi.catalog.nowAndDefault', {
                    now: shown(t, entry, ctx.origin[entry.key]),
                    value: shown(t, entry, ctx.defaults[entry.key])
                  })}
                </p>
              </div>
              <Button variant="outline" size="sm" className="rounded-lg" disabled={busy === entry.key} onClick={() => restore(entry.key)}>
                <RotateCcw />
                {t('settingsUi.catalog.restore')}
              </Button>
            </li>
          ))}
        </ul>
      )}
    </ActionCard>
  );
}

ModifiedSettingsCard.propTypes = { ctx: PropTypes.object.isRequired };
