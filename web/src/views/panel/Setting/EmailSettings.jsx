import { useTranslation } from 'react-i18next';
import { Info } from 'lucide-react';

import { Alert } from '@/components/ui/alert';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { SettingsCard, TextRow, HeaderSwitch, SaveButton } from './parts';

const SMTP_TLS_MODES = ['auto', 'ssl', 'starttls', 'starttls_opportunistic', 'none'];

// Email cards: SMTP server configuration (「站点」theme) + email domain whitelist
// (「登录与注册」theme). The whitelist is stored as a comma-joined string (v1
// parity); edited one domain per line.

export function SmtpCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const sys = (k) => t(`setting_index.systemSettings.${k}`);
  const field = (key, labelKey, phKey, type = 'text') => (
    <TextRow
      id={key}
      type={type}
      label={labelKey}
      value={inputs[key]}
      onChange={(v) => setField(key, v)}
      placeholder={phKey}
      disabled={loading}
    />
  );
  const keys = ['SMTPServer', 'SMTPPort', 'SMTPAccount', 'SMTPFrom', 'SMTPToken', 'SMTPTLSMode'];
  const tlsMode = SMTP_TLS_MODES.includes(inputs.SMTPTLSMode) ? inputs.SMTPTLSMode : 'auto';

  return (
    <SettingsCard title={sys('configureSMTP.title')} description={sys('configureSMTP.subTitle')} highlight="smtp">
      <Alert variant="info">
        <Info />
        <span>{sys('configureSMTP.alert')}</span>
      </Alert>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {field('SMTPServer', sys('configureSMTP.smtpServer'), sys('configureSMTP.smtpServerPlaceholder'))}
        {field('SMTPPort', sys('configureSMTP.smtpPort'), sys('configureSMTP.smtpPortPlaceholder'))}
        {field('SMTPAccount', sys('configureSMTP.smtpAccount'), sys('configureSMTP.smtpAccountPlaceholder'))}
        {field('SMTPFrom', sys('configureSMTP.smtpFrom'), sys('configureSMTP.smtpFromPlaceholder'))}
        {field('SMTPToken', sys('configureSMTP.smtpToken'), sys('configureSMTP.smtpTokenPlaceholder'), 'password')}
        <div className="space-y-2">
          <Label htmlFor="SMTPTLSMode">{sys('configureSMTP.smtpTlsMode')}</Label>
          <Select value={tlsMode} onValueChange={(v) => setField('SMTPTLSMode', v)} disabled={loading}>
            <SelectTrigger id="SMTPTLSMode">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SMTP_TLS_MODES.map((mode) => (
                <SelectItem key={mode} value={mode}>
                  {sys(`configureSMTP.smtpTlsModes.${mode}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">{sys('configureSMTP.smtpTlsModeDescription')}</p>
        </div>
      </div>
      <SaveButton loading={loading} disabled={!isDirty(keys)} onClick={() => saveKeys(keys)}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}

export function EmailDomainCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const sys = (k) => t(`setting_index.systemSettings.${k}`);
  const is = (k) => inputs[k] === 'true';

  const whitelistText = (() => {
    const v = inputs.EmailDomainWhitelist;
    if (Array.isArray(v)) return v.join('\n');
    return (v || '').split(',').filter(Boolean).join('\n');
  })();

  return (
    <SettingsCard
      title={sys('configureEmailDomainWhitelist.title')}
      description={sys('configureEmailDomainWhitelist.subTitle')}
      headerAction={
        <HeaderSwitch
          id="EmailDomainRestrictionEnabled"
          checked={is('EmailDomainRestrictionEnabled')}
          onCheckedChange={() => toggle('EmailDomainRestrictionEnabled')}
          disabled={loading}
        />
      }
      highlight="email-domain"
    >
      <TextRow
        id="EmailDomainWhitelist"
        label={sys('configureEmailDomainWhitelist.allowedEmailDomains')}
        value={whitelistText}
        onChange={(v) =>
          setField(
            'EmailDomainWhitelist',
            v
              .split('\n')
              .map((s) => s.trim())
              .filter(Boolean)
              .join(',')
          )
        }
        placeholder="example.com"
        multiline
        rows={5}
        disabled={loading}
      />
      <SaveButton loading={loading} disabled={!isDirty(['EmailDomainWhitelist'])} onClick={() => saveKeys(['EmailDomainWhitelist'])}>
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}
