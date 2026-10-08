import { useEffect, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Alert } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { FormField } from '@/components/ui/form-field';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import { toast } from '@/components/ui/sonner';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import ModelLimitSelector from './ModelLimitSelector';

const ORIGIN = {
  is_edit: false,
  name: '',
  remain_quota: '',
  expired_time: -1,
  unlimited_quota: true,
  group: '',
  backup_group: '',
  setting: {
    heartbeat: { enabled: false, timeout_seconds: 30 },
    limits: {
      limit_model_setting: { enabled: false, models: [] },
      limits_ip_setting: { enabled: false, whitelist: [] }
    }
  }
};

const pad = (n) => String(n).padStart(2, '0');
const toLocal = (unix) => {
  const d = new Date(unix * 1000);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
const fromLocal = (s) => Math.floor(new Date(s).getTime() / 1000);

function Section({ title, description, children }) {
  return (
    <section className="py-6 first:pt-0 last:pb-0">
      <h3 className="text-sm font-semibold text-foreground">{title}</h3>
      {description && <p className="mt-1 text-xs text-muted-foreground">{description}</p>}
      <div className="mt-4 space-y-4">{children}</div>
    </section>
  );
}

function FeatureToggle({ label, description, checked, onCheckedChange, children }) {
  return (
    <div className="space-y-3">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-2.5">
          <Label className="font-medium">{label}</Label>
          {description && <p className="text-xs text-muted-foreground">{description}</p>}
        </div>
        <div className="mt-0.5 shrink-0">
          <Switch checked={checked} onCheckedChange={onCheckedChange} />
        </div>
      </div>
      {checked && children}
    </div>
  );
}

const RESET_PERIODS = ['__none', 'daily', 'weekly', 'monthly'];

function ToggleRow({ label, checked, onCheckedChange }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1">
      <Label className="font-normal">{label}</Label>
      <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  );
}

