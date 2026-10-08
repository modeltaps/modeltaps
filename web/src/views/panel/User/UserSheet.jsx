import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Badge } from '@/components/ui/badge';
import { API } from 'utils/api';
import { showError, showSuccess, trims, renderQuota, renderSpend, renderNumber, timestamp2string, useIsRoot } from 'utils/common';

const ORIGIN = { is_edit: false, username: '', display_name: '', password: '', group: 'default' };

function DetailSection({ title, children }) {
  return (
    <section className="mb-5">
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
      {children}
    </section>
  );
}

function Row({ label, value }) {
  return (
    <div className="flex items-start justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium text-foreground">{value}</span>
    </div>
  );
}

// ==============================|| USER — CREATE / EDIT SHEET ||============================== //
// Reuses the v1 endpoints verbatim: POST /api/user/ (create), PUT /api/user/ (edit,
// full-object submit), GET /api/user/:id, GET /api/group/.
export default function UserSheet({ open, userId, onClose, onSaved }) {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState(ORIGIN);
  // 加载时落库的邮箱及其「已验证」状态:角标讲的是库里那个地址的来源,只在输入框还没被改动时展示。
  const [storedEmail, setStoredEmail] = useState(null);
  const [groups, setGroups] = useState([]);
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState({});
  const isEdit = Boolean(userId);
  const isRoot = useIsRoot();
  // 目标用户已是超级管理员时角色只读:与列表页「设为…」对 role 100 的保护一致。
  const roleLocked = inputs.role === 100;

  const set = (k, v) => setInputs((p) => ({ ...p, [k]: v }));

  // 「已验证」按后端记下的真实来源显示(users.email_verified):管理员在这里直接改的邮箱
  // 不发验证码,存下去就是未验证。输入框被改动后先不表态,等保存后重新加载再说。
  const emailPristine = storedEmail && storedEmail.email && inputs.email === storedEmail.email;
  const emailLabel = (
    <span className="inline-flex items-center gap-2">
      {t('userPage.email')}
      {emailPristine && (
        <Badge variant={storedEmail.verified ? 'secondary' : 'outline'}>
          {t(storedEmail.verified ? 'settingsPage.security.verified' : 'settingsPage.security.notVerified')}
        </Badge>
      )}
    </span>
  );

  useEffect(() => {
    if (!open) return;
    setErrors({});
    // 换一个用户打开时先清角标,免得上一个用户的状态在新数据到达前闪一下。
    setStoredEmail(null);
    (async () => {
      try {
        const res = await API.get('/api/group/');
        const list = Array.isArray(res.data.data)
          ? res.data.data.map((g) => (typeof g === 'string' ? g : g.symbol))
          : [];
        setGroups(list.sort((a, b) => String(a).localeCompare(String(b))));
      } catch (e) {
        showError(e.message);
      }
    })();
    if (userId) {
      (async () => {
        try {
          const res = await API.get(`/api/user/${userId}`);
          const { success, message, data } = res.data;
          if (success) {
            setInputs({ ...data, is_edit: true, password: '' });
            setStoredEmail({ email: data.email || '', verified: !!data.email_verified });
          } else showError(message);
        } catch (e) {
          // surfaced globally
        }
      })();
    } else {
      setInputs(ORIGIN);
    }
  }, [open, userId]);

  const submit = async () => {
    const values = trims({ ...inputs });
    const errs = {};
    if (!values.username) errs.username = t('userPage.usernameRequired');
    if (!isEdit && !values.password) errs.password = t('userPage.passwordRequired');
    else if (values.password && (values.password.length < 8 || values.password.length > 64))
      errs.password = t('userPage.passwordLength');
    setErrors(errs);
    if (Object.keys(errs).length > 0) return;
    setSubmitting(true);
    try {
      const res = isEdit ? await API.put('/api/user/', { ...values, id: parseInt(userId) }) : await API.post('/api/user/', values);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('userPage.saveSuccess'));
        onSaved?.();
      } else {
        showError(message);
      }
    } catch (e) {
      // surfaced globally
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose?.()}>
      <SheetContent onClose={onClose} className="sm:max-w-2xl">
        <SheetHeader>
          <SheetTitle>{isEdit ? t('userPage.editUser') : t('userPage.createUser')}</SheetTitle>
        </SheetHeader>
        <SheetBody>
          <div className="mb-6 space-y-4">
            <FormField id="u-username" label={t('userPage.username')} required error={errors.username}>
              <Input
                id="u-username"
                value={inputs.username || ''}
                autoComplete="username"
                onChange={(e) => {
                  set('username', e.target.value);
                  if (errors.username) setErrors((x) => ({ ...x, username: '' }));
                }}
              />
            </FormField>
            <FormField id="u-display" label={t('userPage.displayName')}>
              <Input id="u-display" value={inputs.display_name || ''} onChange={(e) => set('display_name', e.target.value)} />
            </FormField>
            {isEdit && (
              <FormField id="u-email" label={emailLabel} hint={t('userPage.emailEditHint')}>
                <Input
                  id="u-email"
                  type="email"
                  value={inputs.email || ''}
                  autoComplete="off"
                  onChange={(e) => set('email', e.target.value)}
                />
              </FormField>
            )}
            <FormField
              id="u-password"
              label={t('userPage.password')}
              required={!isEdit}
              error={errors.password}
              hint={isEdit ? t('userPage.passwordEditHint') : null}
            >
              <Input
                id="u-password"
                type="password"
                value={inputs.password || ''}
                autoComplete="new-password"
                onChange={(e) => {
                  set('password', e.target.value);
                  if (errors.password) setErrors((x) => ({ ...x, password: '' }));
                }}
              />
            </FormField>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField id="u-group" label={t('userPage.group')}>
                <Select value={inputs.group} onValueChange={(v) => set('group', v)}>
                  <SelectTrigger id="u-group">
                    <SelectValue placeholder="default" />
                  </SelectTrigger>
                  <SelectContent>
                    {groups.map((g) => (
                      <SelectItem key={g} value={g}>
                        {g}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
              {isEdit && (
                <FormField id="u-role" label={t('userPage.userRole')}>
                  <Select value={String(inputs.role ?? 1)} onValueChange={(v) => set('role', Number(v))} disabled={roleLocked}>
                    <SelectTrigger id="u-role">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1">{t('userPage.cUserRole')}</SelectItem>
                      <SelectItem value="3">{t('userPage.reliableUserRole')}</SelectItem>
                      <SelectItem value="10">{t('userPage.adminUserRole')}</SelectItem>
                      {/* role 100 只有 root 通过 PUT /api/user/ 能写入,非 root 不显示该项。 */}
                      {(isRoot || roleLocked) && <SelectItem value="100">{t('userPage.superAdminRole')}</SelectItem>}
                    </SelectContent>
                  </Select>
                </FormField>
              )}
            </div>
          </div>

          {isEdit && (
            <>
              <DetailSection title={t('userPage.statistics')}>
                <Row label={t('token_index.remainingQuota')} value={renderQuota(inputs.quota ?? 0)} />
                <Row label={t('token_index.usedQuota')} value={renderSpend(inputs.used_quota ?? 0)} />
                <Row label={t('userPage.useQuota')} value={renderNumber(inputs.request_count ?? 0)} />
              </DetailSection>

              <DetailSection title={t('profilePage.accountBinding')}>
                {[
                  ['WeChat', inputs.wechat_id],
                  ['GitHub', inputs.github_id],
                  ['LINUX DO', inputs.linuxdo_username]
                ].map(([label, val]) => (
                  <Row key={label} label={label} value={val || t('profilePage.notBound')} />
                ))}
                {/* 手机号只读:IdP 下发的已验证 phone_number claim，不提供编辑入口。 */}
                <Row label={t('userPage.phoneNumber')} value={inputs.phone_number || t('userPage.phoneNumberNotProvided')} />
              </DetailSection>

              <DetailSection title={t('userPage.status')}>
                <Row label={t('userPage.id')} value={inputs.id ?? '-'} />
                <Row
                  label={t('userPage.status')}
                  value={
                    <Badge variant={inputs.status === 1 ? 'default' : 'secondary'}>
                      {inputs.status === 1 ? t('common.enable') : t('common.disable')}
                    </Badge>
                  }
                />
                <Row
                  label={t('userPage.creationTime')}
                  value={inputs.created_time ? timestamp2string(inputs.created_time) : t('common.unknown')}
                />
                <Row
                  label={t('userPage.lastLoginTime')}
                  value={inputs.last_login_time ? timestamp2string(inputs.last_login_time) : t('common.unknown')}
                />
                <Row label={t('userPage.lastLoginIP')} value={inputs.last_login_ip || t('common.unknown')} />
              </DetailSection>
            </>
          )}
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={onClose}>
            {t('userPage.cancel')}
          </Button>
          <Button disabled={submitting} onClick={submit}>
            {t('userPage.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
