import { createElement, useEffect, useState } from 'react';
import { Navigate, useParams, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';

import SettingsPage from '@/components/settings/SettingsPage';
import { useIsAdmin, useIsRoot } from 'utils/common';
import useHighlightTarget from 'hooks/useHighlightTarget';
import { SettingsMetaContext, useOptionSettings } from './parts';
import { groupDescriptionKey, groupLabelKey, resolveSettingsTarget, settingsGroupUrl, tabLabelKey, visibleGroups } from './registry';
import { BrandingCard, DisplayCard, ExternalLinksCard, ServerAddressCard, SiteLanguageCard, VersionNoticeCard } from './cards/SiteCards';
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
import { PricingCard } from './cards/PricingCard';
import { InvoiceCard, PaymentGatewaysCard, PaymentSettingsCard } from './cards/PaymentCards';
import {
  AutoRoutingCard,
  ChannelAutoCard,
  ErrorHandlingCard,
  ImageProxyCard,
  ImageStorageCard,
  ModelNameMatchCard,
  NativeProtocolCard,
  PromptLimitsCard,
  RetryCard,
  SessionAffinityCard,
  SupplyModeCard,
  SupplyPauseRulesCard,
  SupplyIntakeRulesCard
} from './cards/GatewayCards';
import SupplyFlow from './cards/SupplyFlow';
import { LogConsumeCard, LogIOCard, SafetyCard } from './cards/PrivacyCards';
import { BuiltinChatCard, MjNotifyCard, NotificationsCard, NotifyChannelsCard, TelegramCard } from './cards/NotifyCards';
import { SmtpCard } from './cards/MailCard';
import { MailTemplatesCard } from './cards/MailTemplatesCard';
import { MailLogsCard, NotifyLogsCard } from './cards/MessageLogCard';
import { OrgPolicyCard } from './cards/OrgPolicyCards';
import { ModifiedSettingsCard } from './cards/ModifiedSettingsCard';
import { workbench } from 'routes/paths';
import { API } from 'utils/api';

// ==============================|| PANEL — PLATFORM SETTINGS (admin) ||============================== //
// One settings page in ten groups (`/panel/admin/settings/:group`), each group a menu item of
// its own (nav-config files them under the sections they configure); the groups and their cards come
// from `./registry`, this file attaches a component to every card. All option cards share one
// `useOptionSettings` state, so unsaved edits survive a group switch. Old group ids, the older
// section ids and `?tab=` values, and highlights of cards that moved redirect to the right group.

const CARD_COMPONENTS = {
  'server-address': ServerAddressCard,
  branding: BrandingCard,
  display: DisplayCard,
  'site-language': SiteLanguageCard,
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
  pricing: PricingCard,
  'billing-rules': BillingRulesCard,
  'quota-reset': QuotaResetCard,
  'invite-reward': InviteRewardCard,
  payment: PaymentSettingsCard,
  'payment-gateways': PaymentGatewaysCard,
  invoice: InvoiceCard,
  retry: RetryCard,
  'error-handling': ErrorHandlingCard,
  'channel-auto': ChannelAutoCard,
  'supply-mode': SupplyModeCard,
  'supply-pause-rules': SupplyPauseRulesCard,
  'supply-intake-rules': SupplyIntakeRulesCard,
  'session-affinity': SessionAffinityCard,
  'model-name-match': ModelNameMatchCard,
  'native-protocol': NativeProtocolCard,
  'prompt-limits': PromptLimitsCard,
  'image-proxy': ImageProxyCard,
  'image-storage': ImageStorageCard,
  'auto-routing': AutoRoutingCard,
  'log-consume': LogConsumeCard,
  'log-io': LogIOCard,
  safety: SafetyCard,
  smtp: SmtpCard,
  'mail-templates': MailTemplatesCard,
  'mail-logs': MailLogsCard,
  'notify-logs': NotifyLogsCard,
  notifications: NotificationsCard,
  'notify-channels': NotifyChannelsCard,
  telegram: TelegramCard,
  'builtin-chat': BuiltinChatCard,
  'mj-notify': MjNotifyCard,
  'org-policy': OrgPolicyCard,
  'modified-settings': ModifiedSettingsCard
};

// Overviews drawn above a group's cards.
const GROUP_INTROS = { supply: SupplyFlow };

export default function Setting() {
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();
  const ctx = useOptionSettings();
  const { section } = useParams();
  const [searchParams] = useSearchParams();
  useHighlightTarget();
  // How many objects set their own value instead of following a site setting (root only).
  const [overrides, setOverrides] = useState({});
  useEffect(() => {
    if (!isRoot) return;
    API.get('/api/settings/overrides')
      .then((res) => res.data?.success && setOverrides(res.data.data || {}))
      .catch(() => {});
  }, [isRoot]);

  if (!isAdmin) return <Navigate to={workbench.dashboard()} replace />;

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
    label: t(groupLabelKey(group.id)),
    description: t(groupDescriptionKey(group.id)),
    tabs: group.tabs?.map((id) => ({ id, label: t(tabLabelKey(id)) })),
    intro: GROUP_INTROS[group.id] ? () => createElement(GROUP_INTROS[group.id], { ctx }) : undefined,
    cards: group.cards.map((card) => {
      const Component = CARD_COMPONENTS[card.id];
      return { id: card.id, tab: card.tab, keys: card.keys, render: () => <Component ctx={ctx} /> };
    })
  }));

  return (
    <SettingsMetaContext.Provider value={{ defaults: ctx.defaults, origin: ctx.origin, overrides }}>
      <SettingsPage label={t('settingsUi.platform.label')} groups={groups} activeGroupId={section} hideNav />
    </SettingsMetaContext.Provider>
  );
}
