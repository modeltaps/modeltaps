import { describe, expect, it } from 'vitest';

import en from 'i18n/locales/modeltaps/en_US.json';
import ja from 'i18n/locales/modeltaps/ja_JP.json';
import zhCN from 'i18n/locales/modeltaps/zh_CN.json';
import zhHK from 'i18n/locales/modeltaps/zh_HK.json';
import { pauseRulesError } from './cards/pauseRules';
import { INVITE_CODE_URL, PLATFORM_GROUPS, cardById, cardKeys, resolveSettingsTarget, visibleGroups } from './registry';

// Every option key the platform settings pages could save before the regrouping (the theme
// pages and their cards). Each must still belong to exactly one card.
const LEGACY_OPTION_KEYS = [
  'About',
  'AccountSystem',
  'AdminLoginEnabled',
  'AnalyticsCode',
  'ApproximateTokenEnabled',
  'AutomaticDisableChannelEnabled',
  'AutomaticDisableChannelNotifyEnabled',
  'AutomaticEnableChannelEnabled',
  'BrandIconFaviconFetchEnabled',
  'BrandIconRegistry',
  'BrandIconSyncEnabled',
  'BuiltinChatEnabled',
  'CFWorkerImageKey',
  'CFWorkerImageUrl',
  'ChannelCredentialRefreshMinutes',
  'ChannelDisableThreshold',
  'ChannelFailErrorMessage',
  'ChannelFailErrorWrapEnabled',
  'ChannelMetricsEnabled',
  'ChannelMetricsRetentionDays',
  'ChannelTestIntervalMinutes',
  'ChannelTestMode',
  'ChatImageRequestProxy',
  'ChatLink',
  'ChatLinks',
  'ClaudeAPIEnabled',
  'ClaudeBudgetTokensPercentage',
  'ClaudeDefaultMaxTokens',
  'ClaudePromptCachingEnabled',
  'DisableChannelKeywords',
  'DisplayInCurrencyEnabled',
  'DisplayTokenStatEnabled',
  'DocsLink',
  'EmailCodeLoginEnabled',
  'EmailDomainRestrictionEnabled',
  'EmailDomainWhitelist',
  'EmailVerificationEnabled',
  'EmptyResponseBillingEnabled',
  'EnableSafe',
  'FingerprintPassThroughEnabled',
  'Footer',
  'GeminiAPIEnabled',
  'GeminiOpenThink',
  'GitHubClientId',
  'GitHubClientSecret',
  'GitHubOAuthEnabled',
  'GitHubOldIdCloseEnabled',
  'HomePageContent',
  'InviteCodeRegisterEnabled',
  'InviteRewardEnabled',
  'InviterRewardType',
  'InviterRewardValue',
  'LarkAuthEnabled',
  'LarkClientId',
  'LarkClientSecret',
  'LinuxDoClientId',
  'LinuxDoClientSecret',
  'LinuxDoOAuthDynamicTrustLevel',
  'LinuxDoOAuthEnabled',
  'LinuxDoOAuthLowestTrustLevel',
  'LinuxDoOAuthTrustLevelEnabled',
  'LogAutoDeleteDays',
  'LogAutoDeleteEnabled',
  'LogConsumeEnabled',
  'LogIODefaultUser',
  'LogIOEnabled',
  'LogRelayErrorEnabled',
  'Logo',
  'MaxPromptTokens',
  'MjNotifyEnabled',
  'ModelNameCaseInsensitiveEnabled',
  'Notice',
  'NotifyDisabledKinds',
  'NotifyKindLimitPerHour',
  'OrganizationDefaultMaxMembers',
  'OrganizationEnabled',
  'OrganizationLogIODefault',
  'OrganizationMaxPerUser',
  'OrganizationQuotaTransferEnabled',
  'PasskeyLoginEnabled',
  'PasswordLoginEnabled',
  'PasswordRegisterEnabled',
  'PaymentCallbackAddress',
  'PaymentMinAmount',
  'PaymentUSDRate',
  'PreConsumedOutputTokens',
  'PreConsumedQuota',
  'PriceSyncAutoApply',
  'QuotaForInvitee',
  'QuotaForInviter',
  'QuotaForNewUser',
  'QuotaPerUnit',
  'QuotaRemindEnabled',
  'QuotaRemindThreshold',
  'QuotaResetTimezone',
  'QuotaResetWeekStart',
  'RechargeDiscount',
  'RegisterEnabled',
  'RetryCooldownPerStatus',
  'RetryCooldownSeconds',
  'RetryTimeOut',
  'RetryTimes',
  'RoutingDecisionBillingEnabled',
  'SMTPAccount',
  'SMTPFrom',
  'SMTPPort',
  'SMTPServer',
  'SMTPTLSMode',
  'SMTPToken',
  'SafeKeyWords',
  'SafeToolName',
  'ServerAddress',
  'SessionAffinityEnabled',
  'SessionAffinityStrict',
  'SessionAffinityTTLSeconds',
  'SystemName',
  'TopUpLink',
  'TurnstileCheckEnabled',
  'TurnstileSecretKey',
  'TurnstileSiteKey',
  'UnifiedRequestResponseModelEnabled',
  'WeChatAccountQRCodeImageURL',
  'WeChatAuthEnabled',
  'WeChatServerAddress',
  'WeChatServerToken'
];

const allCards = PLATFORM_GROUPS.flatMap((group) => group.cards);