export default function TokenSheet({
  open,
  onOpenChange,
  tokenId,
  adminMode = false,
  userGroupOptions = [],
  userGroup = {},
  userIsReliable = false,
  onSaved
}) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const [values, setValues] = useState(ORIGIN);
  const [models, setModels] = useState([]);
  const [saving, setSaving] = useState(false);
  const [nameError, setNameError] = useState('');
  const [quotaError, setQuotaError] = useState('');
  const [periodLimitError, setPeriodLimitError] = useState('');

  const setField = (path, value) => {
    setValues((prev) => {
      const next = structuredClone(prev);
      const keys = path.split('.');
      let o = next;
      for (let i = 0; i < keys.length - 1; i++) o = o[keys[i]];
      o[keys[keys.length - 1]] = value;
      return next;
    });
  };

  const getGroupLabel = (symbol) => userGroup?.[symbol]?.name || symbol;

  useEffect(() => {
    if (!open) return;
    setNameError('');
    setQuotaError('');
    setPeriodLimitError('');
    API.get('/api/available_model')
      .then((res) => {
        const { success, data } = res.data;
        if (success) {
          setModels(Object.keys(data).map((id) => ({ id, name: id, owned_by: data[id].owned_by, groups: data[id].groups })));
        }
      })
      .catch(showError);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    if (!tokenId) {
      setValues(ORIGIN);
      return;
    }
    const load = async () => {
      try {
        const res = adminMode
          ? await API.get('/api/token/admin/search', { params: { token_id: tokenId, page: 1, size: 1 } })
          : await API.get(`/api/token/${tokenId}`);
        const { success, message, data } = res.data;
        if (!success) {
          showError(message);
          onOpenChange(false);
          return;
        }
        const d = adminMode ? data?.data?.[0] : data;
        if (!d) {
          showError(t('usagePage.loadFailed'));
          onOpenChange(false);
          return;
        }
        d.is_edit = true;
        // OpenRouter 风格：单一额度框。设了周期 → 额度=quota_reset.limit；否则 unlimited → 空值
        if (d.setting?.quota_reset?.period) {
          d.remain_quota = d.setting.quota_reset.limit ?? '';
        } else {
          d.remain_quota = d.unlimited_quota ? '' : d.remain_quota;
        }
        d.setting = { ...ORIGIN.setting, ...(d.setting || {}) };
        d.setting.limits = { ...ORIGIN.setting.limits, ...(d.setting.limits || {}) };
        d.setting.heartbeat = { ...ORIGIN.setting.heartbeat, ...(d.setting.heartbeat || {}) };
        d.setting.limits.limit_model_setting = {
          ...ORIGIN.setting.limits.limit_model_setting,
          ...(d.setting.limits.limit_model_setting || {})
        };
        d.setting.limits.limits_ip_setting = { ...ORIGIN.setting.limits.limits_ip_setting, ...(d.setting.limits.limits_ip_setting || {}) };
        setValues(d);
      } catch (error) {
        showError(error);
        onOpenChange(false);
      }
    };
    load();
  }, [open, tokenId, adminMode]);

  const submit = async () => {
    if (!values.name?.trim()) {
      setNameError(t('token_index.name') + ' *');
      return;
    }
    const quotaEmpty = values.remain_quota === '' || values.remain_quota == null;
    const quotaVal = quotaEmpty ? 0 : parseInt(values.remain_quota, 10);
    if (!quotaEmpty && (!Number.isFinite(quotaVal) || quotaVal < 0)) {
      setQuotaError(t('redemption_edit.requiredQuota'));
      return;
    }
    const period = values.setting?.quota_reset?.period;
    // 选了周期则额度即为每周期上限，必须 > 0
    if (period && (quotaEmpty || quotaVal <= 0)) {
      setPeriodLimitError(t('token_index.quotaResetLimitRequired'));
      return;
    }
    setSaving(true);
    const payload = structuredClone(values);
    payload.setting.heartbeat.timeout_seconds = parseInt(payload.setting.heartbeat.timeout_seconds, 10) || 30;
    // 单一额度 + 重置周期：三种提交映射
    if (period) {
      // 设了周期：额度=每周期上限走 quota_reset，主额度不限额；period_used/period_start 由服务端维护
      payload.unlimited_quota = true;
      payload.remain_quota = 0;
      payload.setting.quota_reset = { ...(payload.setting.quota_reset || {}), period, limit: quotaVal };
    } else if (quotaEmpty) {
      // 留空：不限额
      payload.unlimited_quota = true;
      payload.remain_quota = 0;
      delete payload.setting.quota_reset;
    } else {
      // 有值无周期：限额，用完即停用
      payload.unlimited_quota = false;
      payload.remain_quota = quotaVal;
      delete payload.setting.quota_reset;
    }
    if (payload.setting.limits.limits_ip_setting.whitelist) {
      payload.setting.limits.limits_ip_setting.whitelist = payload.setting.limits.limits_ip_setting.whitelist.filter(
        (ip) => ip.trim() !== ''
      );
    }
    try {
      let res;
      if (values.is_edit) {
        const apiPath = adminMode ? '/api/token/admin' : '/api/token/';
        payload.id = parseInt(tokenId, 10);
        if (adminMode && values.user_id) payload.user_id = parseInt(values.user_id, 10);
        res = await API.put(apiPath, payload);
      } else {
        res = await API.post('/api/token/', payload);
      }
      const { success, message } = res.data;
      if (success) {
        toast.success(values.is_edit ? t('token_index.editToken') : t('token_index.createToken'));
        onSaved?.();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  const groupLabel = (val, fallback) => {
    if (!val) return fallback;
    return userGroupOptions.find((o) => o.value === val)?.label || `${val}`;
  };

  const resetPeriod = values.setting?.quota_reset?.period || '';
  const hasResetPeriod = Boolean(resetPeriod);
  const quotaLabel = hasResetPeriod ? t('token_index.quotaPeriodLabel') : t('token_index.quota');
  const quotaHelper = hasResetPeriod ? t('token_index.quotaPeriodHint') : t('token_index.quotaUnlimitedHint');
  const quotaPlaceholder = hasResetPeriod ? '0' : t('token_index.noLimit');
  // 缺 key 时回退 UTC / 周一，兼容旧后端
  const resetTz = siteInfo?.quota_reset_timezone || 'UTC';
  const resetWeekStart = siteInfo?.quota_reset_week_start === 'sunday' ? 'sunday' : 'monday';
  const resetTzNote = t('token_index.quotaResetTimezoneNote', {
    tz: resetTz,
    weekStart: t(resetWeekStart === 'sunday' ? 'token_index.weekStartSunday' : 'token_index.weekStartMonday')
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{tokenId ? t('token_index.editToken') : t('token_index.createToken')}</SheetTitle>
        </SheetHeader>
        <SheetBody>
          <div className="divide-y divide-border">
            <Section title={t('token_index.sectionBasic')}>
              {adminMode && values.is_edit && (
                <FormField id="token-user-id" label={t('token_index.transferToUser')} hint={t('token_index.transferToUserHelper')}>
                  <Input
                    id="token-user-id"
                    type="number"
                    value={values.user_id || ''}
                    onChange={(e) => setField('user_id', e.target.value)}
                  />
                </FormField>
              )}

              <FormField id="token-name" label={t('token_index.name')} required error={nameError}>
                <Input id="token-name" value={values.name} onChange={(e) => setField('name', e.target.value)} />
              </FormField>

              <div className="space-y-3">
                <ToggleRow
                  label={t('token_index.neverExpires')}
                  checked={values.expired_time === -1}
                  onCheckedChange={(c) => setField('expired_time', c ? -1 : Math.floor(Date.now() / 1000))}
                />
                {values.expired_time !== -1 && (
                  <FormField id="token-expiry-time" label={t('token_index.expiryTime')}>
                    <Input
                      id="token-expiry-time"
                      type="datetime-local"
                      value={values.expired_time > 0 ? toLocal(values.expired_time) : ''}
                      onChange={(e) => setField('expired_time', e.target.value ? fromLocal(e.target.value) : -1)}
                    />
                  </FormField>
                )}
              </div>
            </Section>

            <Section title={t('token_index.sectionQuota')}>
              <div className="space-y-1.5">
                <QuotaInput
                  name="remain_quota"
                  label={quotaLabel}
                  value={values.remain_quota}
                  placeholder={quotaPlaceholder}
                  helperText={quotaHelper}
                  allowEmpty={!hasResetPeriod}
                  onChange={(v) => {
                    setQuotaError('');
                    setPeriodLimitError('');
                    setField('remain_quota', v);
                  }}
                />
                {quotaError && <p className="text-sm text-destructive">{quotaError}</p>}
                {periodLimitError && <p className="text-sm text-destructive">{periodLimitError}</p>}
              </div>

              <FormField
                id="token-quota-reset-period"
                label={t('token_index.quotaResetPeriod')}
                hint={hasResetPeriod ? `${t('token_index.quotaResetHint')} ${resetTzNote}` : undefined}
              >
                <Select
                  value={resetPeriod || '__none'}
                  onValueChange={(v) => {
                    setPeriodLimitError('');
                    if (v === '__none') setField('setting.quota_reset', null);
                    else setField('setting.quota_reset', { ...(values.setting?.quota_reset || {}), period: v });
                  }}
                >
                  <SelectTrigger id="token-quota-reset-period">
                    <span className="truncate">
                      {hasResetPeriod ? t(`token_index.quotaReset_${resetPeriod}`) : t('token_index.quotaResetNone')}
                    </span>
                  </SelectTrigger>
                  <SelectContent>
                    {RESET_PERIODS.map((p) => (
                      <SelectItem key={p} value={p}>
                        {p === '__none' ? t('token_index.quotaResetNone') : t(`token_index.quotaReset_${p}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>

              <Alert variant="info">{t('token_index.quotaNote')}</Alert>
            </Section>

            <Section title={t('token_index.channelGroup')} description={t('token_index.selectGroupInfo')}>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField id="token-group" label={t('token_index.userGroup')}>
                  <Select
                    value={values.group || '-1'}
                    onValueChange={(v) => {
                      const val = v === '-1' ? '' : v;
                      setField('group', val);
                      if (values.backup_group === val && val !== '') setField('backup_group', '');
                    }}
                  >
                    <SelectTrigger id="token-group">
                      <span className="truncate">{values.group ? groupLabel(values.group) : t('token_index.selectGroup')}</span>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="-1">{t('token_index.selectGroup')}</SelectItem>
                      {userGroupOptions.map((o) => (
                        <SelectItem key={o.value} value={o.value}>
                          {o.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </FormField>
                <FormField id="token-backup-group" label={t('token_index.userBackupGroup')}>
                  <Select value={values.backup_group || '-1'} onValueChange={(v) => setField('backup_group', v === '-1' ? '' : v)}>
                    <SelectTrigger id="token-backup-group">
                      <span className="truncate">{values.backup_group ? groupLabel(values.backup_group) : '-'}</span>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="-1">-</SelectItem>
                      {userGroupOptions
                        .filter((o) => o.value !== values.group || values.group === '')
                        .map((o) => (
                          <SelectItem key={o.value} value={o.value}>
                            {o.label}
                          </SelectItem>
                        ))}
                    </SelectContent>
                  </Select>
                </FormField>
              </div>
              {userIsReliable && (
                <FormField id="token-billing-tag" label={t('token_index.billingTag')} hint={t('token_index.billingTagHelper')}>
                  <Select
                    value={values.setting?.billing_tag || '__none'}
                    onValueChange={(v) => setField('setting.billing_tag', v === '__none' ? null : v)}
                  >
                    <SelectTrigger id="token-billing-tag">
                      <span className="truncate">{values.setting?.billing_tag ? groupLabel(values.setting.billing_tag) : '-'}</span>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__none">-</SelectItem>
                      {userGroupOptions.map((o) => (
                        <SelectItem key={o.value} value={o.value}>
                          {o.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </FormField>
              )}
            </Section>

            <Section title={t('token_index.limits')}>
              <FeatureToggle
                label={t('token_index.limits_models_switch')}
                description={t('token_index.limits_info')}
                checked={values.setting?.limits?.limit_model_setting?.enabled === true}
                onCheckedChange={(c) => {
                  setField('setting.limits.limit_model_setting.enabled', c);
                  if (!c) setField('setting.limits.limit_model_setting.models', []);
                }}
              >
                <ModelLimitSelector
                  models={models}
                  value={values.setting.limits.limit_model_setting.models}
                  onChange={(ids) => setField('setting.limits.limit_model_setting.models', ids)}
                  getGroupLabel={getGroupLabel}
                />
              </FeatureToggle>

              <FeatureToggle
                label={t('token_index.limits_ip_whitelist_switch')}
                description={t('token_index.limits_ip_whitelist_info')}
                checked={values.setting?.limits?.limits_ip_setting?.enabled === true}
                onCheckedChange={(c) => {
                  setField('setting.limits.limits_ip_setting.enabled', c);
                  if (!c) setField('setting.limits.limits_ip_setting.whitelist', []);
                }}
              >
                <Textarea
                  rows={5}
                  value={values.setting.limits.limits_ip_setting.whitelist?.join('\n') || ''}
                  onChange={(e) => setField('setting.limits.limits_ip_setting.whitelist', e.target.value.split('\n'))}
                  placeholder={'192.168.1.1\n10.0.0.0/8'}
                />
                <p className="mt-2 text-xs text-muted-foreground">{t('token_index.limits_ip_whitelist_helper')}</p>
              </FeatureToggle>
            </Section>

            <Section title={t('token_index.sectionAdvanced')}>
              <FeatureToggle
                label={t('token_index.heartbeat')}
                description={t('token_index.heartbeatTip')}
                checked={values.setting?.heartbeat?.enabled === true}
                onCheckedChange={(c) => setField('setting.heartbeat.enabled', c)}
              >
                <FormField
                  id="token-heartbeat-timeout"
                  label={t('token_index.heartbeatTimeout')}
                  hint={t('token_index.heartbeatTimeoutHelperText')}
                >
                  <Input
                    id="token-heartbeat-timeout"
                    type="number"
                    value={values.setting.heartbeat.timeout_seconds}
                    onChange={(e) => setField('setting.heartbeat.timeout_seconds', e.target.value)}
                  />
                </FormField>
              </FeatureToggle>
            </Section>
          </div>
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('token_index.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('token_index.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
