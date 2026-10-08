import { useEffect, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { AlertTriangle } from 'lucide-react';
import { marked } from 'marked';

import { API } from 'utils/api';
import { showError, showSuccess, verifyJSON } from 'utils/common';
import { sanitizeHtml } from 'utils/sanitize';
import { Alert } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { SettingsCard, SettingsSection, TextRow, ToggleRow, HeaderSwitch, SaveButton, ConfirmAction, CollapsibleHelp } from './parts';
import ChatLinksEditor from './ChatLinksEditor';

// Cards from the former "Other" tab, exported one component per SettingsCard so
// the theme pages (site / billing / relay / privacy / integrations) can
// re-assemble them. Option keys preserved from v1.

// 版本更新提示 — version check plus the site notice.
export function VersionNoticeCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const oth = (k, opts) => t(`setting_index.otherSettings.${k}`, opts);
  const [update, setUpdate] = useState({ open: false, tag: '', html: '' });

  const checkUpdate = async () => {
    try {
      const ver = import.meta.env.VITE_APP_VERSION;
      if (!ver) return showError(oth('generalSettings.cannotGetVersion'));
      const url = ver.startsWith('v')
        ? 'https://api.github.com/repos/modeltaps/modeltaps/releases/latest'
        : 'https://api.github.com/repos/modeltaps/modeltaps/commits/main';
      const res = await API.get(url);
      const tag = ver.startsWith('v') ? res.data.tag_name : 'dev-' + res.data.sha.substr(0, 7);
      const body = ver.startsWith('v') ? res.data.body : res.data.commit.message;
      if (tag === ver) return showSuccess(oth('generalSettings.alreadyLatest', { tag }));
      setUpdate({ open: true, tag, html: sanitizeHtml(marked.parse(body)) });
    } catch (e) {
      /* ignore */
    }
  };

  return (
    <SettingsCard title={oth('versionNotice.title')} highlight="version-notice">
      <p className="text-sm text-muted-foreground">
        {oth('generalSettings.currentVersion')}: {import.meta.env.VITE_APP_VERSION}
      </p>
      <Button variant="outline" onClick={checkUpdate}>
        {oth('generalSettings.checkUpdate')}
      </Button>
      <TextRow
        id="Notice"
        label={oth('generalSettings.noticeLabel')}
        value={inputs.Notice}
        onChange={(v) => setField('Notice', v)}
        placeholder={oth('generalSettings.noticePlaceholder')}
        description={oth('generalSettings.noticeDescription')}
        multiline
        rows={4}
        disabled={loading}
      />
      <SaveButton loading={loading} disabled={!isDirty(['Notice'])} onClick={() => saveKeys(['Notice'])}>
        {t('common.save')}
      </SaveButton>

      <Dialog open={update.open} onOpenChange={(o) => setUpdate((u) => ({ ...u, open: o }))}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {oth('updateDialog.newVersion')}: {update.tag}
            </DialogTitle>
          </DialogHeader>
          <DialogBody>
            <div className="prose prose-sm dark:prose-invert max-w-none" dangerouslySetInnerHTML={{ __html: update.html }} />
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setUpdate((u) => ({ ...u, open: false }))}>
              {oth('updateDialog.close')}
            </Button>
            <Button
              onClick={() => {
                window.location = 'https://github.com/modeltaps/modeltaps/releases/latest';
              }}
            >
              {oth('updateDialog.viewGitHub')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsCard>
  );
}

