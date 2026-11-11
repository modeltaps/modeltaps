// ==============================|| PLATFORM SETTINGS — GROUP AND CARD REGISTRY ||============================== //
// The eight platform settings groups and their cards, as plain data: the page (index.jsx) attaches
// a component to every card id, the command palette lists the groups and cards, and the tests
// check that every option key still has a card. A card id doubles as its `?highlight=` anchor.
//
// kind: 'form' (saved by its footer through `/api/option/`, `keys` are its save scope),
//       'collection' (items saved one by one) or 'action' (one-off actions only).

export const SETTINGS_BASE = '/panel/setting';

export const settingsGroupUrl = (groupId) => `${SETTINGS_BASE}/${groupId}`;

const card = (id, kind, keys = []) => ({ id, kind, keys });

export const PLATFORM_GROUPS = [
  {
    id: 'site',
    cards: [
      card('server-address', 'form', ['ServerAddress']),
      card('branding', 'form', [
        'SystemName',
        'Logo',
        'HomePageContent',
        'About',
        'Footer',
        'AnalyticsCode',
        'BrandIconSyncEnabled',
        'BrandIconFaviconFetchEnabled',
        'BrandIconRegistry'
      ]),
      card('display', 'form', ['DisplayInCurrencyEnabled', 'DisplayTokenStatEnabled', 'QuotaPerUnit']),
      card('external-links', 'form', ['TopUpLink', 'ChatLink', 'DocsLink']),
      card('version-notice', 'form', ['Notice'])
    ]
  },
  {
    id: 'auth',
    cards: [
      card('account-system', 'form', ['AccountSystem']),
      card('login-methods', 'form', [
        'PasswordLoginEnabled',
        'EmailCodeLoginEnabled',
        'PasskeyLoginEnabled',
        'RegisterEnabled',
        'PasswordRegisterEnabled',
        'EmailVerificationEnabled',
        'InviteCodeRegisterEnabled',
        'TurnstileCheckEnabled'
      ]),
      card('oidc-providers', 'collection'),
      card('admin-login', 'form', ['AdminLoginEnabled']),
      card('github-oauth', 'form', ['GitHubOAuthEnabled', 'GitHubOldIdCloseEnabled', 'GitHubClientId', 'GitHubClientSecret']),
      card('wechat-auth', 'form', ['WeChatAuthEnabled', 'WeChatServerAddress', 'WeChatServerToken', 'WeChatAccountQRCodeImageURL']),
      card('lark-auth', 'form', ['LarkAuthEnabled', 'LarkClientId', 'LarkClientSecret']),
      card('linuxdo-oauth', 'form', [
        'LinuxDoOAuthEnabled',
        'LinuxDoOAuthTrustLevelEnabled',
        'LinuxDoOAuthDynamicTrustLevel',
        'LinuxDoClientId',
        'LinuxDoClientSecret',
        'LinuxDoOAuthLowestTrustLevel'
      ]),
      card('turnstile', 'form', ['TurnstileSiteKey', 'TurnstileSecretKey']),
      card('email-domain', 'form', ['EmailDomainRestrictionEnabled', 'EmailDomainWhitelist'])
    ]
  },
  {
    id: 'billing',
    cards: [
      card('quota', 'form', [
        'QuotaForNewUser',
        'PreConsumedQuota',
        'PreConsumedOutputTokens',
        'QuotaRemindEnabled',
        'QuotaRemindThreshold'
      ]),
      card('billing-rules', 'form', ['EmptyResponseBillingEnabled', 'ApproximateTokenEnabled', 'PriceSyncAutoApply']),
      card('quota-reset', 'form', ['QuotaResetTimezone', 'QuotaResetWeekStart']),
      card('invite-reward', 'form', [
        'InviteRewardEnabled',
        'QuotaForInviter',
        'QuotaForInvitee',
        'InviterRewardType',
        'InviterRewardValue'
      ])
    ]
  },
  {
    id: 'payment',
    cards: [
      card('payment', 'form', ['PaymentUSDRate', 'PaymentMinAmount', 'PaymentCallbackAddress', 'RechargeDiscount']),
      card('payment-gateways', 'collection'),
      card('invoice', 'action')
    ]
  },
  {
    id: 'gateway',
    cards: [
      card('retry', 'form', ['RetryTimes', 'RetryCooldownSeconds', 'RetryTimeOut', 'RetryCooldownPerStatus']),
      card('error-handling', 'form', ['ChannelFailErrorWrapEnabled', 'ChannelFailErrorMessage']),
      card('channel-auto', 'form', [
        'AutomaticDisableChannelEnabled',
        'AutomaticEnableChannelEnabled',
        'AutomaticDisableChannelNotifyEnabled',
        'ChannelDisableThreshold',
        'DisableChannelKeywords',
        'ChannelTestMode',
        'ChannelTestIntervalMinutes',
        'ChannelCredentialRefreshMinutes'
      ]),
      card('supply-pause-rules', 'form', ['PauseRulesEnabled', 'PauseRules']),
      card('session-affinity', 'form', ['SessionAffinityEnabled', 'SessionAffinityStrict', 'SessionAffinityTTLSeconds']),
      card('model-name-match', 'form', ['ModelNameCaseInsensitiveEnabled', 'UnifiedRequestResponseModelEnabled']),
      card('native-protocol', 'form', [
        'ClaudeAPIEnabled',
        'GeminiAPIEnabled',
        'ClaudePromptCachingEnabled',
        'FingerprintPassThroughEnabled',
        'ClaudeBudgetTokensPercentage',
        'ClaudeDefaultMaxTokens',
        'GeminiOpenThink'
      ]),
      card('prompt-limits', 'form', ['MaxPromptTokens']),
      card('image-proxy', 'form', ['ChatImageRequestProxy', 'CFWorkerImageUrl', 'CFWorkerImageKey']),
      card('auto-routing', 'form', [
        'RoutingDecisionBillingEnabled',
        'UserRoutingPolicyEnabled',
        'OrganizationRoutingPolicyMaxCount',
        'PersonalRoutingPolicyMaxCount'
      ])
    ]
  },
  {
    id: 'privacy',
    cards: [
      card('log-consume', 'form', [
        'LogConsumeEnabled',
        'LogRelayErrorEnabled',
        'ChannelMetricsEnabled',
        'ChannelMetricsRetentionDays',
        'LogAutoDeleteEnabled',
        'LogAutoDeleteDays'
      ]),
      card('log-io', 'form', ['LogIOEnabled', 'LogIODefaultUser', 'OrganizationLogIODefault']),
      card('safety', 'form', ['EnableSafe', 'SafeToolName', 'SafeKeyWords'])
    ]
  },
  {
    id: 'notify',
    cards: [
      card('smtp', 'form', ['SMTPServer', 'SMTPPort', 'SMTPAccount', 'SMTPFrom', 'SMTPToken', 'SMTPTLSMode']),
      card('notifications', 'form', ['NotifyDisabledKinds', 'NotifyKindLimitPerHour']),
      card('telegram', 'collection'),
      card('builtin-chat', 'form', ['BuiltinChatEnabled']),
      card('chat-links', 'collection', ['ChatLinks']),
      card('mj-notify', 'form', ['MjNotifyEnabled'])
    ]
  },
  {
    id: 'orgpolicy',
    rootOnly: true,
    cards: [
      card('org-policy', 'form', [
        'OrganizationEnabled',
        'OrganizationQuotaTransferEnabled',
        'OrganizationMaxPerUser',
        'OrganizationDefaultMaxMembers'
      ])
    ]
  }
];

