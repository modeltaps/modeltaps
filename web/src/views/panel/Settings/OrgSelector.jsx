import PropTypes from 'prop-types';
import { useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Building2, Check, ChevronsUpDown } from 'lucide-react';

import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel } from '@/components/ui/dropdown-menu';
import { orgSettingsUrl } from './sections';

// ==============================|| SETTINGS — ORG SELECTOR ||============================== //
// 只列出我是 Owner / Admin 的组织。切换仅改写 URL 中的 orgId(保持当前 section),
// 不调用 switchOrg —— 设置页的「管理哪个组织」与左上角的全局上下文解耦。

export default function OrgSelector({ orgs, activeOrgId, section }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const activeOrg = orgs.find((o) => o.id === activeOrgId) || null;

  return (
    <div className="[&>div]:w-full">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm font-medium text-foreground/80 transition-colors hover:bg-muted hover:text-foreground"
            aria-label={t('settingsPage.orgSelector.label', { defaultValue: 'Select organization' })}
          >
            <Building2 className="size-[1.125rem] shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate">
              {activeOrg ? activeOrg.name : t('settingsPage.orgSelector.label', { defaultValue: 'Select organization' })}
            </span>
            <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />
          </button>
        </DropdownMenuTrigger>
        {/* w-0 + matchTriggerWidth(内联 min-width = 触发器宽度)让面板宽度恰好等于触发按钮,长名称走 truncate */}
        <DropdownMenuContent align="start" matchTriggerWidth className="w-0 min-w-0">
          <DropdownMenuLabel>{t('settingsPage.orgSelector.label', { defaultValue: 'Select organization' })}</DropdownMenuLabel>
          {orgs.map((org) => (
            <DropdownMenuItem key={org.id} onClick={() => navigate(orgSettingsUrl(org.id, section))}>
              <span className="flex-1 truncate">{org.name}</span>
              <span className="shrink-0 text-[0.65rem] text-muted-foreground">
                {t(`org.roles.${org.role}`, { defaultValue: org.role })}
              </span>
              {org.id === activeOrgId && <Check className="size-4 shrink-0" />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

OrgSelector.propTypes = {
  orgs: PropTypes.array.isRequired,
  activeOrgId: PropTypes.number,
  section: PropTypes.string.isRequired
};
