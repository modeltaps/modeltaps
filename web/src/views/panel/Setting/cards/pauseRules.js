// Client-side check of the PauseRules option before it is saved: empty (built-in rules) or a JSON
// array. The backend validates the rules themselves and answers with its own message.
export const pauseRulesError = (value) => {
  if (!value || !String(value).trim()) return null;
  try {
    return Array.isArray(JSON.parse(value)) ? null : 'notArray';
  } catch {
    return 'invalidJson';
  }
};

// The built-in rules as the editor text for "start from defaults".
export const pauseRulesText = (rules) => JSON.stringify(rules || [], null, 2);

// One rule in words, as i18n keys (under settingsUi.platform.pauseRules.summary) with params:
// { when: [...], then: [...] }. Conditions are listed in the order the backend checks them.
export function describePauseRule(rule = {}) {
  const when = [];
  if (rule.status_codes?.length) when.push(['status', { codes: rule.status_codes.join(' / ') }]);
  const patterns = (rule.keywords?.length || 0) + (rule.error_types?.length || 0) + (rule.error_codes?.length || 0);
  if (patterns) when.push(['patterns', { count: patterns }]);
  if (rule.unusable) when.push(['unusable']);
  if (rule.model_not_found) when.push(['modelNotFound']);
  if (rule.rate_limit_reset) when.push(['rateLimitReset']);
  if (rule.per_status_config) when.push(['perStatus']);
  if (!when.length) when.push(['anyError']);
  if (rule.times > 1) when.push(['times', { count: rule.times, seconds: rule.window_seconds || 0 }]);

  const then = [];
  if (rule.action === 'retry') {
    then.push(['retry']);
  } else {
    const scope = rule.scope === 'model' ? 'model' : 'channel';
    if (rule.seconds > 0) then.push([`pause_${scope}`, { seconds: rule.seconds }]);
    else if (rule.rate_limit_reset) then.push([`pauseUntilReset_${scope}`]);
    else then.push([`pauseConfigured_${scope}`]);
    if (rule.probe) then.push(['probe']);
  }
  if (rule.notify) then.push(['notify']);
  return { when, then };
}
