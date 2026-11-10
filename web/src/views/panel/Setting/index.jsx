import { Navigate, useParams, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { CreditCard, Globe, Landmark, LogIn, Plug, ReceiptText, Route, ShieldCheck } from 'lucide-react';

import SettingsPage from '@/components/settings/SettingsPage';
import { useIsAdmin, useIsRoot } from 'utils/common';
import useHighlightTarget from 'hooks/useHighlightTarget';
import { useOptionSettings } from './parts';
import { groupDescriptionKey, groupLabelKey, resolveSettingsTarget, settingsGroupUrl, visibleGroups } from './registry';
import { BrandingCard, DisplayCard, ExternalLinksCard, ServerAddressCard, VersionNoticeCard } from './cards/SiteCards';
import {
  AccountSystemCard,
  AdminLoginCard,
  EmailDomainCard,
  GitHubOAuthCard,
  LarkAuthCard,
  LinuxDoOAuthCard,
  LoginMethodsCard,
  OidcProvidersCard,
  TurnstileCard,
  WeChatAuthCard
} from './cards/AuthCards';
import { BillingRulesCard, InviteRewardCard, QuotaCard, QuotaResetCard } from './cards/BillingCards';
import { InvoiceCard, PaymentGatewaysCard, PaymentSettingsCard } from './cards/PaymentCards';
import {
  AutoRoutingCard,
  ChannelAutoCard,
  ErrorHandlingCard,
  ImageProxyCard,
  ModelNameMatchCard,
  NativeProtocolCard,
  PromptLimitsCard,
  RetryCard,
  SessionAffinityCard,
  SupplyPauseRulesCard
} from './cards/GatewayCards';
import { LogConsumeCard, LogIOCard, SafetyCard } from './cards/PrivacyCards';
import { BuiltinChatCard, ChatLinksCard, MjNotifyCard, NotificationsCard, SmtpCard, TelegramCard } from './cards/NotifyCards';
import { OrgPolicyCard } from './cards/OrgPolicyCards';

// ==============================|| PANEL — PLATFORM SETTINGS (admin) ||============================== //
// One settings page in eight groups (`/panel/setting/:group`); the groups and their cards come
// from `./registry`, this file attaches a component to every card. All option cards share one
// `useOptionSettings` state, so unsaved edits survive a group switch. Old group ids, the older
// section ids and `?tab=` values, and highlights of cards that moved redirect to the right group.

const CARD_COMPONENTS = {
  'server-address': ServerAddressCard,
  branding: BrandingCard,
  display: DisplayCard,
  'external-links': ExternalLinksCard,
  'version-notice': VersionNoticeCard,
  'account-system': AccountSystemCard,
  'login-methods': LoginMethodsCard,
  'oidc-providers': OidcProvidersCard,
  'admin-login': AdminLoginCard,
  'github-oauth': GitHubOAuthCard,
  'wechat-auth': WeChatAuthCard,
  'lark-auth': LarkAuthCard,
  'linuxdo-oauth': LinuxDoOAuthCard,
  turnstile: TurnstileCard,
  'email-domain': EmailDomainCard,
  quota: QuotaCard,
  'billing-rules': BillingRulesCard,
  'quota-reset': QuotaResetCard,
  'invite-reward': InviteRewardCard,
  payment: PaymentSettingsCard,
  'payment-gateways': PaymentGatewaysCard,
  invoice: InvoiceCard,
  retry: RetryCard,
  'error-handling': ErrorHandlingCard,
  'channel-auto': ChannelAutoCard,
  'supply-pause-rules': SupplyPauseRulesCard,
  'session-affinity': SessionAffinityCard,
  'model-name-match': ModelNameMatchCard,
  'native-protocol': NativeProtocolCard,
  'prompt-limits': PromptLimitsCard,
  'image-proxy': ImageProxyCard,
  'auto-routing': AutoRoutingCard,
  'log-consume': LogConsumeCard,
  'log-io': LogIOCard,
  safety: SafetyCard,
  smtp: SmtpCard,
  notifications: NotificationsCard,
  telegram: TelegramCard,
  'builtin-chat': BuiltinChatCard,
  'chat-links': ChatLinksCard,
  'mj-notify': MjNotifyCard,
  'org-policy': OrgPolicyCard
};

const GROUP_ICONS = {
  site: Globe,
  auth: LogIn,
  billing: ReceiptText,
  payment: CreditCard,
  gateway: Route,
  privacy: ShieldCheck,
  notify: Plug,
  orgpolicy: Landmark
};

export default function Setting() {
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();
  const ctx = useOptionSettings();
  const { section } = useParams();
  const [searchParams] = useSearchParams();
  useHighlightTarget();

  if (!isAdmin) return <Navigate to="/panel/dashboard" replace />;

  const target = resolveSettingsTarget({
    section,
    tab: searchParams.get('tab'),
    highlight: searchParams.get('highlight'),
    isRoot
  });
  if (target) return <Navigate to={target} replace />;

  const groups = visibleGroups(isRoot).map((group) => ({
    id: group.id,
    to: settingsGroupUrl(group.id),
    icon: GROUP_ICONS[group.id],
    label: t(groupLabelKey(group.id)),
    description: t(groupDescriptionKey(group.id)),
    badge: group.rootOnly ? 'root' : undefined,
    cards: group.cards.map((card) => {
      const Component = CARD_COMPONENTS[card.id];
      return { id: card.id, render: () => <Component ctx={ctx} /> };
    })
  }));

  return <SettingsPage label={t('settingsUi.platform.label')} groups={groups} activeGroupId={section} className="max-w-7xl" />;
}