describe('platform settings registry', () => {
  it('has the eight groups in order', () => {
    expect(PLATFORM_GROUPS.map((g) => g.id)).toEqual(['site', 'auth', 'billing', 'payment', 'gateway', 'privacy', 'notify', 'orgpolicy']);
    expect(visibleGroups(false).map((g) => g.id)).not.toContain('orgpolicy');
    expect(visibleGroups(true).map((g) => g.id)).toContain('orgpolicy');
  });

  it('every legacy option key belongs to exactly one card', () => {
    const owners = {};
    for (const card of allCards) for (const key of card.keys) (owners[key] ||= []).push(card.id);
    const missing = LEGACY_OPTION_KEYS.filter((key) => !owners[key]);
    expect(missing).toEqual([]);
    const shared = Object.entries(owners).filter(([, ids]) => ids.length > 1);
    expect(shared).toEqual([]);
  });

  it('card ids are unique and every form card has a save scope', () => {
    const ids = allCards.map((c) => c.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const card of allCards) {
      expect(['form', 'collection', 'action']).toContain(card.kind);
      if (card.kind === 'form') expect(card.keys.length).toBeGreaterThan(0);
    }
    expect(cardKeys('smtp')).toContain('SMTPToken');
    expect(cardById('nope')).toBeNull();
  });

  it('every group and card has copy in all four locales', () => {
    for (const locale of [zhCN, en, zhHK, ja]) {
      const platform = locale.settingsUi.platform;
      for (const group of PLATFORM_GROUPS) {
        expect(platform.groups[group.id]?.label, group.id).toBeTruthy();
        expect(platform.groups[group.id]?.description, group.id).toBeTruthy();
      }
      for (const card of allCards) expect(platform.cards[card.id]?.title, card.id).toBeTruthy();
    }
  });
});

describe('model supply options', () => {
  it('pause rules are a gateway form card with their own switch', () => {
    expect(cardById('supply-pause-rules')).toMatchObject({ groupId: 'gateway', kind: 'form', keys: ['PauseRulesEnabled', 'PauseRules'] });
  });

  it('pause rules must be empty or a JSON array', () => {
    expect(pauseRulesError('')).toBeNull();
    expect(pauseRulesError('  ')).toBeNull();
    expect(pauseRulesError('[{"name":"quota","status_codes":[402]}]')).toBeNull();
    expect(pauseRulesError('{"name":"quota"}')).toBe('notArray');
    expect(pauseRulesError('[{')).toBe('invalidJson');
  });
});

describe('resolveSettingsTarget', () => {
  const go = (args) => resolveSettingsTarget({ isRoot: false, ...args });

  it('leaves current group links alone', () => {
    expect(go({ section: 'gateway' })).toBeNull();
    expect(go({ section: 'privacy', highlight: 'log-io' })).toBeNull();
    expect(go({ section: 'payment' })).toBeNull();
  });

  it('bare and unknown paths land on the first group', () => {
    expect(go({})).toBe('/panel/setting/site');
    expect(go({ section: 'nope' })).toBe('/panel/setting/site');
  });

  it('old group ids redirect to their new groups', () => {
    expect(go({ section: 'site' })).toBeNull();
    expect(go({ section: 'auth' })).toBeNull();
    expect(go({ section: 'billing' })).toBeNull();
    expect(go({ section: 'relay' })).toBe('/panel/setting/gateway');
    expect(go({ section: 'privacy' })).toBeNull();
    expect(go({ section: 'integrations' })).toBe('/panel/setting/notify');
    expect(resolveSettingsTarget({ section: 'organization', isRoot: true })).toBe('/panel/setting/orgpolicy');
  });

  it('a highlight of a card that moved wins over the section', () => {
    expect(go({ section: 'site', highlight: 'smtp' })).toBe('/panel/setting/notify?highlight=smtp');
    expect(go({ section: 'billing', highlight: 'payment-gateways' })).toBe('/panel/setting/payment?highlight=payment-gateways');
    expect(go({ section: 'integrations', highlight: 'brand-icon' })).toBe('/panel/setting/site?highlight=branding');
    expect(go({ section: 'integrations', highlight: 'telegram' })).toBe('/panel/setting/notify?highlight=telegram');
    expect(go({ highlight: 'log-io' })).toBe('/panel/setting/privacy?highlight=log-io');
  });

  it('older section ids and ?tab= values land on their card', () => {
    expect(go({ section: 'email' })).toBe('/panel/setting/notify?highlight=smtp');
    expect(go({ section: 'ops' })).toBe('/panel/setting/gateway?highlight=channel-auto');
    expect(go({ section: 'gateways' })).toBe('/panel/setting/payment?highlight=payment-gateways');
    expect(go({ section: 'chat' })).toBe('/panel/setting/notify?highlight=chat-links');
    expect(go({ tab: 'other' })).toBe('/panel/setting/site?highlight=branding');
    expect(go({ tab: 'safety' })).toBe('/panel/setting/privacy?highlight=safety');
  });

  it('invite codes moved to their own page', () => {
    expect(go({ section: 'invite' })).toBe(INVITE_CODE_URL);
    expect(go({ section: 'auth', highlight: 'invite-code' })).toBe(INVITE_CODE_URL);
  });

  it('the root-only group is not reachable for other admins', () => {
    expect(go({ section: 'orgpolicy' })).toBe('/panel/setting/site');
    expect(go({ section: 'org' })).toBe('/panel/setting/site');
    expect(resolveSettingsTarget({ section: 'org', isRoot: true })).toBe('/panel/setting/orgpolicy?highlight=org-policy');
  });
});
