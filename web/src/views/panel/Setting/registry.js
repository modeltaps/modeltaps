import { admin } from 'routes/paths';
// ==============================|| PLATFORM SETTINGS — GROUP AND CARD REGISTRY ||============================== //
// The eight platform settings groups and their cards, as plain data: the page (index.jsx) attaches
// a component to every card id, the command palette lists the groups and cards, and the tests
// check that every option key still has a card. A card id doubles as its `?highlight=` anchor.
//
// kind: 'form' (saved by its footer through `/api/option/`, `keys` are its save scope),
//       'collection' (items saved one by one) or 'action' (one-off actions only).
// A group with many cards lists `tabs`; each of its cards names its tab, and the page shows one
// tab at a time (`?tab=<id>`).

export const SETTINGS_BASE = admin.settings();

export const settingsGroupUrl = (groupId) => `${SETTINGS_BASE}/${groupId}`;

export const IMAGE_STORAGE_KEYS = [
  'ImageStorageProvider',
  'ImageStorageS3Endpoint',
  'ImageStorageS3Region',
  'ImageStorageS3BucketName',
  'ImageStorageS3AccessKeyId',
  'ImageStorageS3AccessKeySecret',
  'ImageStorageS3CDNURL',
  'ImageStorageS3ExpirationDays',
  'ImageStorageOSSEndpoint',
  'ImageStorageOSSRegion',
  'ImageStorageOSSBucketName',
  'ImageStorageOSSAccessKeyId',
  'ImageStorageOSSAccessKeySecret',
  'ImageStorageSMMSSecret',
  'ImageStorageImgurClientId',
  'ImageStorageRehostUpstream'
];

