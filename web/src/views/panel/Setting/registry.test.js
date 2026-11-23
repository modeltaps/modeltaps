import { describe, expect, it } from 'vitest';

import en from 'i18n/locales/modeltaps/en_US.json';
import ja from 'i18n/locales/modeltaps/ja_JP.json';
import zhCN from 'i18n/locales/modeltaps/zh_CN.json';
import zhHK from 'i18n/locales/modeltaps/zh_HK.json';
import { describePauseRule, pauseRulesError, pauseRulesText } from './cards/pauseRules';
import { INVITE_CODE_URL, PLATFORM_GROUPS, cardById, cardKeys, resolveSettingsTarget, visibleGroups } from './registry';

// Every option key the platform settings pages could save before the regrouping (the theme
// pages and their cards). Each must still belong to exactly one card; the routing policy
// settings moved to the routing policies page and are left out.
const LEGACY_OPTION_KEYS = [
  'About',
  'AccountSystem',
  'AdminLoginEnabled',
  'AnalyticsCode',
  'ApproximateTokenEnabled',
  'AutomaticDisableChannelEnabled',
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
  it('has the groups in order', () => {
    expect(PLATFORM_GROUPS.map((g) => g.id)).toEqual([
      'site',
      'auth',
      'billing',
      'payment',
      'supply',
      'gateway',
      'privacy',
      'media',
      'entries',
      'mail',
      'notify',
      'modified',
      'orgpolicy'
    ]);
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
      for (const group of PLATFORM_GROUPS.filter((g) => g.tabs)) {
        for (const tab of group.tabs) expect(platform.tabs[tab], tab).toBeTruthy();
      }
    }
  });
});

describe('auto routing options', () => {
  it('the routing policy settings left the settings page for the routing policies page', () => {
    expect(cardById('auto-routing')).toBeNull();
    expect(resolveSettingsTarget({ section: 'supply', highlight: 'auto-routing' })).toBe(
      '/panel/admin/routing?highlight=RoutingDecisionBillingEnabled'
    );
  });
});

describe('model supply options', () => {
  it('pause rules are a gateway form card with their own switch', () => {
    expect(cardById('supply-pause-rules')).toMatchObject({ groupId: 'gateway', kind: 'form', keys: ['PauseRules'] });
  });

  it('the routing switch and the default supply mode are a supply allocation form card', () => {
    expect(cardById('supply-mode')).toMatchObject({
      groupId: 'supply',
      kind: 'form',
      keys: ['ModelSupplyEnabled', 'SupplyCostOrder', 'SupplyCostMinSavingPercent', 'SupplyCostMinSuccessPercent']
    });
  });

  it('pause rules must be empty or a JSON array', () => {
    expect(pauseRulesError('')).toBeNull();
    expect(pauseRulesError('  ')).toBeNull();
    expect(pauseRulesError('[{"name":"quota","status_codes":[402]}]')).toBeNull();
    expect(pauseRulesError('{"name":"quota"}')).toBe('notArray');
    expect(pauseRulesError('[{')).toBe('invalidJson');
  });

  it('the built-in rules fill the editor as a valid list', () => {
    const text = pauseRulesText([{ name: 'quota', status_codes: [402], action: 'pause', seconds: 1800 }]);
    expect(pauseRulesError(text)).toBeNull();
    expect(JSON.parse(text)[0].name).toBe('quota');
    expect(pauseRulesText(null)).toBe('[]');
  });

  it('describes a rule as its conditions and its action', () => {
    expect(
      describePauseRule({
        status_codes: [402],
        keywords: ['a', 'b'],
        error_codes: ['c'],
        action: 'pause',
        seconds: 1800,
        scope: 'channel',
        probe: true,
        notify: true
      })
    ).toEqual({
      when: [
        ['status', { codes: '402' }],
        ['patterns', { count: 3 }]
      ],
      then: [['pause_channel', { seconds: 1800 }], ['probe'], ['notify']]
    });
    expect(describePauseRule({ status_codes: [429], rate_limit_reset: true, action: 'pause', scope: 'model' })).toEqual({
      when: [['status', { codes: '429' }], ['rateLimitReset']],
      then: [['pauseUntilReset_model']]
    });
    expect(describePauseRule({ times: 3, window_seconds: 60, action: 'retry' })).toEqual({
      when: [['anyError'], ['times', { count: 3, seconds: 60 }]],
      then: [['retry']]
    });
    expect(describePauseRule({ per_status_config: true, action: 'pause', scope: 'model' }).then).toEqual([['pauseConfigured_model']]);
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
    expect(go({})).toBe('/panel/admin/settings/site');
    expect(go({ section: 'nope' })).toBe('/panel/admin/settings/site');
  });

  it('old group ids redirect to their new groups', () => {
    expect(go({ section: 'site' })).toBeNull();
    expect(go({ section: 'auth' })).toBeNull();
    expect(go({ section: 'billing' })).toBeNull();
    expect(go({ section: 'relay' })).toBe('/panel/admin/settings/gateway');
    expect(go({ section: 'privacy' })).toBeNull();
    expect(go({ section: 'integrations' })).toBe('/panel/admin/settings/notify');
    expect(resolveSettingsTarget({ section: 'organization', isRoot: true })).toBe('/panel/admin/settings/orgpolicy');
  });

  it('a highlight of a card that moved wins over the section', () => {
    expect(go({ section: 'site', highlight: 'smtp' })).toBe('/panel/admin/settings/mail?highlight=smtp');
    expect(go({ section: 'billing', highlight: 'payment-gateways' })).toBe('/panel/admin/settings/payment?highlight=payment-gateways');
    expect(go({ section: 'integrations', highlight: 'brand-icon' })).toBe('/panel/admin/settings/site?highlight=branding');
    expect(go({ section: 'integrations', highlight: 'telegram' })).toBe('/panel/admin/settings/entries?highlight=telegram');
    expect(go({ highlight: 'log-io' })).toBe('/panel/admin/settings/privacy?highlight=log-io');
  });

  it('older section ids and ?tab= values land on their card', () => {
    expect(go({ section: 'email' })).toBe('/panel/admin/settings/mail?highlight=smtp');
    expect(go({ section: 'ops' })).toBe('/panel/admin/settings/gateway?highlight=channel-auto');
    expect(go({ section: 'gateways' })).toBe('/panel/admin/settings/payment?highlight=payment-gateways');
    expect(go({ section: 'chat' })).toBe('/panel/admin/settings/entries?highlight=builtin-chat');
    expect(go({ tab: 'other' })).toBe('/panel/admin/settings/site?highlight=branding');
    expect(go({ tab: 'safety' })).toBe('/panel/admin/settings/privacy?highlight=safety');
  });

  it('invite codes moved to their own page', () => {
    expect(go({ section: 'invite' })).toBe(INVITE_CODE_URL);
    expect(go({ section: 'auth', highlight: 'invite-code' })).toBe(INVITE_CODE_URL);
  });

  it('the root-only group is not reachable for other admins', () => {
    expect(go({ section: 'orgpolicy' })).toBe('/panel/admin/settings/site');
    expect(go({ section: 'org' })).toBe('/panel/admin/settings/site');
    expect(resolveSettingsTarget({ section: 'org', isRoot: true })).toBe('/panel/admin/settings/orgpolicy?highlight=org-policy');
  });
});

