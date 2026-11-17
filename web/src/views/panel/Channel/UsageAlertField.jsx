import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { AlertTriangle } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { Field } from '@/components/objects';
import { timestamp2string } from 'utils/common';
import { PROBLEM_LINKS, noticeProblems, noticeTargetName } from './balanceAlert';
import { effectiveUsageTiers, formatTiers, parseUsageTiers, usageSource } from './usageAlert';

// The subscription usage alerts of the monitoring tab (Codex, Claude Code): the channel's own
// tiers, and for the tiers in force, how often the usage is checked, where the alerts go and
// what would keep them from arriving. `caps` is GET /api/channel/capabilities.
export default function UsageAlertField({ channel, caps, value, onChange }) {
  const { t } = useTranslation();
  const notice = caps?.usage_alert;
  const siteTiers = notice ? notice.tiers || [] : null;
  const { tiers, source } = effectiveUsageTiers(value, siteTiers);
  const on = tiers.length > 0;
  const invalid = Boolean(parseUsageTiers(value).error);
  const problems = on ? noticeProblems(notice) : [];
  const fetchedAt = channel.usage_windows?.fetched_at;
  const every = t(`channels.monitor.usage.every.${usageSource(channel.type)}`, { minutes: notice?.refresh_minutes });

  return (
    <Field
      label={t('channels.monitor.usage.label')}
      htmlFor="cp-usage-tiers"
      optional
      error={invalid ? t('channels.monitor.usage.invalid') : undefined}
    >
      <Input
        id="cp-usage-tiers"
        className="w-48 tabular-nums"
        value={value}
        placeholder={siteTiers && siteTiers.length === 0 ? t('channels.monitor.thresholdOff') : (siteTiers || []).join(', ')}
        onChange={(e) => onChange(e.target.value)}
      />
      <p className="mt-1.5 text-xs text-muted-foreground">{t('channels.monitor.usage.hint')}</p>
      {notice && (
        <div className="mt-1 space-y-1 text-xs text-muted-foreground">
          {on ? (
            <p>
              {t(`channels.monitor.usage.on.${source}`, { tiers: formatTiers(tiers) })}
              {notice.refresh_minutes > 0 && (
                <>
                  {' · '}
                  {every}
                </>
              )}
              {notice.targets?.length > 0 && (
                <>
                  {' · '}
                  {t('channels.monitor.noticeTo', {
                    targets: notice.targets.map((target) => noticeTargetName(t, target)).join(t('channels.monitor.targetSep'))
                  })}
                </>
              )}
            </p>
          ) : (
            <p>
              {t(`channels.monitor.usage.off.${source}`)}
              {source === 'site' && (
                <Link to={PROBLEM_LINKS.kindOff} className="ml-1 text-info hover:underline">
                  {t('channels.monitor.problemLink.kindOff')}
                </Link>
              )}
            </p>
          )}
          {on && fetchedAt > 0 && <p>{t('channels.monitor.usage.updatedAt', { time: timestamp2string(fetchedAt) })}</p>}
          {problems.map((problem) => (
            <p key={problem} className="flex items-start gap-1.5 text-warning">
              <AlertTriangle className="mt-px size-3.5 shrink-0" />
              <span>
                {t(problem === 'noTarget' ? 'channels.monitor.problem.noTarget' : `channels.monitor.usage.problem.${problem}`)}
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

UsageAlertField.propTypes = {
  channel: PropTypes.object.isRequired,
  caps: PropTypes.object,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired
};