const card = (id, kind, keys = [], tab) => ({ id, kind, keys, ...(tab ? { tab } : {}) });

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
      card('site-language', 'form', ['DefaultLanguage']),
      card('external-links', 'form', ['TopUpLink', 'DocsLink']),
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
      card('pricing', 'form', ['PaymentCostPercent']),
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
    // 容错: what happens when an upstream fails; 分配: which supply serves a request;
    // 请求与协议: how a request is forwarded.
    tabs: ['resilience', 'allocation', 'request'],
    cards: [
      card('retry', 'form', ['RetryTimes', 'RetryCooldownSeconds', 'RetryTimeOut', 'RetryCooldownPerStatus'], 'resilience'),
      card('supply-pause-rules', 'form', ['PauseRules'], 'resilience'),
      card(
        'channel-auto',
        'form',
        [
          'AutomaticDisableChannelEnabled',
          'AutomaticEnableChannelEnabled',
          'ChannelDisableThreshold',
          'DisableChannelKeywords',
          'ChannelTestMode',
          'ChannelTestIntervalMinutes',
          'ChannelCredentialRefreshMinutes'
        ],
        'resilience'
      ),
      card('error-handling', 'form', ['ChannelFailErrorWrapEnabled', 'ChannelFailErrorMessage'], 'resilience'),
      card(
        'supply-mode',
        'form',
        ['ModelSupplyEnabled', 'SupplyModeDefault', 'SupplyCostOrder', 'SupplyCostMinSavingPercent', 'SupplyCostMinSuccessPercent'],
        'allocation'
      ),
      card('supply-intake-rules', 'form', ['SupplyIntakeRules'], 'allocation'),
      card('session-affinity', 'form', ['SessionAffinityEnabled', 'SessionAffinityStrict', 'SessionAffinityTTLSeconds'], 'allocation'),
      card(
        'auto-routing',
        'form',
        ['RoutingDecisionBillingEnabled', 'UserRoutingPolicyEnabled', 'OrganizationRoutingPolicyMaxCount', 'PersonalRoutingPolicyMaxCount'],
        'allocation'
      ),
      card('model-name-match', 'form', ['ModelNameCaseInsensitiveEnabled', 'UnifiedRequestResponseModelEnabled'], 'request'),
      card(
        'native-protocol',
        'form',
        [
          'ClaudeAPIEnabled',
          'GeminiAPIEnabled',
          'ClaudePromptCachingEnabled',
          'FingerprintPassThroughEnabled',
          'ClaudeBudgetTokensPercentage',
          'ClaudeDefaultMaxTokens',
          'GeminiOpenThink'
        ],
        'request'
      ),
      card('prompt-limits', 'form', ['MaxPromptTokens'], 'request'),
      card('image-proxy', 'form', ['ChatImageRequestProxy', 'CFWorkerImageUrl', 'CFWorkerImageKey'], 'request'),
      // Where generated images are uploaded so a request asking for a URL gets one.
      card('image-storage', 'form', IMAGE_STORAGE_KEYS, 'request'),
      card('mj-notify', 'form', ['MjNotifyEnabled'], 'request')
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
    id: 'entries',
    // What users reach from outside the console: the Telegram user bot and the built-in chat.
    cards: [card('telegram', 'collection'), card('builtin-chat', 'form', ['BuiltinChatEnabled'])]
  },
  {
    id: 'mail',
    // The mail services behind every email (login codes, password resets, quota warnings, email
    // notices), one in use. The SMTP* keys mirror the one in use and are not saved by a form.
    cards: [
      card('smtp', 'collection', [
        'SMTPProvider',
        'SMTPServer',
        'SMTPPort',
        'SMTPAccount',
        'SMTPFrom',
        'SMTPFromName',
        'SMTPReplyTo',
        'SMTPToken',
        'SMTPTLSMode'
      ]),
      card('mail-templates', 'form', ['MailTemplates']),
      card('mail-logs', 'action')
    ]
  },
  {
    id: 'notify',
    // The platform's admin notices: which kinds are sent, the channels they go to, what went out.
    cards: [
      card('notifications', 'form', ['NotifyDisabledKinds', 'NotifyKindLimitPerHour', 'UsageAlertTiers']),
      card('notify-channels', 'collection'),
      card('notify-logs', 'action')
    ]
  },
  {
    // Every setting that differs from its default, across the groups.
    id: 'modified',
    cards: [card('modified-settings', 'action')]
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
export const tabLabelKey = (tabId) => `settingsUi.platform.tabs.${tabId}`;

const ALL_CARDS = PLATFORM_GROUPS.flatMap((group) => group.cards.map((c) => ({ ...c, groupId: group.id })));

export const cardById = (cardId) => ALL_CARDS.find((c) => c.id === cardId) || null;

// Save scope of a card (its option keys).
export const cardKeys = (cardId) => cardById(cardId)?.keys || [];

// The card an option key belongs to.
export const cardOfKey = (key) => (key ? ALL_CARDS.find((c) => c.keys.includes(key)) || null : null);

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
  email: { group: 'mail', highlight: 'smtp' },
  oauth: { group: 'auth', highlight: 'login-methods' },
  login: { group: 'auth', highlight: 'login-methods' },
  payment: { group: 'payment', highlight: 'payment' },
  gateways: { group: 'payment', highlight: 'payment-gateways' },
  model: { group: 'gateway', highlight: 'native-protocol' },
  image: { group: 'gateway', highlight: 'image-proxy' },
  ops: { group: 'gateway', highlight: 'channel-auto' },
  safety: { group: 'privacy', highlight: 'safety' },
  chat: { group: 'entries', highlight: 'builtin-chat' },
  org: { group: 'orgpolicy', highlight: 'org-policy' }
};

// Older `?tab=` values fold into a legacy section first.
const LEGACY_TABS = { other: 'branding' };

// Card anchors that were renamed or merged into another card.
// The chat links card left: its links could no longer open a chat with the user's key.
const HIGHLIGHT_ALIASES = { 'brand-icon': 'branding', chat: 'builtin-chat', 'chat-links': 'builtin-chat' };

// The invite code table left the settings for a page of its own.
export const INVITE_CODE_URL = admin.inviteCodes();
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
  // A highlight names a card, or an option key (a field of a card).
  const owner = anchor ? (cardById(anchor) || cardOfKey(anchor))?.groupId : null;
  const current = Boolean(requested) && groups.includes(requested);
  const legacy = current ? null : LEGACY_SECTIONS[requested];
  let group = current ? requested : legacy?.group;
  if (owner && groups.includes(owner)) group = owner;
  // A card of a group the admin cannot see (root only) is not highlighted anywhere.
  const hidden = !group || !groups.includes(group);
  if (hidden) group = DEFAULT_GROUP;
  const wanted = anchor || legacy?.highlight || null;
  const target = wanted && (!hidden || !(cardById(wanted) || cardOfKey(wanted))) ? wanted : null;

  if (section && section === group && target === (highlight || null)) return null;
  return withHighlight(group, target);
}
