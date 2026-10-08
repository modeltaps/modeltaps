import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { Eye, EyeOff, Copy } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { RowActions, RowActionsSurfaceContext } from '@/components/ui/row-actions';
import { cn } from '@/lib/utils';
import { renderQuota, SpendAmount, timestamp2string } from 'utils/common';
import { isPlaygroundToken } from 'utils/playgroundToken';
import { statusInfo, GroupBadge, LimitBar, nextResetTime, buildTokenActions } from './TokenTable';

// 详情行:左侧 muted 标签,右侧右对齐值。
function Field({ label, children }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-border py-2.5 last:border-b-0">
      <span className="shrink-0 text-sm text-muted-foreground">{label}</span>
      <div className="min-w-0 flex-1 text-right text-sm">{children}</div>
    </div>
  );
}

Field.propTypes = { label: PropTypes.node, children: PropTypes.node };

// TC3:移动端点卡片弹出的 API Key 详情底部 Sheet。字段与表格数据同源;动作复用 buildTokenActions,
// 触发后先关闭本 Sheet 再执行(避免与编辑 Sheet 等叠加)。
export default function TokenDetailSheet({
  open,
  onOpenChange,
  token,
  userGroup,
  userIsReliable,
  orgMemberNames,
  todayUsage,
  quotaResetTimezone,
  quotaResetWeekStart,
  onToggleStatus,
  onCopyKey,
  onEdit,
  onDelete,
  onShowUsage,
  onConnectOpenClaw
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [shown, setShown] = useState(false);
  useEffect(() => {
    if (!open) setShown(false);
  }, [open, token?.id]);

  if (!token) return null;

  const info = statusInfo(t, token.status);
  const isAbnormal = token.status === 3 || token.status === 4;
  const k = token.key || '';
  const masked = k.length > 6 ? `sk-${k.slice(0, 3)}…${k.slice(-3)}` : `sk-${k}`;
  const nowSec = Math.floor(Date.now() / 1000);
  const expired = token.expired_time !== -1 && token.expired_time <= nowSec;

  // 限额三形态(与表格一致):周期 / 未限额 / 总额。
  const qr = token.setting?.quota_reset;
  let limitNode;
  if (qr?.period && qr.limit > 0) {
    const used = token.period_used || 0;
    const next = nextResetTime(qr.period, quotaResetTimezone, quotaResetWeekStart);
    limitNode = (
      <div className="flex flex-col items-end gap-1">
        <div className="flex items-center gap-1.5">
          <span className="font-mono tabular-nums">{renderQuota(qr.limit)}</span>
          <Badge variant="secondary" className="text-[11px]">
            {t(`token_index.limitPeriod_${qr.period}`)}
          </Badge>
        </div>
        <div className="w-full">
          <LimitBar ratio={used / qr.limit} />
        </div>
        {next && (
          <span className="text-xs text-muted-foreground">
            {t('token_index.quotaResetNext')}: {timestamp2string(next)}
          </span>
        )}
      </div>
    );
  } else if (token.unlimited_quota) {
    limitNode = <span className="text-muted-foreground">{t('token_index.noLimit')}</span>;
  } else {
    const used = token.used_quota || 0;
    const total = used + (token.remain_quota || 0);
    limitNode = (
      <div className="flex flex-col items-end gap-1">
        <div className="flex items-center gap-1.5">
          <span className="font-mono tabular-nums">{renderQuota(total)}</span>
          <Badge variant="outline" className="text-[11px]">
            {t('token_index.limitTotal')}
          </Badge>
        </div>
        <div className="w-full">
          <LimitBar ratio={total > 0 ? used / total : 0} />
        </div>
      </div>
    );
  }

  // 动作触发后先关闭详情 Sheet,再走原回调。
  const closeThen =
    (fn) =>
    (...args) => {
      onOpenChange(false);
      fn?.(...args);
    };
  const actions = buildTokenActions({
    item: token,
    t,
    navigate,
    onEdit,
    onDelete,
    onCopyKey,
    onShowUsage,
    onConnectOpenClaw
  })
    .filter(Boolean)
    .map((a) => ({
      ...a,
      onClick: a.onClick ? closeThen(a.onClick) : undefined,
      items: a.items ? a.items.map((it) => ({ ...it, onClick: closeThen(it.onClick) })) : undefined
    }));

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="bottom" onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{t('token_index.tokenDetail')}</SheetTitle>
        </SheetHeader>
        <SheetBody>
          <Field label={t('token_index.name')}>
            <span className="font-medium">{token.name}</span>
          </Field>
          <Field label={t('token_index.status')}>
            <div className="flex items-center justify-end gap-2">
              {isAbnormal && (
                <Badge variant="destructive" className="text-[11px]">
                  {info.label}
                </Badge>
              )}
              <Switch checked={token.status === 1} onCheckedChange={() => onToggleStatus(token)} aria-label={info.label} />
            </div>
          </Field>
          <Field label={t('token_index.secretKey')}>
            {isPlaygroundToken(token) ? (
              <Badge variant="secondary" className="text-[11px]">
                {t('token_index.playgroundManaged')}
              </Badge>
            ) : (
              <div className="flex items-center justify-end gap-1">
                <code className="min-w-0 truncate font-mono text-[13px] text-muted-foreground">{shown ? `sk-${token.key}` : masked}</code>
                <Button variant="ghost" size="icon" className="size-6" onClick={() => setShown((s) => !s)} aria-label="reveal">
                  {shown ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
                </Button>
                <Button variant="ghost" size="icon" className="size-6" onClick={() => onCopyKey(token)} aria-label="copy">
                  <Copy className="size-3.5" />
                </Button>
              </div>
            )}
          </Field>
          <Field label={t('token_index.userGroup')}>
            <GroupBadge item={token} userGroup={userGroup} />
          </Field>
          {orgMemberNames && (
            <Field label={t('org.creator')}>
              {token.created_by ? <Badge variant="outline">{orgMemberNames[token.created_by] || `#${token.created_by}`}</Badge> : '-'}
            </Field>
          )}
          {userIsReliable && <Field label={t('token_index.billingTag')}>{userGroup[token.setting?.billing_tag]?.name || '-'}</Field>}
          <Field label={t('token_index.expiryTime')}>
            {token.expired_time === -1 ? (
              <span className="text-muted-foreground">{t('token_index.expiryNever')}</span>
            ) : (
              <span className={cn(expired && 'text-destructive')}>{timestamp2string(token.expired_time)}</span>
            )}
          </Field>
          <Field label={t('token_index.createdTime')}>{timestamp2string(token.created_time)}</Field>
          <Field label={t('token_index.lastUsed')}>
            {token.accessed_time > 0 ? (
              timestamp2string(token.accessed_time)
            ) : (
              <span className="text-muted-foreground">{t('token_index.neverUsed')}</span>
            )}
          </Field>
          <Field label={t('token_index.usageColumn')}>
            <div className="flex flex-col items-end gap-0.5">
              <SpendAmount quota={token.used_quota} className="font-mono tabular-nums" />
              {todayUsage && (
                <span className="text-xs text-muted-foreground">
                  {t('token_index.usageToday')} <SpendAmount quota={todayUsage[token.name] || 0} />
                </span>
              )}
            </div>
          </Field>
          <Field label={t('token_index.limitColumn')}>{limitNode}</Field>
        </SheetBody>
        <SheetFooter>
          <RowActionsSurfaceContext.Provider value="card">
            <div className="flex flex-1 flex-wrap items-center justify-end gap-1.5">
              <RowActions actions={actions} />
            </div>
          </RowActionsSurfaceContext.Provider>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

TokenDetailSheet.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  token: PropTypes.object,
  userGroup: PropTypes.object,
  userIsReliable: PropTypes.bool,
  orgMemberNames: PropTypes.object,
  todayUsage: PropTypes.object,
  quotaResetTimezone: PropTypes.string,
  quotaResetWeekStart: PropTypes.string,
  onToggleStatus: PropTypes.func,
  onCopyKey: PropTypes.func,
  onEdit: PropTypes.func,
  onDelete: PropTypes.func,
  onShowUsage: PropTypes.func,
  onConnectOpenClaw: PropTypes.func
};
