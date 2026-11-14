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
import { TextRow, ToggleRow } from '../parts';
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
        label={tr('generalSettings.serverAddress')}
        labelHidden
        value={inputs.ServerAddress}
        onChange={(v) => setField('ServerAddress', v)}
        placeholder={tr('generalSettings.serverAddressPlaceholder')}
        description={tr('generalSettings.serverAddressDescription')}
      />
      {isPlaceholderServerAddress(inputs.ServerAddress) && (
        <Alert variant="warning">
          <AlertTriangle />
          <span>{tr('generalSettings.serverAddressWarning', { suggested: window.location.origin })}</span>
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
  const text = (key, labelKey, phKey, extra = {}) => (
    <TextRow
      id={key}
      label={oth(labelKey)}
      value={inputs[key]}
      onChange={(v) => setField(key, v)}
      placeholder={phKey ? oth(phKey) : undefined}
      {...extra}
    />
  );

  return (
    <OptionCard ctx={ctx} id="branding">
      <Alert variant="warning">
        <AlertTriangle />
        <span>{oth('customSettings.copyrightWarning')}</span>
      </Alert>
      <div className="grid gap-5 sm:grid-cols-2">
        {text('SystemName', 'customSettings.systemNameLabel', 'customSettings.systemNamePlaceholder', {
          description: oth('customSettings.systemNameDescription')
        })}
        {text('Logo', 'customSettings.logoLabel', 'customSettings.logoPlaceholder', { description: oth('customSettings.logoDescription') })}
      </div>
      {text('HomePageContent', 'customSettings.homePageContentLabel', 'customSettings.homePageContentPlaceholder', { multiline: true })}
      {text('About', 'customSettings.aboutLabel', 'customSettings.aboutPlaceholder', { multiline: true })}
      {text('Footer', 'customSettings.footerLabel', 'customSettings.footerPlaceholder', { multiline: true, rows: 3 })}
      {text('AnalyticsCode', 'customSettings.analyticsCodeLabel', 'customSettings.analyticsCodePlaceholder', {
        description: oth('customSettings.analyticsCodeDescription'),
        multiline: true,
        rows: 3
      })}
      <FieldSection title={bi('title')} description={bi('description')}>
        <OptionList>
          <ToggleRow
            id="BrandIconSyncEnabled"
            label={bi('syncEnabled')}
            description={bi('syncEnabledDescription')}
            {...bindSwitch(ctx, 'BrandIconSyncEnabled')}
          />
          <ToggleRow
            id="BrandIconFaviconFetchEnabled"
            label={bi('faviconFetchEnabled')}
            description={bi('faviconFetchEnabledDescription')}
            {...bindSwitch(ctx, 'BrandIconFaviconFetchEnabled')}
          />
        </OptionList>
        <TextRow
          id="BrandIconRegistry"
          label={bi('registry')}
          value={inputs.BrandIconRegistry}
          onChange={(v) => setField('BrandIconRegistry', v)}
          placeholder="https://registry.npmmirror.com"
          description={bi('registryDescription')}
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
        <ToggleRow
          id="DisplayInCurrencyEnabled"
          label={gs('displayInCurrency')}
          description={gs('displayInCurrencyDescription')}
          {...bindSwitch(ctx, 'DisplayInCurrencyEnabled')}
        />
        <ToggleRow
          id="DisplayTokenStatEnabled"
          label={gs('displayTokenStat')}
          description={gs('displayTokenStatDescription')}
          {...bindSwitch(ctx, 'DisplayTokenStatEnabled')}
        />
      </OptionList>
      <TextRow
        id="QuotaPerUnit"
        label={gs('quotaPerUnit.label')}
        value={inputs.QuotaPerUnit}
        onChange={(v) => setField('QuotaPerUnit', v)}
        placeholder={gs('quotaPerUnit.placeholder')}
        description={gs('quotaPerUnit.description')}
      />
    </OptionCard>
  );
}

export function ExternalLinksCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const gs = (k) => t(`setting_index.operationSettings.generalSettings.${k}`);
  const link = (key, name) => (
    <TextRow
      id={key}
      label={gs(`${name}.label`)}
      value={inputs[key]}
      onChange={(v) => setField(key, v)}
      placeholder={gs(`${name}.placeholder`)}
      description={gs(`${name}.description`)}
    />
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
        label={oth('generalSettings.noticeLabel')}
        value={inputs.Notice}
        onChange={(v) => setField('Notice', v)}
        placeholder={oth('generalSettings.noticePlaceholder')}
        description={oth('generalSettings.noticeDescription')}
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
