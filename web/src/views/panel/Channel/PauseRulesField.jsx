import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Textarea } from '@/components/ui/textarea';
import { Field } from '@/components/objects';
import { pauseRulesError } from '../Setting/cards/pauseRules';
import { fetchPauseRules } from '../ModelInfo/supply/supplyApi';

const PLACEHOLDER = '[{ "name": "quota", "status_codes": [402], "action": "pause", "seconds": 1800 }]';

// Pretty-printed stored rules, so the draft reads like the site rules shown below it.
export function pauseRulesText(raw) {
  if (!raw || !String(raw).trim()) return '';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

// The `pause_rules` part of a channel save: `{}` when unchanged (the backend then keeps the stored
// value), `{ pause_rules }` when changed ("" clears it), or `{ error }` when it is not empty or a
// JSON array. The backend validates the rules themselves.
export function pauseRulesPatch(draftValue, channel, t) {
  const value = (draftValue || '').trim();
  const error = pauseRulesError(value);
  if (error) return { error: t(`channels.pauseRules.${error}`) };
  return value === pauseRulesText(channel.pause_rules).trim() ? {} : { pause_rules: value };
}

// Channel-level supply pause rules: a JSON list tried before the site rules, with the site rules
// in force shown read-only for reference.
export default function PauseRulesField({ channelId, value, onChange }) {
  const { t } = useTranslation();
  const [site, setSite] = useState(null);
  const error = pauseRulesError(value);

  useEffect(() => {
    let live = true;
    fetchPauseRules(channelId)
      .then((res) => live && res.success && setSite(res.data || null))
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [channelId]);

  const siteRules = Array.isArray(site?.rules) ? site.rules : [];
  return (
    <Field
      label={t('channels.pauseRules.label')}
      htmlFor="cp-pause-rules"
      optional
      hint={t('channels.pauseRules.hint')}
      error={error ? t(`channels.pauseRules.${error}`) : null}
    >
      <Textarea
        id="cp-pause-rules"
        rows={6}
        className="font-mono text-[13px]"
        placeholder={PLACEHOLDER}
        aria-invalid={Boolean(error)}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      {site && (
        <details className="mt-2 rounded-[10px] border border-border/60 bg-muted/40 px-3 py-2 text-xs">
          <summary className="cursor-pointer text-muted-foreground">
            {t('channels.pauseRules.siteRules', { count: siteRules.length })}
          </summary>
          {site.enabled === false && <p className="mt-2 text-warning">{t('channels.pauseRules.siteOff')}</p>}
          <pre data-testid="site-pause-rules" className="mt-2 max-h-64 overflow-auto whitespace-pre font-mono text-[12px] leading-relaxed">
            {JSON.stringify(siteRules, null, 2)}
          </pre>
        </details>
      )}
    </Field>
  );
}

PauseRulesField.propTypes = {
  channelId: PropTypes.number.isRequired,
  value: PropTypes.string.isRequired,
  onChange: PropTypes.func.isRequired
};
