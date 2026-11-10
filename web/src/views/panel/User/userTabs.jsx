import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Wallet } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Field, InfoRow, InfoRows, Section, StatRow } from '@/components/objects';
import { renderNumber, renderQuota, renderSpend, timestamp2string } from 'utils/common';

// Tabs of the user panel (see components/objects/ObjectPanel). Overview is read-only; Profile,
// Quota & groups and Security each save on their own. The update endpoint takes the whole user,
// so a save sends the user as just reloaded with the tab's fields replaced.

export const ROLE_LABEL = {
  1: 'userPage.cUserRole',
  3: 'userPage.reliableUserRole',
  10: 'userPage.adminUserRole',
  100: 'userPage.superAdminRole'
};

const when = (t, ts) => (ts ? timestamp2string(ts) : t('common.unknown'));

function OverviewBody({ object: user }) {
  const { t } = useTranslation();
  return (
    <>
      <StatRow
        items={[
          { label: t('token_index.remainingQuota'), value: renderQuota(user.quota ?? 0) },
          { label: t('token_index.usedQuota'), value: renderSpend(user.used_quota ?? 0) },
          { label: t('userPage.useQuota'), value: renderNumber(user.request_count ?? 0) }
        ]}
      />
      <Section title={t('userPage.panel.account')}>
        <InfoRows>
          <InfoRow label={t('userPage.id')} mono>
            {user.id}
          </InfoRow>
          <InfoRow label={t('userPage.status')}>{user.status === 1 ? t('userPage.statusEnabled') : t('userPage.statusDisabled')}</InfoRow>
          <InfoRow label={t('userPage.userRole')}>{t(ROLE_LABEL[user.role] || 'userPage.uUserRole')}</InfoRow>
          <InfoRow label={t('userPage.group')} mono>
            {user.group || '-'}
          </InfoRow>
          {user.org_name && <InfoRow label={t('userPage.org')}>{user.org_name}</InfoRow>}
          <InfoRow label={t('userPage.creationTime')}>{when(t, user.created_time)}</InfoRow>
          <InfoRow label={t('userPage.lastLoginTime')}>{when(t, user.last_login_time)}</InfoRow>
          <InfoRow label={t('userPage.lastLoginIP')} mono>
            {user.last_login_ip || t('common.unknown')}
          </InfoRow>
        </InfoRows>
      </Section>
      <Section title={t('profilePage.accountBinding')}>
        <InfoRows>
          {[
            ['WeChat', user.wechat_id],
            ['GitHub', user.github_id],
            ['LINUX DO', user.linuxdo_username]
          ].map(([label, val]) => (
            <InfoRow key={label} label={label}>
              {val || <span className="text-muted-foreground">{t('profilePage.notBound')}</span>}
            </InfoRow>
          ))}
          {/* 手机号只读:IdP 下发的已验证 phone_number claim，不提供编辑入口。 */}
          <InfoRow label={t('userPage.phoneNumber')}>
            {user.phone_number || <span className="text-muted-foreground">{t('userPage.phoneNumberNotProvided')}</span>}
          </InfoRow>
        </InfoRows>
      </Section>
    </>
  );
}

const overviewTab = { key: 'overview', Body: OverviewBody };

function ProfileBody({ object: user, draft, setDraft }) {
  const { t } = useTranslation();
  const set = (key) => (e) => setDraft((d) => ({ ...d, [key]: e.target.value }));
  // 「已验证」讲的是库里那个地址的来源(users.email_verified),只在输入框还没被改动时展示;管理员在这里
  // 直接改的邮箱不发验证码,存下去就是未验证。
  const pristine = user.email && draft.email === user.email;
  return (
    <>
      <Field label={t('userPage.username')} htmlFor="u-username">
        <Input id="u-username" autoComplete="off" value={draft.username} onChange={set('username')} />
      </Field>
      <Field label={t('userPage.displayName')} htmlFor="u-display" optional>
        <Input id="u-display" value={draft.display_name} onChange={set('display_name')} />
      </Field>
      <Field
        label={t('userPage.email')}
        htmlFor="u-email"
        hint={t('userPage.emailEditHint')}
        extra={
          pristine ? (
            <Badge variant={user.email_verified ? 'secondary' : 'outline'} className="font-normal">
              {t(user.email_verified ? 'settingsPage.security.verified' : 'settingsPage.security.notVerified')}
            </Badge>
          ) : null
        }
      >
        <Input id="u-email" type="email" autoComplete="off" value={draft.email} onChange={set('email')} />
      </Field>
    </>
  );
}

