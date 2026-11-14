import { useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowRightLeft, Building2, LogOut } from 'lucide-react';

import { cn } from '@/lib/utils';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { API } from 'utils/api';
import { renderBalance, renderSpend, showError, showSuccess, timestamp2string } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import { nextResetTime } from '../Token/TokenTable';
import TransferQuotaDialog from './TransferQuotaDialog';

// ==============================|| ORGANIZATION — OVERVIEW TAB ||============================== //
// Org profile header + stat cards. Balance card (shadow-account quota) is only
// returned by the API for Owner/Admin; members see a "hidden" placeholder.
// S5:成员在此退出组织(Owner 须先转让所有权,转让/解散仍在统一设置页的危险区)。
// 回落规则同 S4:仅当退出的是当前全局上下文所在组织时才切回个人。

// A stat is a label, a figure and at most one line of context; an action sits at the end of the
// label row (icon only on phones, where two cards share a row).
function StatCard({ label, value, hint, action, className }) {
  return (
    <Card className={cn('min-w-0 space-y-1 p-4 sm:p-5', className)}>
      <div className="flex h-7 items-center justify-between gap-2">
        <p className="truncate text-sm text-muted-foreground">{label}</p>
        {action}
      </div>
      <p className="truncate text-xl font-medium tabular-nums sm:text-2xl">{value}</p>
      {hint && <p className="truncate text-xs text-muted-foreground">{hint}</p>}
    </Card>
  );
}

StatCard.propTypes = {
  label: PropTypes.string.isRequired,
  value: PropTypes.node,
  hint: PropTypes.node,
  action: PropTypes.node,
  className: PropTypes.string
};

export default function OverviewTab({ orgId, detail, isAdmin, role, onRefresh }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const { refreshOrganizations, switchOrg, currentOrgId } = useOrg();
  const [transferOpen, setTransferOpen] = useState(false);
  const [leaveOpen, setLeaveOpen] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const isOwner = role === 'owner';

  const leave = async () => {
    setLeaving(true);
    try {
      const res = await API.post(`/api/org/${orgId}/leave`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.settings.leaveSuccess'));
        if (currentOrgId === orgId) switchOrg(null);
        refreshOrganizations();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLeaving(false);
      setLeaveOpen(false);
    }
  };

  if (!detail) {
    return (
      <div className="space-y-6" aria-busy="true">
        <Skeleton className="h-[88px] rounded-xl" />
        <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-3">
          <Skeleton className="h-[100px] rounded-xl" />
          <Skeleton className="h-[100px] rounded-xl" />
          <Skeleton className="col-span-2 h-[100px] rounded-xl lg:col-span-1" />
        </div>
      </div>
    );
  }

  const org = detail.organization || {};

  // 组织周期预算(T1):budget/budget_used 仅 Owner/Admin 可见,未配置视为不限
  const budget = detail.budget;
  const hasBudget = budget?.period && budget.limit > 0;
  const budgetNext = hasBudget ? nextResetTime(budget.period, siteInfo?.quota_reset_timezone, siteInfo?.quota_reset_week_start) : null;

  return (
    <div className="space-y-6">
      <Card className="flex items-center gap-3 p-4 sm:gap-4 sm:p-6">
        {org.avatar_url ? (
          <img src={org.avatar_url} alt={org.name} className="size-10 shrink-0 rounded-md object-cover sm:size-12" />
        ) : (
          <span className="flex size-10 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground sm:size-12">
            <Building2 className="size-5 sm:size-6" />
          </span>
        )}
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-baseline gap-2">
            <h2 className="truncate text-lg font-semibold">{org.name}</h2>
            <span className="shrink-0 whitespace-nowrap text-xs text-muted-foreground">{t(`org.roles.${detail.role}`)}</span>
          </div>
          {/* Slug and creation date are reference data: one quiet line under the name; the slug gives
              way first so the date stays whole. */}
          <p className="flex min-w-0 text-sm text-muted-foreground">
            <span className="truncate">{org.slug}</span>
            {org.created_time > 0 && (
              <span className="shrink-0 whitespace-nowrap">
                {org.slug && <span className="px-1">·</span>}
                {t('orgList.createdAt', { time: timestamp2string(org.created_time).slice(0, 10) })}
              </span>
            )}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          className="shrink-0"
          disabled={isOwner}
          aria-label={t('orgPage.settings.leave')}
          title={isOwner ? t('orgPage.settings.leaveOwnerHint') : undefined}
          onClick={() => setLeaveOpen(true)}
        >
          <LogOut className="size-4" />
          <span className="hidden sm:inline">{t('orgPage.settings.leave')}</span>
        </Button>
      </Card>

      <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-3">
        <StatCard
          label={t('orgPage.overview.balance')}
          value={isAdmin && detail.quota != null ? renderBalance(detail.quota) : t('org.balanceHidden')}
          action={
            isAdmin && (
              <Button
                size="sm"
                variant="ghost"
                className="-mr-2 h-7 gap-1.5 px-2 text-muted-foreground hover:text-foreground"
                aria-label={t('orgPage.overview.transferQuota')}
                title={t('orgPage.overview.transferQuota')}
                onClick={() => setTransferOpen(true)}
              >
                <ArrowRightLeft className="size-4" />
                <span className="hidden sm:inline">{t('orgPage.overview.transferQuota')}</span>
              </Button>
            )
          }
        />
        <StatCard label={t('orgPage.overview.memberCount')} value={detail.member_count} />
        {isAdmin && (
          <StatCard
            className="col-span-2 lg:col-span-1"
            label={t('orgPage.overview.teamBudget')}
            value={hasBudget ? `${renderSpend(detail.budget_used || 0)} / ${renderSpend(budget.limit)}` : t('org.budgetUnlimited')}
            hint={
              hasBudget
                ? [
                    t(`orgPage.limits.period_${budget.period}`, { defaultValue: budget.period }),
                    budgetNext && `${t('token_index.quotaResetNext')}: ${timestamp2string(budgetNext)}`
                  ]
                    .filter(Boolean)
                    .join(' · ')
                : null
            }
          />
        )}
      </div>

      <TransferQuotaDialog open={transferOpen} onOpenChange={setTransferOpen} orgId={orgId} onTransferred={onRefresh} />

      <Dialog open={leaveOpen} onOpenChange={setLeaveOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('orgPage.settings.leave')}</DialogTitle>
            <DialogDescription>{t('orgPage.settings.leaveConfirm')}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setLeaveOpen(false)} disabled={leaving}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={leave} disabled={leaving}>
              {t('orgPage.settings.leave')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

OverviewTab.propTypes = {
  orgId: PropTypes.number.isRequired,
  detail: PropTypes.object,
  isAdmin: PropTypes.bool,
  role: PropTypes.string,
  onRefresh: PropTypes.func
};