export const DEFAULT_GROUP = PLATFORM_GROUPS[0].id;

export const groupLabelKey = (groupId) => `settingsUi.platform.groups.${groupId}.label`;
export const groupDescriptionKey = (groupId) => `settingsUi.platform.groups.${groupId}.description`;
export const cardTitleKey = (cardId) => `settingsUi.platform.cards.${cardId}.title`;

const ALL_CARDS = PLATFORM_GROUPS.flatMap((group) => group.cards.map((c) => ({ ...c, groupId: group.id })));

export const cardById = (cardId) => ALL_CARDS.find((c) => c.id === cardId) || null;

// Save scope of a card (its option keys).
export const cardKeys = (cardId) => cardById(cardId)?.keys || [];

export const visibleGroups = (isRoot) => PLATFORM_GROUPS.filter((group) => !group.rootOnly || isRoot);

// ---------------------------------------------------------------- legacy links

// Group ids before the regrouping, plus the older per-topic section ids, mapped to the new group
// and the card to highlight there.
const LEGACY_SECTIONS = {
  relay: { group: 'gateway' },
  integrations: { group: 'notify' },
  organization: { group: 'orgpolicy' },
  general: { group: 'site', highlight: 'server-address' },
  branding: { group: 'site', highlight: 'branding' },
  email: { group: 'notify', highlight: 'smtp' },
  oauth: { group: 'auth', highlight: 'login-methods' },
  login: { group: 'auth', highlight: 'login-methods' },
  payment: { group: 'payment', highlight: 'payment' },
  gateways: { group: 'payment', highlight: 'payment-gateways' },
  model: { group: 'gateway', highlight: 'native-protocol' },
  image: { group: 'gateway', highlight: 'image-proxy' },
  ops: { group: 'gateway', highlight: 'channel-auto' },
  safety: { group: 'privacy', highlight: 'safety' },
  chat: { group: 'notify', highlight: 'chat-links' },
  org: { group: 'orgpolicy', highlight: 'org-policy' }
};