const profileTab = {
  key: 'profile',
  make: (u) => ({ username: u.username || '', display_name: u.display_name || '', email: u.email || '' }),
  patch: (d, u, { t }) => {
    const username = d.username.trim();
    if (!username) return { error: t('userPage.usernameRequired') };
    return { username, display_name: d.display_name.trim(), email: d.email.trim() };
  },
  Body: ProfileBody
};

function AccessBody({ object: user, draft, setDraft, ctx }) {
  const { t } = useTranslation();
  // 目标用户已是超级管理员时角色只读:与列表页「设为…」对 role 100 的保护一致。
  const roleLocked = user.role === 100;
  const groups = ctx.groups.includes(draft.group) || !draft.group ? ctx.groups : [...ctx.groups, draft.group];
  return (
    <>
      <Field
        label={t('userPage.balance')}
        extra={
          <Button type="button" variant="ghost" size="sm" className="h-7 rounded-lg px-2 text-xs font-normal" onClick={ctx.onAdjustQuota}>
            <Wallet className="size-3.5" />
            {t('userPage.changeQuota')}
          </Button>
        }
        hint={t('userPage.panel.quotaHint')}
      >
        <p className="text-lg font-medium tabular-nums">{renderQuota(user.quota ?? 0)}</p>
      </Field>
      <Field label={t('userPage.group')} htmlFor="u-group" hint={t('userPage.panel.groupHint')}>
        <Select value={draft.group} onValueChange={(v) => setDraft((d) => ({ ...d, group: v }))}>
          <SelectTrigger id="u-group" className="w-64">
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
      </Field>
      <Field label={t('userPage.userRole')} htmlFor="u-role" hint={roleLocked ? t('userPage.panel.roleLocked') : null}>
        <Select value={String(draft.role)} onValueChange={(v) => setDraft((d) => ({ ...d, role: Number(v) }))} disabled={roleLocked}>
          <SelectTrigger id="u-role" className="w-64">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="1">{t('userPage.cUserRole')}</SelectItem>
            <SelectItem value="3">{t('userPage.reliableUserRole')}</SelectItem>
            <SelectItem value="10">{t('userPage.adminUserRole')}</SelectItem>
            {/* role 100 只有 root 通过 PUT /api/user/ 能写入,非 root 不显示该项。 */}
            {(ctx.isRoot || roleLocked) && <SelectItem value="100">{t('userPage.superAdminRole')}</SelectItem>}
          </SelectContent>
        </Select>
      </Field>
    </>
  );
}

const accessTab = {
  key: 'access',
  make: (u) => ({ group: u.group || 'default', role: u.role ?? 1 }),
  patch: (d) => ({ group: d.group, role: d.role }),
  Body: AccessBody
};

function SecurityBody({ draft, setDraft }) {
  const { t } = useTranslation();
  return (
    <Field label={t('userPage.panel.newPassword')} htmlFor="u-password" hint={t('userPage.panel.passwordHint')}>
      <Input
        id="u-password"
        type="password"
        autoComplete="new-password"
        value={draft.password}
        onChange={(e) => setDraft((d) => ({ ...d, password: e.target.value }))}
      />
    </Field>
  );
}

const securityTab = {
  key: 'security',
  make: () => ({ password: '' }),
  patch: (d, u, { t }) => {
    const password = d.password.trim();
    if (password.length < 8 || password.length > 64) return { error: t('userPage.passwordLength') };
    return { password };
  },
  Body: SecurityBody
};

export const USER_TABS = [overviewTab, profileTab, accessTab, securityTab];
export const USER_TAB_KEYS = USER_TABS.map((x) => x.key);

const bodyProps = { object: PropTypes.object, draft: PropTypes.object, setDraft: PropTypes.func, ctx: PropTypes.object };
OverviewBody.propTypes = bodyProps;
ProfileBody.propTypes = bodyProps;
AccessBody.propTypes = bodyProps;
SecurityBody.propTypes = bodyProps;
