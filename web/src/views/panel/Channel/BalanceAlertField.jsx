import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { AlertTriangle } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { Field } from '@/components/objects';
import { settingsGroupUrl } from '../Setting/registry';
import { fetchChannelCapabilities } from './channelApi';
import { balanceCurrency, balanceSupport, noticeProblems } from './balanceAlert';

// Where each problem is fixed: the notice kinds, or the SMTP card for email. The refresh
// interval lives in the config file, so it has no link.
const PROBLEM_LINKS = {
  kindOff: `${settingsGroupUrl('notify')}?highlight=notifications`,
  noTarget: `${settingsGroupUrl('notify')}?highlight=smtp`
};

// The low-balance notice of the monitoring tab. It says when the provider has no balance to
// watch, and for the ones that do, how often the balance is checked, where the notice goes and
// what would keep it from arriving.
export default function BalanceAlertField({ channel, value, onChange }) {
  const { t } = useTranslation();
  const [caps, setCaps] = useState(null);

  useEffect(() => {
    let alive = true;
    fetchChannelCapabilities()
      .then((data) => alive && setCaps(data))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  const support = balanceSupport(channel, caps);
  const watchable = support === 'balance' || support === null;
  const currency = balanceCurrency(Number(channel.type));
  const notice = caps?.low_balance;
  const problems = support === 'balance' ? noticeProblems(notice) : [];
  const targetName = (target) => {
    const name = t(`channels.monitor.target.${target.name}`, { defaultValue: target.name });
    const label = target.detail ? `${name} ${target.detail}` : name;
    return target.ready ? label : t('channels.monitor.targetNotReady', { name: label });
  };

  return (
    <Field label={t('channels.monitor.thresholdIn', { currency })} htmlFor="cp-threshold" optional>
      <Input
        id="cp-threshold"
        type="number"
        min="0"
        step="0.01"
        className="w-40 tabular-nums"
        value={value}
        disabled={!watchable}
        placeholder={t('channels.monitor.thresholdOff')}
        onChange={(e) => onChange(e.target.value)}
      />
      <p className="mt-1.5 text-xs text-muted-foreground">
        {watchable ? t('channels.monitor.thresholdHint') : t(`channels.monitor.thresholdSupport.${support}`)}
      </p>
      {support === 'balance' && notice && (
        <div className="mt-1 space-y-1 text-xs text-muted-foreground">
          {notice.refresh_minutes > 0 && notice.targets?.length > 0 && (
            <p>
              {t('channels.monitor.noticeEvery', { minutes: notice.refresh_minutes })}
              {' · '}
              {t('channels.monitor.noticeTo', {
                targets: notice.targets.map(targetName).join(t('channels.monitor.targetSep'))
              })}
            </p>
          )}
          {problems.map((problem) => (
            <p key={problem} className="flex items-start gap-1.5 text-warning">
              <AlertTriangle className="mt-px size-3.5 shrink-0" />
              <span>
                {t(`channels.monitor.problem.${problem}`)}
                {PROBLEM_LINKS[problem] && (
                  <Link to={PROBLEM_LINKS[problem]} className="ml-1 text-info hover:underline">
                    {t(`channels.monitor.problemLink.${problem}`)}
                  </Link>
                )}
              </span>
            </p>
          ))}
        </div>
      )}
    </Field>
  );
}

BalanceAlertField.propTypes = {
  channel: PropTypes.object.isRequired,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired
};
