import { useEffect, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Gift } from 'lucide-react';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { copy, renderQuota, showError } from 'utils/common';

// ==============================|| INVITE CARD ||============================== //
// 账单页与仪表盘共用的邀请奖励卡:图标徽标头部 + 指标行(累计奖励 · 已邀请)+ 邀请链接行。
// 首次点击从 /api/user/aff 取邀请码拼链接并复制,之后点击直接复制。无插图以便与相邻卡片同高。
// 指标优先读 redux 里的 /api/user/self 结果;未加载(如直接落到本卡所在页)时自取一次。
// 卡片默认取内容自然高度;需要与同排卡片等高时由父级传 className="h-full"。

export default function InviteCard({ className }) {
  const { t } = useTranslation();
  const reduxUser = useSelector((state) => state.account.user);
  const [inviteUrl, setInviteUrl] = useState('');
  const [selfStats, setSelfStats] = useState(null);

  const stats = reduxUser && reduxUser.aff_quota !== undefined ? reduxUser : selfStats;

  useEffect(() => {
    if (reduxUser && reduxUser.aff_quota !== undefined) return;
    let cancelled = false;
    (async () => {
      try {
        const res = await API.get('/api/user/self');
        const { success, data } = res.data;
        if (success && !cancelled) setSelfStats(data);
      } catch (error) {
        // surfaced globally by the API interceptor
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [reduxUser]);

  const handleInviteUrl = async () => {
    if (inviteUrl) {
      copy(inviteUrl, t('inviteCard.inviteUrlLabel'));
      return;
    }

    try {
      const res = await API.get('/api/user/aff');
      const { success, message, data } = res.data;
      if (success) {
        const link = `${window.location.origin}/register?aff=${data}`;
        setInviteUrl(link);
        copy(link, t('inviteCard.inviteUrlLabel'));
      } else {
        showError(message);
      }
    } catch (error) {
      // surfaced globally by the API interceptor
    }
  };

  return (
    <Card className={cn('flex flex-col', className)}>
      <CardHeader className="flex-row items-start gap-3 space-y-0 p-4">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Gift className="size-4" />
        </span>
        <div className="flex flex-col gap-1">
          <CardTitle className="text-sm font-semibold">{t('inviteCard.inviteReward')}</CardTitle>
          <p className="text-xs text-muted-foreground">{t('inviteCard.inviteDescription')}</p>
        </div>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col gap-4 p-4 pt-0">
        <div className="grid grid-cols-2 gap-3">
          <div className="flex flex-col gap-1">
            <span className="text-[10px] uppercase tracking-wider text-muted-foreground">{t('inviteCard.stats.totalReward')}</span>
            {stats ? (
              <span className="text-sm font-semibold tabular-nums">{renderQuota(stats.aff_quota || 0)}</span>
            ) : (
              <span className="h-5 w-16 animate-pulse rounded bg-muted" />
            )}
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-[10px] uppercase tracking-wider text-muted-foreground">{t('inviteCard.stats.invited')}</span>
            {stats ? (
              <span className="text-sm font-semibold tabular-nums">
                {t('inviteCard.stats.invitedValue', { value: stats.aff_count || 0 })}
              </span>
            ) : (
              <span className="h-5 w-16 animate-pulse rounded bg-muted" />
            )}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Input
            id="invite-url"
            value={inviteUrl}
            readOnly
            placeholder={t('inviteCard.generateInvite')}
            className="min-w-40 flex-1 font-mono text-xs"
          />
          <Button onClick={handleInviteUrl} className="shrink-0">
            {inviteUrl ? t('inviteCard.copyButton.copy') : t('inviteCard.copyButton.generate')}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
