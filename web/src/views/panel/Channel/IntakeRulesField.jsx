import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Field } from '@/components/objects';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import IntakeRulesEditor, { IntakeRuleList, intakeRulesJSON, parseIntakeRules } from '../ModelInfo/supply/IntakeRules';
import { fetchIntakeRules } from '../ModelInfo/supply/supplyApi';

// The `supply_intake_rules` part of a channel save: `{}` when unchanged (the backend then keeps
// the stored value), `{ supply_intake_rules }` when changed ("" goes back to the site rules).
export function intakeRulesPatch(draftValue, channel) {
  const value = draftValue || '';
  return value === (channel.supply_intake_rules || '') ? {} : { supply_intake_rules: value };
}

// Which intake rules a channel uses for new upstream models: the site-wide ones (shown read-only)
// or its own copy, edited here. `value` is the stored JSON, '' for the site rules.
export default function IntakeRulesField({ channelId, value, onChange }) {
  const { t } = useTranslation();
  const [site, setSite] = useState(null);
  useEffect(() => {
    let live = true;
    fetchIntakeRules(channelId)
      .then((res) => live && res.success && setSite(res.data?.rules || []))
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [channelId]);

  const own = parseIntakeRules(value);
  const mode = own ? 'custom' : 'site';
  const choose = (next) => {
    if (next === mode) return;
    // A channel's own rules start as a copy of the site ones.
    onChange(next === 'custom' ? intakeRulesJSON(site || []) : '');
  };

  return (
    <Field label={t('intakeRules.channel.label')} hint={t('intakeRules.channel.hint')}>
      <div className="space-y-2">
        <Select value={mode} onValueChange={choose}>
          <SelectTrigger className="h-9 w-56" aria-label={t('intakeRules.channel.label')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="site">{t('intakeRules.channel.site')}</SelectItem>
            <SelectItem value="custom" disabled={!site}>
              {t('intakeRules.channel.custom')}
            </SelectItem>
          </SelectContent>
        </Select>
        {own ? (
          <IntakeRulesEditor rules={own} onChange={(next) => onChange(intakeRulesJSON(next))} />
        ) : (
          site && <IntakeRuleList rules={site} />
        )}
      </div>
    </Field>
  );
}

IntakeRulesField.propTypes = {
  channelId: PropTypes.number.isRequired,
  value: PropTypes.string.isRequired,
  onChange: PropTypes.func.isRequired
};