describe('tabbed groups', () => {
  it('every card of a tabbed group names one of its tabs', () => {
    for (const group of PLATFORM_GROUPS.filter((g) => g.tabs)) {
      for (const card of group.cards) expect(group.tabs).toContain(card.tab);
    }
    expect(PLATFORM_GROUPS.find((g) => g.id === 'gateway').tabs).toEqual(['resilience', 'request']);
    expect(PLATFORM_GROUPS.find((g) => g.id === 'notify').tabs).toBeUndefined();
  });

  it('mail, notices and user entries are groups of their own', () => {
    expect(cardById('smtp').groupId).toBe('mail');
    expect(cardById('mail-templates').groupId).toBe('mail');
    expect(cardById('notify-logs').groupId).toBe('notify');
    expect(cardById('telegram').groupId).toBe('entries');
    expect(resolveSettingsTarget({ section: 'notify', tab: 'integrations', highlight: 'chat-links' })).toBe(
      '/panel/admin/settings/entries?highlight=builtin-chat'
    );
  });

  it('the Midjourney callback is a media setting, old links follow it', () => {
    expect(cardById('mj-notify')).toMatchObject({ groupId: 'media' });
    expect(resolveSettingsTarget({ section: 'notify', highlight: 'mj-notify' })).toBe('/panel/admin/settings/media?highlight=mj-notify');
    expect(resolveSettingsTarget({ section: 'gateway', highlight: 'mj-notify' })).toBe('/panel/admin/settings/media?highlight=mj-notify');
  });

  it('the allocation tab and the image cards left the gateway, old links follow them', () => {
    expect(resolveSettingsTarget({ section: 'gateway', tab: 'allocation' })).toBe('/panel/admin/settings/supply');
    expect(resolveSettingsTarget({ section: 'gateway', tab: 'allocation', highlight: 'SupplyCostOrder' })).toBe(
      '/panel/admin/settings/supply?highlight=SupplyCostOrder'
    );
    expect(resolveSettingsTarget({ section: 'gateway', highlight: 'session-affinity' })).toBe(
      '/panel/admin/settings/supply?highlight=session-affinity'
    );
    expect(resolveSettingsTarget({ section: 'gateway', tab: 'request', highlight: 'ImageStorageProvider' })).toBe(
      '/panel/admin/settings/media?highlight=ImageStorageProvider'
    );
    expect(resolveSettingsTarget({ section: 'image' })).toBe('/panel/admin/settings/media?highlight=image-proxy');
    expect(resolveSettingsTarget({ section: 'gateway', tab: 'request' })).toBeNull();
  });
});