// 品牌 — site name/logo/footer and other personalization.
export function BrandingCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const oth = (k, opts) => t(`setting_index.otherSettings.${k}`, opts);
  const keys = ['SystemName', 'Logo', 'HomePageContent', 'About', 'Footer', 'AnalyticsCode'];

  return (
    <SettingsCard title={oth('customSettings.title')} highlight="branding">
      <Alert variant="warning">
        <AlertTriangle />
        <span>{oth('customSettings.copyrightWarning')}</span>
      </Alert>
      <TextRow
        id="SystemName"
        label={oth('customSettings.systemNameLabel')}
        value={inputs.SystemName}
        onChange={(v) => setField('SystemName', v)}
        placeholder={oth('customSettings.systemNamePlaceholder')}
        description={oth('customSettings.systemNameDescription')}
        disabled={loading}
      />
      <TextRow
        id="Logo"
        label={oth('customSettings.logoLabel')}
        value={inputs.Logo}
        onChange={(v) => setField('Logo', v)}
        placeholder={oth('customSettings.logoPlaceholder')}
        description={oth('customSettings.logoDescription')}
        disabled={loading}
      />
      <TextRow
        id="HomePageContent"
        label={oth('customSettings.homePageContentLabel')}
        value={inputs.HomePageContent}
        onChange={(v) => setField('HomePageContent', v)}
        placeholder={oth('customSettings.homePageContentPlaceholder')}
        multiline
        disabled={loading}
      />
      <TextRow
        id="About"
        label={oth('customSettings.aboutLabel')}
        value={inputs.About}
        onChange={(v) => setField('About', v)}
        placeholder={oth('customSettings.aboutPlaceholder')}
        multiline
        disabled={loading}
      />
      <TextRow
        id="Footer"
        label={oth('customSettings.footerLabel')}
        value={inputs.Footer}
        onChange={(v) => setField('Footer', v)}
        placeholder={oth('customSettings.footerPlaceholder')}
        multiline
        disabled={loading}
      />
      <TextRow
        id="AnalyticsCode"
        label={oth('customSettings.analyticsCodeLabel')}
        value={inputs.AnalyticsCode}
        onChange={(v) => setField('AnalyticsCode', v)}
        placeholder={oth('customSettings.analyticsCodePlaceholder')}
        description={oth('customSettings.analyticsCodeDescription')}
        multiline
        disabled={loading}
      />
      <SaveButton loading={loading} disabled={!isDirty(keys)} onClick={() => saveKeys(keys)}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 原生协议 — Claude / Gemini passthrough switches and their tuning knobs.
export function NativeProtocolCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);
  const is = (k) => inputs[k] === 'true';

  return (
    <SettingsCard title={t('setting_index.cards.nativeProtocol')} highlight="native-protocol">
      <div className="grid gap-3">
        <ToggleRow
          label={op('otherSettings.claudeAPIEnabled')}
          description={op('otherSettings.claudeAPIEnabledDescription')}
          checked={is('ClaudeAPIEnabled')}
          onCheckedChange={() => toggle('ClaudeAPIEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={op('otherSettings.geminiAPIEnabled')}
          description={op('otherSettings.geminiAPIEnabledDescription')}
          checked={is('GeminiAPIEnabled')}
          onCheckedChange={() => toggle('GeminiAPIEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={op('otherSettings.claudePromptCachingEnabled')}
          description={op('otherSettings.claudePromptCachingEnabledDescription')}
          checked={is('ClaudePromptCachingEnabled')}
          onCheckedChange={() => toggle('ClaudePromptCachingEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={op('otherSettings.fingerprintPassThroughEnabled')}
          description={op('otherSettings.fingerprintPassThroughEnabledDescription')}
          checked={is('FingerprintPassThroughEnabled')}
          onCheckedChange={() => toggle('FingerprintPassThroughEnabled')}
          disabled={loading}
        />
      </div>

      <SettingsSection title={op('claudeSettings.title')}>
        <TextRow
          id="ClaudeBudgetTokensPercentage"
          type="number"
          label={op('claudeSettings.budgetTokensPercentage.label')}
          value={inputs.ClaudeBudgetTokensPercentage}
          onChange={(v) => setField('ClaudeBudgetTokensPercentage', v)}
          placeholder={op('claudeSettings.budgetTokensPercentage.placeholder')}
          description={op('claudeSettings.budgetTokensPercentage.description')}
          disabled={loading}
        />
        <TextRow
          id="ClaudeDefaultMaxTokens"
          label={op('claudeSettings.defaultMaxTokens.label')}
          value={inputs.ClaudeDefaultMaxTokens}
          onChange={(v) => setField('ClaudeDefaultMaxTokens', v)}
          placeholder={op('claudeSettings.defaultMaxTokens.placeholder')}
          description={op('claudeSettings.defaultMaxTokens.description')}
          multiline
          disabled={loading}
        />
        <SaveButton
          loading={loading}
          disabled={!isDirty(['ClaudeBudgetTokensPercentage', 'ClaudeDefaultMaxTokens'])}
          onClick={() =>
            saveKeys(['ClaudeBudgetTokensPercentage', 'ClaudeDefaultMaxTokens'], {
              validate: (i) => (i.ClaudeDefaultMaxTokens && !verifyJSON(i.ClaudeDefaultMaxTokens) ? op('claudeSettings.invalidJson') : null)
            })
          }
        >
          {t('common.save')}
        </SaveButton>
      </SettingsSection>

      <SettingsSection title={op('geminiSettings.title')}>
        <TextRow
          id="GeminiOpenThink"
          label={op('geminiSettings.geminiOpenThink.label')}
          value={inputs.GeminiOpenThink}
          onChange={(v) => setField('GeminiOpenThink', v)}
          placeholder={op('geminiSettings.geminiOpenThink.placeholder')}
          description={op('geminiSettings.geminiOpenThink.description')}
          multiline
          disabled={loading}
        />
        <SaveButton
          loading={loading}
          disabled={!isDirty(['GeminiOpenThink'])}
          onClick={() =>
            saveKeys(['GeminiOpenThink'], {
              validate: (i) => (i.GeminiOpenThink && !verifyJSON(i.GeminiOpenThink) ? op('geminiSettings.invalidJson') : null)
            })
          }
        >
          {t('common.save')}
        </SaveButton>
      </SettingsSection>
    </SettingsCard>
  );
}

// 请求限制 — maximum prompt tokens per request.
export function PromptLimitsCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);

  return (
    <SettingsCard title={op('promptLimits.title')} highlight="prompt-limits">
      <TextRow
        id="MaxPromptTokens"
        type="number"
        label={op('promptLimits.maxPromptTokens.label')}
        value={inputs.MaxPromptTokens}
        onChange={(v) => setField('MaxPromptTokens', v)}
        description={op('promptLimits.maxPromptTokens.description')}
        disabled={loading}
      />
      <SaveButton loading={loading} disabled={!isDirty(['MaxPromptTokens'])} onClick={() => saveKeys(['MaxPromptTokens'])}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 请求/响应留存 — log IO switches (deep-linked from the log detail dialog).
export function LogIOCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, toggle, loading } = ctx;
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);
  const is = (k) => inputs[k] === 'true';

  return (
    <SettingsCard title={t('setting_index.cards.logIO')} highlight="log-io">
      <div className="grid gap-3">
        <ToggleRow
          label={op('otherSettings.logIOEnabled')}
          description={op('otherSettings.logIOEnabledDescription')}
          checked={is('LogIOEnabled')}
          onCheckedChange={() => toggle('LogIOEnabled')}
          disabled={loading}
        />
        {is('LogIOEnabled') && (
          <>
            <ToggleRow
              label={op('otherSettings.logIODefaultUser')}
              description={op('otherSettings.logIODefaultUserDescription')}
              checked={is('LogIODefaultUser')}
              onCheckedChange={() => toggle('LogIODefaultUser')}
              disabled={loading}
            />
            <ToggleRow
              label={op('otherSettings.organizationLogIODefault')}
              description={op('otherSettings.organizationLogIODefaultDescription')}
              checked={is('OrganizationLogIODefault')}
              onCheckedChange={() => toggle('OrganizationLogIODefault')}
              disabled={loading}
            />
          </>
        )}
      </div>
    </SettingsCard>
  );
}

// Midjourney 回调 — MJ task notify switch.
export function MjNotifyCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, toggle, loading } = ctx;
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);

  return (
    <SettingsCard title={t('setting_index.cards.mjNotify')} highlight="mj-notify">
      <ToggleRow
        label={op('otherSettings.mjNotify')}
        description={op('otherSettings.mjNotifyDescription')}
        checked={inputs.MjNotifyEnabled === 'true'}
        onCheckedChange={() => toggle('MjNotifyEnabled')}
        disabled={loading}
      />
    </SettingsCard>
  );
}

