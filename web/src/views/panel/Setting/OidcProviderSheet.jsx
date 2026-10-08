import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { FormField } from '@/components/ui/form-field';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError } from 'utils/common';

const tk = (k) => `setting_index.oidcProviders.${k}`;

const ORIGIN = {
  slug: '',
  display_name: '',
  issuer: '',
  client_id: '',
  client_secret: '',
  scopes: 'openid,email,profile',
  username_claim: 'preferred_username',
  display_name_claim: 'name',
  avatar_claim: 'picture',
  link_by_verified_email: false,
  link_by_verified_phone: false,
  disable_auto_register: false,
  first_party: false,
  account_settings_url: '',
  password_url: '',
  mfa_url: '',
  passkey_url: '',
  identity_url: '',
  enabled: true,
  sort: 0
};

export default function OidcProviderSheet({ open, onOpenChange, editing, serverAddress, onSaved }) {
  const { t } = useTranslation();
  const [values, setValues] = useState(ORIGIN);
  const [saving, setSaving] = useState(false);
  // discovery 失败时后端返回 4xx，把原文留在这里并解锁「仍保存为停用」。
  const [discoveryError, setDiscoveryError] = useState('');
  const isEdit = !!editing;

  const setField = (key, value) => setValues((prev) => ({ ...prev, [key]: value }));

  useEffect(() => {
    if (!open) return;
    setDiscoveryError('');
    if (editing) {
      setValues({
        slug: editing.slug || '',
        display_name: editing.display_name || '',
        issuer: editing.issuer || '',
        client_id: editing.client_id || '',
        client_secret: '',
        scopes: editing.scopes || '',
        username_claim: editing.username_claim || '',
        display_name_claim: editing.display_name_claim || '',
        avatar_claim: editing.avatar_claim || '',
        link_by_verified_email: !!editing.link_by_verified_email,
        link_by_verified_phone: !!editing.link_by_verified_phone,
        disable_auto_register: !!editing.disable_auto_register,
        first_party: !!editing.first_party,
        account_settings_url: editing.account_settings_url || '',
        password_url: editing.password_url || '',
        mfa_url: editing.mfa_url || '',
        passkey_url: editing.passkey_url || '',
        identity_url: editing.identity_url || '',
        enabled: !!editing.enabled,
        sort: editing.sort ?? 0
      });
    } else {
      setValues(ORIGIN);
    }
  }, [open, editing]);

  const submit = async (force = false) => {
    if (saving) return;
    setSaving(true);
    try {
      const payload = { ...values, sort: parseInt(values.sort, 10) || 0 };
      const query = force ? '?force=true' : '';
      const res = isEdit
        ? await API.put(`/api/oidc_provider/${editing.id}${query}`, payload)
        : await API.post(`/api/oidc_provider/${query}`, payload);
      const { success, message } = res.data;
      if (success) {
        toast.success(message || (isEdit ? t(tk('editOk')) : t(tk('createOk'))));
        setDiscoveryError('');
        onSaved?.();
      } else {
        setDiscoveryError(message || '');
        showError(message);
      }
    } catch (error) {
      // 4xx 里的 message 已被 utils/api.js 拦截器搬到 error.message 并弹过 toast，这里只留原文
      setDiscoveryError(error.message || '');
    } finally {
      setSaving(false);
    }
  };

  // 与后端 common/oidc.RedirectURL 同口径：存量 slug=oidc 用无 slug 的旧地址。
  const callbackUrl =
    values.slug === 'oidc' ? `${serverAddress || ''}/oauth/oidc` : `${serverAddress || ''}/oauth/oidc/${values.slug || ':slug'}`;

  const text = (key, label, opts = {}) => (
    <FormField id={`oidc-provider-${key}`} label={label} hint={opts.hint}>
      <Input
        id={`oidc-provider-${key}`}
        type={opts.type || 'text'}
        value={values[key]}
        placeholder={opts.placeholder}
        disabled={opts.disabled}
        onChange={(e) => setField(key, opts.sanitize ? opts.sanitize(e.target.value) : e.target.value)}
      />
    </FormField>
  );

  const toggle = (key, label, hint) => (
    <FormField id={`oidc-provider-${key}`} label={label} hint={hint} row>
      <div className="flex justify-end">
        <Switch id={`oidc-provider-${key}`} checked={!!values[key]} onCheckedChange={(v) => setField(key, v)} />
      </div>
    </FormField>
  );

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{isEdit ? t('common.edit') : t('common.create')}</SheetTitle>
          <SheetDescription>{t(tk('title'))}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          {discoveryError && (
            <Alert variant="error">
              <AlertDescription className="whitespace-pre-wrap break-all">{discoveryError}</AlertDescription>
            </Alert>
          )}

          {text('slug', t(tk('form.slug')), {
            hint: isEdit ? t(tk('form.slugHintEdit')) : t(tk('form.slugHint')),
            placeholder: t(tk('form.slugPlaceholder')),
            disabled: isEdit,
            sanitize: (v) => v.toLowerCase().replace(/[^a-z0-9-]/g, '')
          })}

          <FormField id="oidc-provider-callback" label={t(tk('form.callbackUrl'))} hint={t(tk('form.callbackUrlHint'))}>
            <Input id="oidc-provider-callback" value={callbackUrl} readOnly />
          </FormField>

          {text('display_name', t(tk('form.displayName')), { placeholder: t(tk('form.displayNamePlaceholder')) })}
          {text('issuer', t(tk('form.issuer')), { placeholder: t(tk('form.issuerPlaceholder')) })}
          {text('client_id', t(tk('form.clientId')), { placeholder: t(tk('form.clientIdPlaceholder')) })}
          {text('client_secret', t(tk('form.clientSecret')), {
            type: 'password',
            placeholder: isEdit ? t(tk('form.clientSecretPlaceholderEdit')) : t(tk('form.clientSecretPlaceholder'))
          })}
          {text('scopes', t(tk('form.scopes')), { placeholder: t(tk('form.scopesPlaceholder')) })}
          {text('username_claim', t(tk('form.usernameClaim')), { placeholder: 'preferred_username' })}
          {text('display_name_claim', t(tk('form.displayNameClaim')), { placeholder: 'name' })}
          {text('avatar_claim', t(tk('form.avatarClaim')), { placeholder: 'picture' })}
          {text('sort', t(tk('form.sort')), { type: 'number', hint: t(tk('form.sortHint')) })}

          {toggle('link_by_verified_email', t(tk('form.linkByVerifiedEmail')), t(tk('form.linkByVerifiedEmailHint')))}
          {toggle('link_by_verified_phone', t(tk('form.linkByVerifiedPhone')), t(tk('form.linkByVerifiedPhoneHint')))}
          {toggle('disable_auto_register', t(tk('form.disableAutoRegister')), t(tk('form.disableAutoRegisterHint')))}
          {toggle('first_party', t(tk('form.firstParty')), t(tk('form.firstPartyHint')))}
          {text('account_settings_url', t(tk('form.accountSettingsUrl')), {
            hint: t(tk('form.accountSettingsUrlHint')),
            placeholder: t(tk('form.accountSettingsUrlPlaceholder'))
          })}
          {/* 四个设置页深链:账号安全页各行按此跳转,留空的行不显示 */}
          {text('password_url', t(tk('form.passwordUrl')), {
            hint: t(tk('form.deepLinkHint')),
            placeholder: 'https://auth.example.com/settings/change_password'
          })}
          {text('mfa_url', t(tk('form.mfaUrl')), { placeholder: 'https://auth.example.com/settings/mfa' })}
          {text('passkey_url', t(tk('form.passkeyUrl')), { placeholder: 'https://auth.example.com/settings/passkey' })}
          {text('identity_url', t(tk('form.identityUrl')), { placeholder: 'https://auth.example.com/settings/identity' })}
          {toggle('enabled', t(tk('form.enabled')))}
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          {discoveryError && (
            <Button variant="outline" onClick={() => submit(true)} disabled={saving}>
              {t(tk('forceSave'))}
            </Button>
          )}
          <Button onClick={() => submit(false)} disabled={saving}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
