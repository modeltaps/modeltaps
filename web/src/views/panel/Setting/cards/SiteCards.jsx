import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AlertTriangle } from 'lucide-react';
import { marked } from 'marked';

import { API } from 'utils/api';
import { removeTrailingSlash, showError, showSuccess } from 'utils/common';
import { isPlaceholderServerAddress } from 'utils/serverAddress';
import { sanitizeHtml } from 'utils/sanitize';
import { Alert } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { ActionRow, FieldSection, OptionList } from '@/components/settings/cards';
import i18nList from 'i18n/i18nList';
import { SelectRow, TextRow, ToggleRow } from '../parts';
import OptionCard, { bindSwitch } from './OptionCard';

// ==============================|| PLATFORM SETTINGS — 站点 ||============================== //
// Server address, branding (with the provider icon library), display, external links and the
// site notice with the version check.

export function ServerAddressCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const tr = (k, opts) => t(`setting_index.systemSettings.${k}`, opts);

  return (
    <OptionCard ctx={ctx} id="server-address" options={{ transform: { ServerAddress: (v) => removeTrailingSlash(v || '') } }}>
      <TextRow
        id="ServerAddress"
        labelHidden
        value={inputs.ServerAddress}
        onChange={(v) => setField('ServerAddress', v)}
        placeholder={tr('generalSettings.serverAddressPlaceholder')}
      />
      {isPlaceholderServerAddress(inputs.ServerAddress) && (
        <Alert variant="warning">
          <AlertTriangle />
          <span>{tr('generalSettings.serverAddressWarning', { suggested: window.location.origin })}</span>
        </Alert>
      )}
      {/* A deployment setting like the server address: shown until ENCRYPTION_KEY is set. */}
      {ctx.encryptionEnabled === false && (
        <Alert variant="warning">
          <AlertTriangle />
          <span>{tr('generalSettings.encryptionKeyMissing')}</span>
        </Alert>
      )}
    </OptionCard>
  );
}

export function BrandingCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const oth = (k, opts) => t(`setting_index.otherSettings.${k}`, opts);
  const bi = (k, opts) => t(`brandIconSettings.${k}`, opts);
  const text = (key, phKey, extra = {}) => (
    <TextRow id={key} value={inputs[key]} onChange={(v) => setField(key, v)} placeholder={phKey ? oth(phKey) : undefined} {...extra} />
  );

  return (
    <OptionCard ctx={ctx} id="branding">
      <Alert variant="warning">
        <AlertTriangle />
        <span>{oth('customSettings.copyrightWarning')}</span>
      </Alert>
      <div className="grid gap-5 sm:grid-cols-2">
        {text('SystemName', 'customSettings.systemNamePlaceholder')}
        {text('Logo', 'customSettings.logoPlaceholder')}
      </div>
      {text('HomePageContent', 'customSettings.homePageContentPlaceholder', { multiline: true })}
      {text('About', 'customSettings.aboutPlaceholder', { multiline: true })}
      {text('Footer', 'customSettings.footerPlaceholder', { multiline: true, rows: 3 })}
      {text('AnalyticsCode', 'customSettings.analyticsCodePlaceholder', { multiline: true, rows: 3 })}
      <FieldSection title={bi('title')} description={bi('description')}>
        <OptionList>
          <ToggleRow id="BrandIconSyncEnabled" {...bindSwitch(ctx, 'BrandIconSyncEnabled')} />
          <ToggleRow id="BrandIconFaviconFetchEnabled" {...bindSwitch(ctx, 'BrandIconFaviconFetchEnabled')} />
        </OptionList>
        <TextRow
          id="BrandIconRegistry"
          value={inputs.BrandIconRegistry}
          onChange={(v) => setField('BrandIconRegistry', v)}
          placeholder="https://registry.npmmirror.com"
        />
      </FieldSection>
    </OptionCard>
  );
}

export function DisplayCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const gs = (k) => t(`setting_index.operationSettings.generalSettings.${k}`);

  return (
    <OptionCard ctx={ctx} id="display">
      <OptionList>
        <ToggleRow id="DisplayInCurrencyEnabled" {...bindSwitch(ctx, 'DisplayInCurrencyEnabled')} />
        <ToggleRow id="DisplayTokenStatEnabled" {...bindSwitch(ctx, 'DisplayTokenStatEnabled')} />
      </OptionList>
      <TextRow
        id="QuotaPerUnit"
        value={inputs.QuotaPerUnit}
        onChange={(v) => setField('QuotaPerUnit', v)}
        placeholder={gs('quotaPerUnit.placeholder')}
      />
    </OptionCard>
  );
}

// The site default language: emails to people whose language is unknown, and the home page for
// browsers in a language the console does not have. The options keep each language's own name.
export function SiteLanguageCard({ ctx }) {
  const { inputs, setField } = ctx;
  return (
    <OptionCard ctx={ctx} id="site-language">
      <SelectRow
        id="DefaultLanguage"
        value={inputs.DefaultLanguage || 'en_US'}
        onChange={(v) => setField('DefaultLanguage', v)}
        options={i18nList.map((item) => ({ value: item.lng, label: item.name }))}
      />
    </OptionCard>
  );
}

export function ExternalLinksCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const gs = (k) => t(`setting_index.operationSettings.generalSettings.${k}`);
  const link = (key, name) => (
    <TextRow id={key} value={inputs[key]} onChange={(v) => setField(key, v)} placeholder={gs(`${name}.placeholder`)} />
  );

  return (
    <OptionCard ctx={ctx} id="external-links">
      <div className="grid gap-5 sm:grid-cols-2">
        {link('TopUpLink', 'topUpLink')}
        {link('ChatLink', 'chatLink')}
        {link('DocsLink', 'docsLink')}
      </div>
    </OptionCard>
  );
}

const RELEASES_URL = 'https://github.com/modeltaps/modeltaps/releases/latest';

export function VersionNoticeCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const oth = (k, opts) => t(`setting_index.otherSettings.${k}`, opts);
  const [update, setUpdate] = useState({ open: false, tag: '', html: '' });
  const version = import.meta.env.VITE_APP_VERSION;

  const checkUpdate = async () => {
    try {
      if (!version) return showError(oth('generalSettings.cannotGetVersion'));
      const url = version.startsWith('v')
        ? 'https://api.github.com/repos/modeltaps/modeltaps/releases/latest'
        : 'https://api.github.com/repos/modeltaps/modeltaps/commits/main';
      const res = await API.get(url);
      const tag = version.startsWith('v') ? res.data.tag_name : 'dev-' + res.data.sha.substr(0, 7);
      const body = version.startsWith('v') ? res.data.body : res.data.commit.message;
      if (tag === version) return showSuccess(oth('generalSettings.alreadyLatest', { tag }));
      setUpdate({ open: true, tag, html: sanitizeHtml(marked.parse(body)) });
    } catch (e) {
      /* ignore */
    }
  };

  return (
    <OptionCard
      ctx={ctx}
      id="version-notice"
      actions={
        <ActionRow
          title={oth('generalSettings.checkUpdate')}
          description={t('settingsUi.platform.actions.currentVersion', { version: version || '-' })}
          actionLabel={t('settingsUi.platform.actions.check')}
          onAction={checkUpdate}
        />
      }
    >
      <TextRow
        id="Notice"
        value={inputs.Notice}
        onChange={(v) => setField('Notice', v)}
        placeholder={oth('generalSettings.noticePlaceholder')}
        multiline
        rows={4}
      />

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
                window.location = RELEASES_URL;
              }}
            >
              {oth('updateDialog.viewGitHub')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </OptionCard>
  );
}
