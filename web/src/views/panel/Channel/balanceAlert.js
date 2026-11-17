// Pure helpers for the low-balance notice on the channel's monitoring tab: whether the channel
// has a balance to watch at all, the currency it is in, and what keeps a notice from arriving.
// The subscription usage alerts share the problems, links and target names.

import { settingsGroupUrl } from '../Setting/registry';

const CHANNEL_TYPE_OPENAI = 1;
const OFFICIAL_OPENAI_HOSTS = ['api.openai.com'];

// DeepSeek and SiliconFlow report their balance in CNY, and the threshold is compared as is.
const CNY_BALANCE_TYPES = new Set([28, 45]);

export const balanceCurrency = (type) => (CNY_BALANCE_TYPES.has(type) ? 'CNY' : 'USD');

function isOfficialOpenAI(baseUrl) {
  const raw = String(baseUrl || '').trim();
  if (!raw) return true;
  try {
    return OFFICIAL_OPENAI_HOSTS.includes(new URL(raw).hostname);
  } catch {
    return false;
  }
}

// What a low-balance notice can watch on this channel:
//   'balance'  the upstream reports a balance
//   'usage'    a subscription channel: it reports usage windows, not a balance
//   'official' OpenAI's own API, which has no balance endpoint (relays usually do)
//   'none'     the provider has no balance endpoint
//   null       not known yet (capabilities still loading)
export function balanceSupport(channel, caps) {
  if (!caps?.types) return null;
  const type = Number(channel?.type);
  if (caps.types.usage_windows?.includes(type)) return 'usage';
  if (!caps.types.balance?.includes(type)) return 'none';
  if (type === CHANNEL_TYPE_OPENAI && isOfficialOpenAI(channel?.base_url)) return 'official';
  return 'balance';
}

// Why a notice would not arrive even with a threshold set, most important first.
export function noticeProblems(lowBalance) {
  if (!lowBalance) return [];
  const problems = [];
  if (!lowBalance.refresh_minutes) problems.push('noRefresh');
  if (!lowBalance.kind_on) problems.push('kindOff');
  if (!(lowBalance.targets || []).some((target) => target.ready)) problems.push('noTarget');
  return problems;
}

// Where each problem is fixed: the notice kinds, or the SMTP card for email. The refresh
// interval lives in the config file, so it has no link.
export const PROBLEM_LINKS = {
  kindOff: `${settingsGroupUrl('notify')}?highlight=notifications`,
  noTarget: `${settingsGroupUrl('notify')}?highlight=notify-channels`
};

// How a notice target reads in the "alerts go to" line: its name (or its type) and address.
export function noticeTargetName(t, target) {
  const name = target.name || t(`settingsUi.platform.notifyChannels.types.${target.type}`, { defaultValue: target.type });
  const label = target.detail ? `${name} ${target.detail}` : name;
  return target.ready ? label : t('channels.monitor.targetNotReady', { name: label });
}
