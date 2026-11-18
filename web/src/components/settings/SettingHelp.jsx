import { useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import {
  OBJECT_PAGES,
  catalogEntry,
  settingDescription,
  settingEnumDescription,
  settingEnumLabel,
  settingExamples,
  settingLabel,
  splitFirstSentence
} from './catalog';
import { admin } from 'routes/paths';

// ==============================|| SETTINGS — FIELD HELP FROM THE CATALOG ||============================== //
// The text of a setting as the settings catalog has it: the label and the first sentence for the
// field itself, the rest (more sentences, the values of an enum, examples, related settings) for
// the help a `?` next to the label opens.

// Where a setting is set: a platform setting's field, or the page of the object it belongs to.
export function settingHref(key) {
  const entry = catalogEntry(key);
  if (!entry) return null;
  if (entry.scope === 'site') return `${admin.settings(entry.group)}?highlight=${encodeURIComponent(entry.key)}`;
  return OBJECT_PAGES[entry.group] || null;
}

export function SettingHelpBody({ settingKey, full = false }) {
  const { t } = useTranslation();
  const entry = catalogEntry(settingKey);
  if (!entry) return null;
  const description = settingDescription(t, settingKey);
  const text = full ? description : splitFirstSentence(description).rest;
  const examples = settingExamples(t, settingKey);
  const values = entry.type === 'enum' ? (entry.enum || []).filter((e) => e.description) : [];
  return (
    <div className="space-y-2">
      {text && <p>{text}</p>}
      {values.length > 0 && (
        <ul className="space-y-0.5">
          {values.map((e) => (
            <li key={e.value}>
              <span className="font-medium text-foreground">{settingEnumLabel(t, settingKey, e.value)}</span>
              {' — '}
              {settingEnumDescription(t, settingKey, e.value)}
            </li>
          ))}
        </ul>
      )}
      {examples.length > 0 && (
        <div className="space-y-0.5">
          <p className="font-medium text-foreground">{t('settingsUi.catalog.examples')}</p>
          <ul className="space-y-0.5">
            {examples.map((example) => (
              <li key={example.value}>
                <code className="rounded bg-muted px-1 font-mono text-[11px] text-foreground">{example.value}</code> {example.effect}
              </li>
            ))}
          </ul>
        </div>
      )}
      {entry.related?.length > 0 && (
        <p>
          <span className="font-medium text-foreground">{t('settingsUi.catalog.related')}</span>{' '}
          {entry.related.map((key, i) => {
            const href = settingHref(key);
            const label = settingLabel(t, key) || key;
            return (
              <span key={key}>
                {i > 0 && ' · '}
                {href ? (
                  <Link to={href} className="text-foreground underline underline-offset-2 hover:text-primary">
                    {label}
                  </Link>
                ) : (
                  label
                )}
              </span>
            );
          })}
        </p>
      )}
      <p className="font-mono text-[11px] text-muted-foreground/80">{entry.key}</p>
    </div>
  );
}

SettingHelpBody.propTypes = { settingKey: PropTypes.string.isRequired, full: PropTypes.bool };

// A `?` toggle and the help it opens, for rows that draw their own label (switch rows).
export function SettingHelpToggle({ settingKey, full, className }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  if (!catalogEntry(settingKey)) return null;
  return (
    <>
      <button
        type="button"
        aria-expanded={open}
        aria-label={t('settingsUi.catalog.help')}
        onClick={() => setOpen((o) => !o)}
        className={cn(
          'inline-flex size-4 shrink-0 items-center justify-center rounded-full border border-border align-middle text-[10px] font-medium leading-none text-muted-foreground transition-colors hover:border-foreground hover:text-foreground',
          className
        )}
      >
        ?
      </button>
      {open && (
        <div className="basis-full rounded-md bg-muted/50 px-3 py-2 text-xs leading-relaxed text-muted-foreground">
          <SettingHelpBody settingKey={settingKey} full={full} />
        </div>
      )}
    </>
  );
}

SettingHelpToggle.propTypes = { settingKey: PropTypes.string.isRequired, full: PropTypes.bool, className: PropTypes.string };

// Whether a setting's help has anything beyond its first sentence.
export function hasMoreHelp(t, key, full) {
  const entry = catalogEntry(key);
  if (!entry) return false;
  const rest = full ? settingDescription(t, key) : splitFirstSentence(settingDescription(t, key)).rest;
  return Boolean(rest || entry.examples?.length || entry.related?.length || entry.enum?.some((e) => e.description));
}

// A gray dot next to a label: the setting differs from its default.
export function ModifiedDot({ defaultValue }) {
  const { t } = useTranslation();
  const title = t('settingsUi.catalog.modifiedTitle', { value: defaultValue === '' ? t('settingsUi.catalog.empty') : defaultValue });
  return (
    <span
      role="img"
      aria-label={title}
      title={title}
      data-testid="setting-modified"
      className="ml-1.5 inline-block size-1.5 shrink-0 rounded-full bg-muted-foreground/60 align-middle"
    />
  );
}

ModifiedDot.propTypes = { defaultValue: PropTypes.string };
