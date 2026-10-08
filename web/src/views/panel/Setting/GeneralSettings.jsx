import { useTranslation } from 'react-i18next';
import { AlertTriangle } from 'lucide-react';

import { removeTrailingSlash } from 'utils/common';
import { isPlaceholderServerAddress } from 'utils/serverAddress';
import { Alert } from '@/components/ui/alert';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import QuotaInput from '@/components/QuotaInput';
import { SettingsCard, TextRow, ToggleRow, SaveButton } from './parts';

// Cards from the former "General" tab, exported one component per SettingsCard so
// the theme pages (site / auth / billing / relay) can re-assemble them. Each card
// keeps its own `saveKeys` scope; option keys preserved verbatim from v1.

// 常用 IANA 时区（周期限额重置用）；UTC 为默认，其余覆盖主要人口带。
const QUOTA_RESET_TIMEZONES = [
  'UTC',
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Tokyo',
  'Asia/Singapore',
  'Asia/Kolkata',
  'Asia/Dubai',
  'Europe/London',
  'Europe/Paris',
  'Europe/Moscow',
  'America/New_York',
  'America/Chicago',
  'America/Los_Angeles',
  'America/Sao_Paulo',
  'Australia/Sydney',
  'Pacific/Auckland'
];

// 时区标签附当前 GMT 偏移便于识别；异常时退回原始 IANA 名。
function tzLabel(tz) {
  if (tz === 'UTC') return 'UTC';
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'shortOffset' }).formatToParts(new Date());
    const off = parts.find((p) => p.type === 'timeZoneName')?.value;
    return off ? `${tz} (${off})` : tz;
  } catch {
    return tz;
  }
}

// 站点地址（支付回调地址随支付参数卡迁到「计费与支付」）。
export function ServerAddressCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const tr = (k, opts) => t(`setting_index.systemSettings.${k}`, opts);

  return (
    <SettingsCard title={tr('serverSettings.title')} highlight="server-address">
      <TextRow
        id="ServerAddress"
        label={tr('generalSettings.serverAddress')}
        value={inputs.ServerAddress}
        onChange={(v) => setField('ServerAddress', v)}
        placeholder={tr('generalSettings.serverAddressPlaceholder')}
        description={tr('generalSettings.serverAddressDescription')}
        disabled={loading}
      />
      {isPlaceholderServerAddress(inputs.ServerAddress) && (
        <Alert variant="warning">
          <AlertTriangle />
          <span>{tr('generalSettings.serverAddressWarning', { suggested: window.location.origin })}</span>
        </Alert>
      )}
      <SaveButton
        loading={loading}
        disabled={!isDirty(['ServerAddress'])}
        onClick={() => saveKeys(['ServerAddress'], { transform: { ServerAddress: (v) => removeTrailingSlash(v || '') } })}
      >
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}


