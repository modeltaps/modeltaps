import { useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowRightLeft, Building2, CalendarClock, Gauge, Loader2, LogOut, Users, Wallet } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
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

function StatCard({ icon: Icon, label, value, hint, action }) {
  return (
    <Card className="flex items-center justify-between gap-3 p-5">
      <div className="min-w-0 space-y-1">
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className="truncate text-2xl font-semibold tabular-nums">{value}</p>
        {hint && <p className="truncate text-xs text-muted-foreground">{hint}</p>}
      </div>
      <div className="flex shrink-0 flex-col items-end gap-2">
        <span className="flex size-10 items-center justify-center rounded-md bg-muted text-muted-foreground">
          <Icon className="size-5" />
        </span>
        {action}
      </div>
    </Card>
  );
}

StatCard.propTypes = {
  icon: PropTypes.elementType.isRequired,
  label: PropTypes.string.isRequired,
  value: PropTypes.node,
  hint: PropTypes.node,
  action: PropTypes.node
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
      <div className="flex justify-center p-12">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
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
      <Card className="flex items-center gap-4 p-6">
        {org.avatar_url ? (
          <img src={org.avatar_url} alt={org.name} className="size-12 rounded-md object-cover" />
        ) : (
          <span className="flex size-12 items-center justify-center rounded-md bg-muted text-muted-foreground">
            <Building2 className="size-6" />
          </span>
        )}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="truncate text-lg font-semibold">{org.name}</h2>
            <Badge variant={detail.role === 'owner' ? 'default' : 'secondary'}>{t(`org.roles.${detail.role}`)}</Badge>
          </div>
          <p className="truncate text-sm text-muted-foreground">{org.slug}</p>
        </div>
        <Button
          variant="outline"
          size="sm"
          className="shrink-0"
          disabled={isOwner}
          title={isOwner ? t('orgPage.settings.leaveOwnerHint') : undefined}
          onClick={() => setLeaveOpen(true)}
        >
          <LogOut className="size-4" /> {t('orgPage.settings.leave')}
        </Button>
      </Card>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatCard
          icon={Wallet}
          label={t('orgPage.overview.balance')}
          value={isAdmin && detail.quota != null ? renderBalance(detail.quota) : t('org.balanceHidden')}
          action={
            isAdmin && (
              <Button size="sm" variant="outline" onClick={() => setTransferOpen(true)}>
                <ArrowRightLeft className="size-4" /> {t('orgPage.overview.transferQuota')}
              </Button>
            )
          }
        />
        <StatCard icon={Users} label={t('orgPage.overview.memberCount')} value={detail.member_count} />
        {isAdmin && (
          <StatCard
            icon={Gauge}
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
        <StatCard icon={CalendarClock} label={t('orgPage.overview.createdAt')} value={timestamp2string(org.created_time)} />
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