// Older `?tab=` values fold into a legacy section first.
const LEGACY_TABS = { other: 'branding' };

// Card anchors that were renamed or merged into another card.
const HIGHLIGHT_ALIASES = { 'brand-icon': 'branding', chat: 'chat-links' };

// The invite code table left the settings for a page of its own.
export const INVITE_CODE_URL = '/panel/invite_code';
const INVITE_SECTIONS = new Set(['invite']);
const INVITE_HIGHLIGHTS = new Set(['invite-code']);

const withHighlight = (groupId, highlight) =>
  highlight ? `${settingsGroupUrl(groupId)}?highlight=${encodeURIComponent(highlight)}` : settingsGroupUrl(groupId);

// Where a settings URL should go, or null when it is already right. A highlight that names a card
// of another group wins over the section, so old links land on the card even after it moved.
export function resolveSettingsTarget({ section, tab, highlight, isRoot = false }) {
  const groups = visibleGroups(isRoot).map((group) => group.id);
  const legacyTab = !section && tab ? LEGACY_TABS[tab] || tab : null;
  const requested = section || legacyTab;
  if (INVITE_SECTIONS.has(requested) || INVITE_HIGHLIGHTS.has(highlight)) return INVITE_CODE_URL;

  const anchor = HIGHLIGHT_ALIASES[highlight] || highlight || null;
  const owner = anchor ? cardById(anchor)?.groupId : null;
  const current = Boolean(requested) && groups.includes(requested);
  const legacy = current ? null : LEGACY_SECTIONS[requested];
  let group = current ? requested : legacy?.group;
  if (owner && groups.includes(owner)) group = owner;
  // A card of a group the admin cannot see (root only) is not highlighted anywhere.
  const hidden = !group || !groups.includes(group);
  if (hidden) group = DEFAULT_GROUP;
  const wanted = anchor || legacy?.highlight || null;
  const target = wanted && (!hidden || !cardById(wanted)) ? wanted : null;

  if (section && section === group && target === (highlight || null)) return null;
  return withHighlight(group, target);
}