// 品牌图标 — background icon-library sync and favicon fetching for unknown vendors.
export function BrandIconCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const bi = (k, opts) => t(`brandIconSettings.${k}`, opts);
  const is = (k) => inputs[k] === 'true';

  return (
    <SettingsCard title={bi('title')} description={bi('description')} highlight="brand-icon">
      <div className="grid gap-3">
        <ToggleRow
          label={bi('syncEnabled')}
          description={bi('syncEnabledDescription')}
          checked={is('BrandIconSyncEnabled')}
          onCheckedChange={() => toggle('BrandIconSyncEnabled')}
          disabled={loading}
        />
        <ToggleRow
          label={bi('faviconFetchEnabled')}
          description={bi('faviconFetchEnabledDescription')}
          checked={is('BrandIconFaviconFetchEnabled')}
          onCheckedChange={() => toggle('BrandIconFaviconFetchEnabled')}
          disabled={loading}
        />
      </div>
      <TextRow
        id="BrandIconRegistry"
        label={bi('registry')}
        value={inputs.BrandIconRegistry}
        onChange={(v) => setField('BrandIconRegistry', v)}
        placeholder="https://registry.npmmirror.com"
        description={bi('registryDescription')}
        disabled={loading}
      />
      <SaveButton loading={loading} disabled={!isDirty(['BrandIconRegistry'])} onClick={() => saveKeys(['BrandIconRegistry'])}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 内容安全 — moderation tool selection and keyword blocklist.
export function SafetyCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);
  const is = (k) => inputs[k] === 'true';
  const [safeTools, setSafeTools] = useState([]);

  useEffect(() => {
    API.get('/api/option/safe_tools')
      .then((res) => {
        if (res.data?.success) setSafeTools(res.data.data || []);
      })
      .catch(() => {});
  }, []);

  return (
    <SettingsCard
      title={op('safetySettings.title')}
      description={op('safetySettings.description')}
      headerAction={
        <HeaderSwitch id="EnableSafe" checked={is('EnableSafe')} onCheckedChange={() => toggle('EnableSafe')} disabled={loading} />
      }
      highlight="safety"
    >
      <div className="space-y-4">
        <Label>{op('safetySettings.safeToolName.label')}</Label>
        <Select value={inputs.SafeToolName || ''} onValueChange={(v) => setField('SafeToolName', v)}>
          <SelectTrigger>
            <SelectValue placeholder="..." />
          </SelectTrigger>
          <SelectContent>
            {safeTools.map((tool) => (
              <SelectItem key={tool} value={tool}>
                {tool}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">{op('safetySettings.safeToolName.description')}</p>
      </div>
      <TextRow
        id="SafeKeyWords"
        label={op('safetySettings.safeKeyWords.label')}
        value={Array.isArray(inputs.SafeKeyWords) ? inputs.SafeKeyWords.join('\n') : inputs.SafeKeyWords}
        onChange={(v) => setField('SafeKeyWords', v)}
        placeholder={op('safetySettings.safeKeyWords.placeholder')}
        description={op('safetySettings.safeKeyWords.description')}
        multiline
        disabled={loading}
      />
      <SaveButton
        loading={loading}
        disabled={!isDirty(['SafeToolName', 'SafeKeyWords'])}
        onClick={() => saveKeys(['SafeToolName', 'SafeKeyWords'])}
      >
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 图片代理 — chat image request proxy and Cloudflare worker image relay.
export function ImageProxyCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);
  const keys = ['ChatImageRequestProxy', 'CFWorkerImageUrl', 'CFWorkerImageKey'];

  return (
    <SettingsCard title={op('imageProxySettings.title')} highlight="image-proxy">
      <TextRow
        id="ChatImageRequestProxy"
        label={op('otherSettings.chatImageRequestProxy.label')}
        value={inputs.ChatImageRequestProxy}
        onChange={(v) => setField('ChatImageRequestProxy', v)}
        placeholder={op('otherSettings.chatImageRequestProxy.placeholder')}
        description={op('otherSettings.chatImageRequestProxy.description')}
        disabled={loading}
      />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <TextRow
          id="CFWorkerImageUrl"
          label={op('otherSettings.CFWorkerImageUrl.label')}
          value={inputs.CFWorkerImageUrl}
          onChange={(v) => setField('CFWorkerImageUrl', v)}
          description={op('otherSettings.CFWorkerImageUrl.description')}
          disabled={loading}
        />
        <TextRow
          id="CFWorkerImageKey"
          label={op('otherSettings.CFWorkerImageUrl.key')}
          description={op('otherSettings.CFWorkerImageUrl.keyDescription')}
          value={inputs.CFWorkerImageKey}
          onChange={(v) => setField('CFWorkerImageKey', v)}
          disabled={loading}
        />
      </div>
      <SaveButton loading={loading} disabled={!isDirty(keys)} onClick={() => saveKeys(keys)}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 内置聊天与聊天链接 — built-in chat toggle and external chat link list.
export function ChatCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);
  const is = (k) => inputs[k] === 'true';

  return (
    <SettingsCard
      title={op('chatLinkSettings.title')}
      description={op('chatLinkSettings.description')}
      headerAction={
        <HeaderSwitch
          id="BuiltinChatEnabled"
          label={op('chatLinkSettings.builtinChatEnabled')}
          checked={is('BuiltinChatEnabled')}
          onCheckedChange={() => toggle('BuiltinChatEnabled')}
          disabled={loading}
        />
      }
      highlight="chat"
    >
      <CollapsibleHelp
        trigger={op('chatLinkSettings.help.trigger')}
        lines={[op('chatLinkSettings.help.line1'), op('chatLinkSettings.help.line2'), op('chatLinkSettings.help.line3')]}
      />
      <ChatLinksEditor value={inputs.ChatLinks} onChange={(v) => setField('ChatLinks', v)} disabled={loading} />
      <SaveButton
        loading={loading}
        disabled={!isDirty(['ChatLinks'])}
        onClick={() =>
          saveKeys(['ChatLinks'], {
            validate: (i) => (i.ChatLinks && !verifyJSON(i.ChatLinks) ? op('chatLinkSettings.invalidJson') : null)
          })
        }
      >
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

// 消费日志 — consume log switches, auto-delete window and manual cleanup.
export function LogConsumeCard({ ctx }) {
  const { t, i18n } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);
  const is = (k) => inputs[k] === 'true';
  const [logTime, setLogTime] = useState('');

  const clearLogs = async () => {
    if (!logTime) return showError(op('logSettings.selectCleanupTime'));
    const ts = Math.floor(new Date(logTime).getTime() / 1000);
    const res = await API.delete(`/api/log/?target_timestamp=${ts}`);
    const { success, message, data } = res.data;
    if (success) showSuccess(op('logSettings.logsCleared', { count: data }));
    else showError(op('logSettings.cleanupFailed') + message);
  };

  // Localized echo of the selected cleanup time (#15): native datetime-local
  // renders in the browser locale, so mirror the value in the UI language.
  const intlLocale = (i18n.language || 'zh_CN').replace('_', '-');
  const logTimeText = logTime
    ? new Intl.DateTimeFormat(intlLocale, { dateStyle: 'long', timeStyle: 'short' }).format(new Date(logTime))
    : '';

  return (
    <SettingsCard title={op('logSettings.title')} highlight="log-consume">
      <ToggleRow
        label={op('logSettings.logConsume')}
        description={op('logSettings.logConsumeDescription')}
        checked={is('LogConsumeEnabled')}
        onCheckedChange={() => toggle('LogConsumeEnabled')}
        disabled={loading}
      />
      <ToggleRow
        label={op('logSettings.autoDelete')}
        description={op('logSettings.autoDeleteDescription')}
        checked={is('LogAutoDeleteEnabled')}
        onCheckedChange={() => toggle('LogAutoDeleteEnabled')}
        disabled={loading}
      />
      {is('LogAutoDeleteEnabled') && (
        <>
          <TextRow
            id="LogAutoDeleteDays"
            type="number"
            label={op('logSettings.autoDeleteDays')}
            value={inputs.LogAutoDeleteDays}
            onChange={(v) => setField('LogAutoDeleteDays', v)}
            description={op('logSettings.autoDeleteDaysDescription')}
            disabled={loading}
          />
          <SaveButton loading={loading} disabled={!isDirty(['LogAutoDeleteDays'])} onClick={() => saveKeys(['LogAutoDeleteDays'])}>
            {t('common.save')}
          </SaveButton>
        </>
      )}
      <div className="space-y-4">
        <Label htmlFor="logTime">{op('logSettings.logCleanupTime.label')}</Label>
        <Input id="logTime" type="datetime-local" value={logTime} onChange={(e) => setLogTime(e.target.value)} disabled={loading} />
        <p className="text-xs text-muted-foreground">
          {logTime ? op('logSettings.selectedTime', { time: logTimeText }) : op('logSettings.logCleanupTime.description')}
        </p>
      </div>
      <div className="flex justify-end">
        <ConfirmAction
          title={op('logSettings.clearLogs')}
          description={op('logSettings.confirmDescription', { time: logTimeText })}
          confirmLabel={t('common.confirm')}
          onConfirm={clearLogs}
          disabled={loading || !logTime}
        >
          {op('logSettings.clearLogs')}
        </ConfirmAction>
      </div>
    </SettingsCard>
  );
}

// 发票 — monthly invoice generation, only when the site enables user invoices.
export function InvoiceCard({ ctx }) {
  const { t } = useTranslation();
  const { loading } = ctx;
  const siteInfo = useSelector((state) => state.siteInfo);
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);
  const [invoiceMonth, setInvoiceMonth] = useState('');

  const invoiceAction = async (kind) => {
    if (!invoiceMonth) return showError('Please select invoice Month');
    const date = `${invoiceMonth}-01`;
    const res = await API.post(`/api/option/invoice/${kind}/${date}`);
    const { success, message } = res.data;
    if (success) showSuccess(kind === 'gen' ? op('invoice.genSuccess') : op('invoice.updateSuccess'));
    else showError((kind === 'gen' ? op('invoice.genFailed') : op('invoice.updateFailed')) + message);
  };

  if (!siteInfo?.UserInvoiceMonth) return null;

  return (
    <SettingsCard title={op('invoice.title')} highlight="invoice">
      <div className="space-y-4">
        <Label htmlFor="invoiceMonth">{op('invoice.genTime')}</Label>
        <Input id="invoiceMonth" type="month" value={invoiceMonth} onChange={(e) => setInvoiceMonth(e.target.value)} disabled={loading} />
      </div>
      <div className="flex justify-end gap-2">
        <Button onClick={() => invoiceAction('gen')} disabled={loading}>
          {op('invoice.genMonthInvoice')}
        </Button>
        <Button variant="outline" onClick={() => invoiceAction('update')} disabled={loading}>
          {op('invoice.updateMonthInvoice')}
        </Button>
      </div>
    </SettingsCard>
  );
}
