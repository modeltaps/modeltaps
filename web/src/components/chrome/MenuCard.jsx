import { useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Building2, Check, ChevronsUpDown, Plus } from 'lucide-react';

import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel
} from '@/components/ui/dropdown-menu';
import { useOrg } from 'contexts/OrgContext';
import CreateOrgDialog from '../org/CreateOrgDialog';
import UserAvatar, { accountDisplayName } from './avatar';

// ==============================|| CHROME — ACCOUNT / CONTEXT SWITCHER ||============================== //
// 侧栏顶部的账号行(照 x.ai 控制台的 team switcher):个人上下文显示账号——有名字显示名字,
// 否则显示邮箱或手机号,小字标「个人」;组织上下文显示组织名,小字「账号 · 角色」。
// 点开是纯切换器:个人(带邮箱)、各组织(带角色)、创建组织。退出登录在页脚的「更多」里。
// 组织功能关闭时只剩静态账号行;折叠为窄栏时退化为头像 / 组织头像按钮。

function OrgAvatar({ org, className }) {
  if (org?.avatar_url) {
    return <img src={org.avatar_url} alt={org.name} className={cn('rounded-md object-cover', className)} />;
  }
  return (
    <span className={cn('flex items-center justify-center rounded-md border border-border bg-muted text-muted-foreground', className)}>
      <Building2 className="size-[60%]" />
    </span>
  );
}

OrgAvatar.propTypes = {
  org: PropTypes.object,
  className: PropTypes.string
};

export default function MenuCard({ collapsed = false }) {
  const { t } = useTranslation();
  const user = useSelector((state) => state.account.user);
  const { orgEnabled, organizations, currentOrg, orgRole, switchOrg } = useOrg();
  const [createOpen, setCreateOpen] = useState(false);

  const accountName = accountDisplayName(user);
  const roleLabel = (role) => t(`org.roles.${role}`, { defaultValue: role });
  const primary = currentOrg ? currentOrg.name : accountName;
  const secondary = currentOrg ? `${accountName} · ${roleLabel(orgRole)}` : t('org.personal');
  const avatar = currentOrg ? (
    <OrgAvatar org={currentOrg} className="size-7 shrink-0" />
  ) : (
    <UserAvatar user={user} className="size-7" textClass="text-[11px]" />
  );

  // 上下文切换下拉:orgEnabled 时始终可用(0 个组织也渲染,保证可从 UI 创建第一个)
  const switchMenu = (
    <DropdownMenuContent align="start" className="w-60">
      <DropdownMenuItem onClick={() => switchOrg(null)}>
        <UserAvatar user={user} className="size-6" textClass="text-[10px]" />
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate leading-tight">{accountName}</span>
          <span className="truncate text-[11px] leading-tight text-muted-foreground">
            {t('org.personal')}
            {user?.email && user.email !== accountName ? ` · ${user.email}` : ''}
          </span>
        </span>
        {!currentOrg && <Check className="size-4 shrink-0" />}
      </DropdownMenuItem>
      {organizations.length > 0 && (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuLabel>{t('org.organizations')}</DropdownMenuLabel>
          {organizations.map((org) => (
            <DropdownMenuItem key={org.id} onClick={() => switchOrg(org.id)}>
              <OrgAvatar org={org} className="size-6 shrink-0" />
              <span className="flex-1 truncate">{org.name}</span>
              <span className="shrink-0 text-[0.65rem] text-muted-foreground">{roleLabel(org.role)}</span>
              {currentOrg?.id === org.id && <Check className="size-4 shrink-0" />}
            </DropdownMenuItem>
          ))}
        </>
      )}
      <DropdownMenuSeparator />
      <DropdownMenuItem onClick={() => setCreateOpen(true)}>
        <span className="flex size-6 shrink-0 items-center justify-center rounded-md border border-dashed border-border">
          <Plus className="size-3.5" />
        </span>
        <span className="flex-1 truncate">{t('org.createOrg')}</span>
      </DropdownMenuItem>
    </DropdownMenuContent>
  );

  if (collapsed) {
    const trigger = (
      <button
        type="button"
        aria-label={orgEnabled ? t('sidebar.switchAccount', { defaultValue: 'Switch account' }) : primary}
        title={primary}
        className="flex size-9 items-center justify-center rounded-md border border-border text-foreground transition-colors hover:bg-muted"
      >
        {currentOrg ? (
          <OrgAvatar org={currentOrg} className="size-6" />
        ) : (
          <UserAvatar user={user} className="size-6" textClass="text-[10px]" />
        )}
      </button>
    );
    return (
      <div className="flex justify-center px-2 pt-2">
        {orgEnabled ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
            {switchMenu}
          </DropdownMenu>
        ) : (
          trigger
        )}
        <CreateOrgDialog open={createOpen} onClose={() => setCreateOpen(false)} />
      </div>
    );
  }

  const identity = (
    <>
      {avatar}
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm font-semibold leading-tight text-foreground">{primary}</span>
        <span className="truncate text-[11px] leading-tight text-muted-foreground">{secondary}</span>
      </span>
    </>
  );

  return (
    <div className="px-2 pt-2">
      {/* [&>div]:w-full:DropdownMenu 的外层 div 是 inline-block(收缩适应内容),需撑满宽度,身份行的 flex-1 才能把切换箭头推到行尾 */}
      <div className="[&>div]:w-full">
        {orgEnabled ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                aria-label={t('sidebar.switchAccount', { defaultValue: 'Switch account' })}
                className="flex w-full items-center gap-2 rounded-lg border border-border bg-muted/40 px-2 py-1.5 text-left transition-colors hover:bg-muted"
              >
                {identity}
                <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />
              </button>
            </DropdownMenuTrigger>
            {switchMenu}
          </DropdownMenu>
        ) : (
          <div className="flex w-full items-center gap-2 rounded-lg border border-border bg-muted/40 px-2 py-1.5">{identity}</div>
        )}
      </div>
      <CreateOrgDialog open={createOpen} onClose={() => setCreateOpen(false)} />
    </div>
  );
}

MenuCard.propTypes = {
  collapsed: PropTypes.bool
};