// 显示 — 货币/额度展示开关与单位额度。
export function DisplayCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const is = (k) => inputs[k] === 'true';

  return (
    <SettingsCard title={t('setting_index.cards.display')} highlight="display">
      <div className="grid gap-3">
        <ToggleRow
          label={t('setting_index.operationSettings.generalSettings.displayInCurrency')}
          description={t('setting_index.operationSettings.generalSettings.displayInCurrencyDescription')}
          checked={is('DisplayInCurrencyEnabled')}
          onCheckedChange={() => toggle('DisplayInCurrencyEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={t('setting_index.operationSettings.generalSettings.displayTokenStat')}
          description={t('setting_index.operationSettings.generalSettings.displayTokenStatDescription')}
          checked={is('DisplayTokenStatEnabled')}
          onCheckedChange={() => toggle('DisplayTokenStatEnabled')}
          disabled={loading}
        />
      </div>
      <TextRow
        id="QuotaPerUnit"
        label={t('setting_index.operationSettings.generalSettings.quotaPerUnit.label')}
        value={inputs.QuotaPerUnit}
        onChange={(v) => setField('QuotaPerUnit', v)}
        placeholder={t('setting_index.operationSettings.generalSettings.quotaPerUnit.placeholder')}
        description={t('setting_index.operationSettings.generalSettings.quotaPerUnit.description')}
        disabled={loading}
      />
      <SaveButton loading={loading} disabled={!isDirty(['QuotaPerUnit'])} onClick={() => saveKeys(['QuotaPerUnit'])}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 外链 — 充值 / 聊天 / 文档入口。
export function ExternalLinksCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const keys = ['TopUpLink', 'ChatLink', 'DocsLink'];

  return (
    <SettingsCard title={t('setting_index.cards.externalLinks')} highlight="external-links">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <TextRow
          id="TopUpLink"
          label={t('setting_index.operationSettings.generalSettings.topUpLink.label')}
          value={inputs.TopUpLink}
          onChange={(v) => setField('TopUpLink', v)}
          placeholder={t('setting_index.operationSettings.generalSettings.topUpLink.placeholder')}
          description={t('setting_index.operationSettings.generalSettings.topUpLink.description')}
          disabled={loading}
        />
        <TextRow
          id="ChatLink"
          label={t('setting_index.operationSettings.generalSettings.chatLink.label')}
          value={inputs.ChatLink}
          onChange={(v) => setField('ChatLink', v)}
          placeholder={t('setting_index.operationSettings.generalSettings.chatLink.placeholder')}
          description={t('setting_index.operationSettings.generalSettings.chatLink.description')}
          disabled={loading}
        />
        <TextRow
          id="DocsLink"
          label={t('setting_index.operationSettings.generalSettings.docsLink.label')}
          value={inputs.DocsLink}
          onChange={(v) => setField('DocsLink', v)}
          placeholder={t('setting_index.operationSettings.generalSettings.docsLink.placeholder')}
          description={t('setting_index.operationSettings.generalSettings.docsLink.description')}
          disabled={loading}
        />
      </div>
      <SaveButton loading={loading} disabled={!isDirty(keys)} onClick={() => saveKeys(keys)}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 重试 — 次数 / 冷却 / 超时与按状态码的冷却覆盖。
export function RetryCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const keys = ['RetryTimes', 'RetryCooldownSeconds', 'RetryTimeOut', 'RetryCooldownPerStatus'];

  return (
    <SettingsCard title={t('setting_index.cards.retry')} highlight="retry">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <TextRow
          id="RetryTimes"
          label={t('setting_index.operationSettings.generalSettings.retryTimes.label')}
          value={inputs.RetryTimes}
          onChange={(v) => setField('RetryTimes', v)}
          placeholder={t('setting_index.operationSettings.generalSettings.retryTimes.placeholder')}
          description={t('setting_index.operationSettings.generalSettings.retryTimes.description')}
          disabled={loading}
        />
        <TextRow
          id="RetryCooldownSeconds"
          label={t('setting_index.operationSettings.generalSettings.retryCooldownSeconds.label')}
          value={inputs.RetryCooldownSeconds}
          onChange={(v) => setField('RetryCooldownSeconds', v)}
          placeholder={t('setting_index.operationSettings.generalSettings.retryCooldownSeconds.placeholder')}
          description={t('setting_index.operationSettings.generalSettings.retryCooldownSeconds.description')}
          disabled={loading}
        />
        <TextRow
          id="RetryTimeOut"
          label={t('setting_index.operationSettings.generalSettings.retryTimeOut.label')}
          value={inputs.RetryTimeOut}
          onChange={(v) => setField('RetryTimeOut', v)}
          placeholder={t('setting_index.operationSettings.generalSettings.retryTimeOut.placeholder')}
          description={t('setting_index.operationSettings.generalSettings.retryTimeOut.description')}
          disabled={loading}
        />
      </div>
      <TextRow
        id="RetryCooldownPerStatus"
        label={t('setting_index.operationSettings.generalSettings.retryCooldownPerStatus.title')}
        value={inputs.RetryCooldownPerStatus}
        onChange={(v) => setField('RetryCooldownPerStatus', v)}
        description={t('setting_index.operationSettings.generalSettings.retryCooldownPerStatus.description')}
        placeholder='{"503": 120}'
        multiline
        rows={3}
        disabled={loading}
      />
      <SaveButton loading={loading} disabled={!isDirty(keys)} onClick={() => saveKeys(keys)}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 错误处理 — 渠道失败错误包装开关与自定义文案。
export function ErrorHandlingCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const is = (k) => inputs[k] === 'true';

  return (
    <SettingsCard title={t('setting_index.cards.errorHandling')} highlight="error-handling">
      <ToggleRow
        label={t('setting_index.operationSettings.generalSettings.channelFailErrorWrapEnabled')}
        description={t('setting_index.operationSettings.generalSettings.channelFailErrorWrapEnabledTooltip')}
        checked={is('ChannelFailErrorWrapEnabled')}
        onCheckedChange={() => toggle('ChannelFailErrorWrapEnabled')}
        disabled={loading}
      />
      <TextRow
        id="ChannelFailErrorMessage"
        label={t('setting_index.operationSettings.generalSettings.channelFailErrorMessage.label')}
        value={inputs.ChannelFailErrorMessage}
        onChange={(v) => setField('ChannelFailErrorMessage', v)}
        placeholder={t('setting_index.operationSettings.generalSettings.channelFailErrorMessage.placeholder')}
        disabled={loading || !is('ChannelFailErrorWrapEnabled')}
      />
      <SaveButton loading={loading} disabled={!isDirty(['ChannelFailErrorMessage'])} onClick={() => saveKeys(['ChannelFailErrorMessage'])}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 模型名匹配 — 大小写不敏感与请求/响应模型名统一。
export function ModelNameMatchCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, toggle, loading } = ctx;
  const is = (k) => inputs[k] === 'true';

  return (
    <SettingsCard title={t('setting_index.cards.modelNameMatch')} highlight="model-name-match">
      <div className="grid gap-3">
        <ToggleRow
          label={t('setting_index.operationSettings.generalSettings.modelNameCaseInsensitive')}
          description={t('setting_index.operationSettings.generalSettings.modelNameCaseInsensitiveDescription')}
          checked={is('ModelNameCaseInsensitiveEnabled')}
          onCheckedChange={() => toggle('ModelNameCaseInsensitiveEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={t('setting_index.operationSettings.generalSettings.unifiedRequestResponseModel')}
          description={t('setting_index.operationSettings.generalSettings.unifiedRequestResponseModelDescription')}
          checked={is('UnifiedRequestResponseModelEnabled')}
          onCheckedChange={() => toggle('UnifiedRequestResponseModelEnabled')}
          disabled={loading}
        />
      </div>
    </SettingsCard>
  );
}

// 渠道自动禁用/恢复 — 自动开关、失败阈值与命中关键词。
export function ChannelAutoCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const is = (k) => inputs[k] === 'true';
  const keys = ['ChannelDisableThreshold', 'DisableChannelKeywords'];

  return (
    <SettingsCard title={t('setting_index.cards.channelAuto')} highlight="channel-auto">
      <div className="grid gap-3">
        <ToggleRow
          label={t('setting_index.operationSettings.monitoringSettings.automaticDisableChannel')}
          description={t('setting_index.operationSettings.monitoringSettings.automaticDisableChannelDescription')}
          checked={is('AutomaticDisableChannelEnabled')}
          onCheckedChange={() => toggle('AutomaticDisableChannelEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={t('setting_index.operationSettings.monitoringSettings.automaticEnableChannel')}
          description={t('setting_index.operationSettings.monitoringSettings.automaticEnableChannelDescription')}
          checked={is('AutomaticEnableChannelEnabled')}
          onCheckedChange={() => toggle('AutomaticEnableChannelEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={t('setting_index.operationSettings.monitoringSettings.automaticDisableChannelNotify')}
          description={t('setting_index.operationSettings.monitoringSettings.automaticDisableChannelNotifyDescription')}
          checked={is('AutomaticDisableChannelNotifyEnabled')}
          onCheckedChange={() => toggle('AutomaticDisableChannelNotifyEnabled')}
          disabled={loading}
        />
      </div>
      <TextRow
        id="ChannelDisableThreshold"
        type="number"
        label={t('setting_index.operationSettings.monitoringSettings.channelDisableThreshold.label')}
        value={inputs.ChannelDisableThreshold}
        onChange={(v) => setField('ChannelDisableThreshold', v)}
        placeholder={t('setting_index.operationSettings.monitoringSettings.channelDisableThreshold.placeholder')}
        description={t('setting_index.operationSettings.monitoringSettings.channelDisableThreshold.description')}
        disabled={loading}
      />
      <TextRow
        id="DisableChannelKeywords"
        label={t('setting_index.operationSettings.disableChannelKeywordsSettings.label')}
        description={t('setting_index.operationSettings.disableChannelKeywordsSettings.description')}
        value={inputs.DisableChannelKeywords}
        onChange={(v) => setField('DisableChannelKeywords', v)}
        placeholder={t('setting_index.operationSettings.disableChannelKeywordsSettings.info')}
        multiline
        disabled={loading}
      />
      <SaveButton loading={loading} disabled={!isDirty(keys)} onClick={() => saveKeys(keys)}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 计费规则 — 空回复计费与近似计费。
export function BillingRulesCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, toggle, loading } = ctx;
  const is = (k) => inputs[k] === 'true';

  return (
    <SettingsCard title={t('setting_index.cards.billingRules')} highlight="billing-rules">
      <div className="grid gap-3">
        <ToggleRow
          label={t('setting_index.operationSettings.generalSettings.emptyResponseBilling')}
          description={t('setting_index.operationSettings.generalSettings.emptyResponseBillingDescription')}
          checked={is('EmptyResponseBillingEnabled')}
          onCheckedChange={() => toggle('EmptyResponseBillingEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={t('setting_index.operationSettings.generalSettings.approximateToken')}
          description={t('setting_index.operationSettings.generalSettings.approximateTokenDescription')}
          checked={is('ApproximateTokenEnabled')}
          onCheckedChange={() => toggle('ApproximateTokenEnabled')}
          disabled={loading}
        />
      </div>
    </SettingsCard>
  );
}

// 额度 — 新用户额度、预扣额度与额度提醒阈值。
export function QuotaCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  // origin 是 string、QuotaInput 回传 number；归一避免 saveKeys 的 origin diff 误判触发冗余 PUT
  const setQuotaField = (key) => (v) => setField(key, typeof v === 'number' ? String(v) : v);
  const keys = ['QuotaForNewUser', 'PreConsumedQuota', 'QuotaRemindThreshold'];

  return (
    <SettingsCard title={t('setting_index.operationSettings.quotaSettings.title')} highlight="quota">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <QuotaInput
          id="QuotaForNewUser"
          name="QuotaForNewUser"
          label={t('setting_index.operationSettings.quotaSettings.quotaForNewUser.label')}
          value={inputs.QuotaForNewUser}
          onChange={setQuotaField('QuotaForNewUser')}
          helperText={t('setting_index.operationSettings.quotaSettings.quotaForNewUser.description')}
          disabled={loading}
        />
        <QuotaInput
          id="PreConsumedQuota"
          name="PreConsumedQuota"
          label={t('setting_index.operationSettings.quotaSettings.preConsumedQuota.label')}
          value={inputs.PreConsumedQuota}
          onChange={setQuotaField('PreConsumedQuota')}
          helperText={t('setting_index.operationSettings.quotaSettings.preConsumedQuota.description')}
          disabled={loading}
        />
        <TextRow
          id="QuotaRemindThreshold"
          type="number"
          label={t('setting_index.operationSettings.monitoringSettings.quotaRemindThreshold.label')}
          value={inputs.QuotaRemindThreshold}
          onChange={(v) => setField('QuotaRemindThreshold', v)}
          placeholder={t('setting_index.operationSettings.monitoringSettings.quotaRemindThreshold.placeholder')}
          description={t('setting_index.operationSettings.monitoringSettings.quotaRemindThreshold.description')}
          disabled={loading}
        />
      </div>
      <SaveButton loading={loading} disabled={!isDirty(keys)} onClick={() => saveKeys(keys)}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 邀请奖励 — 邀请人/被邀请人额度与充值奖励方式。
export function InviteRewardCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const is = (k) => inputs[k] === 'true';
  const setQuotaField = (key) => (v) => setField(key, typeof v === 'number' ? String(v) : v);
  const keys = ['QuotaForInviter', 'QuotaForInvitee', 'InviterRewardType', 'InviterRewardValue'];

  return (
    <SettingsCard title={t('setting_index.cards.inviteReward')} highlight="invite-reward">
      <ToggleRow
        label={t('setting_index.operationSettings.quotaSettings.inviteRewardEnabled.label')}
        description={t('setting_index.operationSettings.quotaSettings.inviteRewardEnabled.description')}
        checked={is('InviteRewardEnabled')}
        onCheckedChange={() => toggle('InviteRewardEnabled')}
        disabled={loading}
      />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <QuotaInput
          id="QuotaForInviter"
          name="QuotaForInviter"
          label={t('setting_index.operationSettings.quotaSettings.quotaForInviter.label')}
          value={inputs.QuotaForInviter}
          onChange={setQuotaField('QuotaForInviter')}
          helperText={t('setting_index.operationSettings.quotaSettings.quotaForInviter.description')}
          disabled={loading}
        />
        <QuotaInput
          id="QuotaForInvitee"
          name="QuotaForInvitee"
          label={t('setting_index.operationSettings.quotaSettings.quotaForInvitee.label')}
          value={inputs.QuotaForInvitee}
          onChange={setQuotaField('QuotaForInvitee')}
          helperText={t('setting_index.operationSettings.quotaSettings.quotaForInvitee.description')}
          disabled={loading}
        />
        <TextRow
          id="InviterRewardValue"
          type="number"
          label={t('setting_index.operationSettings.quotaSettings.rechargeRewardValue.label')}
          value={inputs.InviterRewardValue}
          onChange={(v) => setField('InviterRewardValue', v)}
          description={t('setting_index.operationSettings.quotaSettings.rechargeRewardValue.description')}
          disabled={loading}
        />
        <div className="space-y-4">
          <Label>{t('setting_index.operationSettings.quotaSettings.rechargeRewardType.label')}</Label>
          <Select value={inputs.InviterRewardType || 'fixed'} onValueChange={(v) => setField('InviterRewardType', v)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="fixed">{t('setting_index.operationSettings.quotaSettings.rechargeRewardType.fixed')}</SelectItem>
              <SelectItem value="percentage">{t('setting_index.operationSettings.quotaSettings.rechargeRewardType.percentage')}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      <SaveButton loading={loading} disabled={!isDirty(keys)} onClick={() => saveKeys(keys)}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 额度重置周期 — 时区与周起始日。
export function QuotaResetCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  // 周期限额重置：缺 key 时回退 UTC / 周一；自定义时区值也纳入下拉避免丢失
  const quotaResetTz = inputs.QuotaResetTimezone || 'UTC';
  const quotaResetWeekStart = inputs.QuotaResetWeekStart === 'sunday' ? 'sunday' : 'monday';
  const tzOptions = QUOTA_RESET_TIMEZONES.includes(quotaResetTz) ? QUOTA_RESET_TIMEZONES : [quotaResetTz, ...QUOTA_RESET_TIMEZONES];

  return (
    <SettingsCard
      title={t('setting_index.operationSettings.quotaResetSettings.title')}
      description={t('setting_index.operationSettings.quotaResetSettings.description')}
      highlight="quota-reset"
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label>{t('setting_index.operationSettings.quotaResetSettings.timezone.label')}</Label>
          <Select value={quotaResetTz} onValueChange={(v) => setField('QuotaResetTimezone', v)} disabled={loading}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {tzOptions.map((tz) => (
                <SelectItem key={tz} value={tz}>
                  {tzLabel(tz)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">{t('setting_index.operationSettings.quotaResetSettings.timezone.description')}</p>
        </div>
        <div className="space-y-2">
          <Label>{t('setting_index.operationSettings.quotaResetSettings.weekStart.label')}</Label>
          <Select value={quotaResetWeekStart} onValueChange={(v) => setField('QuotaResetWeekStart', v)} disabled={loading}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="monday">{t('setting_index.operationSettings.quotaResetSettings.weekStart.monday')}</SelectItem>
              <SelectItem value="sunday">{t('setting_index.operationSettings.quotaResetSettings.weekStart.sunday')}</SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">{t('setting_index.operationSettings.quotaResetSettings.weekStart.description')}</p>
        </div>
      </div>
      <SaveButton
        loading={loading}
        disabled={!isDirty(['QuotaResetTimezone', 'QuotaResetWeekStart'])}
        onClick={() => saveKeys(['QuotaResetTimezone', 'QuotaResetWeekStart'])}
      >
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}
