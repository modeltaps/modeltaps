import { useCallback, useEffect, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Link, Navigate, useSearchParams } from 'react-router';
import { AlertTriangle, Info, RefreshCw, Wallet } from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import PageActions from '@/components/chrome/PageActions';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import { API } from 'utils/api';
import { LOGIN } from 'store/actions';
import { useOrg } from 'contexts/OrgContext';
import InviteCard from '@/components/InviteCard';
import TopupDialog from '../Topup/TopupDialog';
import RedemptionDialog from '../Topup/RedemptionDialog';
import usePaymentMethods from '../Topup/usePaymentMethods';
import Ledger from '../Ledger';
import Invoice from '../Invoice';
import OrdersTab from './OrdersTab';

// ==============================|| PANEL — BILLING ||============================== //
// 布局对齐 OpenRouter Credits:通栏余额横幅 → 充值卡 + 邀请奖励卡(两栏)→ 最近记录 Tab。
// 充值卡主体是主按钮「充值」(有支付渠道时)打开充值弹层,无渠道时改为一行「暂未开放
// 在线充值」说明;卡底两个次级链接:「查看用量」与「使用兑换码」(打开兑换码弹层)。
// 邀请奖励卡受站点开关 invite_reward_enabled 控制(未下发时视为开启),关闭时充值卡通栏。
// Tab 只放数据视图,当前 Tab 由 ?tab=ledger|orders|invoice 单向驱动(刷新/深链自动恢复);
// 页头右上角的刷新是整页动作:重拉余额 + 递增 reloadToken 让当前 Tab 重新拉取
// (列表无筛选/时间范围/导出)。余额还会在页面重新可见、兑换/充值成功后自动重拉。
// 深链 ?topup=1(旧 /panel/topup 与 ?tab=topup 都归到它)自动打开充值弹层,无渠道
// 时打开兑换码弹层,打开后把参数从 URL 抹掉(replace,不污染历史)。
// 组织上下文:Owner/Admin 看组织池横幅 + 同样的入口(资金进池),不显示 Tab 与邀请卡;
// 成员直访重定向回 /panel/log。

const DEFAULT_TAB = 'ledger';

export default function Billing() {
  const { t } = useTranslation();
  const { currentOrgId, orgRole, orgDetail, refreshOrgDetail } = useOrg();
  const invoiceEnabled = useSelector((state) => Boolean(state.siteInfo.UserInvoiceMonth));
  // 开关未下发时按开启处理,避免站点信息落后于前端时邀请卡直接消失
  const inviteEnabled = useSelector((state) => state.siteInfo.invite_reward_enabled !== false);
  const reduxQuota = useSelector((state) => state.account.user?.quota);
  const dispatch = useDispatch();
  const [searchParams, setSearchParams] = useSearchParams();
  const { payment, selectedPayment, setSelectedPayment, loaded: paymentLoaded } = usePaymentMethods();

  const isOrgContext = Boolean(currentOrgId);
  const isOrgAdmin = orgRole === 'owner' || orgRole === 'admin';
  const canOnlineTopup = payment.length > 0;

  const [personalQuota, setPersonalQuota] = useState(reduxQuota || 0);
  const [topupOpen, setTopupOpen] = useState(false);
  const [redemptionOpen, setRedemptionOpen] = useState(false);
  const [reloadToken, setReloadToken] = useState(0);

  // 个人余额自持一份:兑换成功后就地刷新,不依赖整页重载;同时写回 redux,
  // 让侧栏页脚的额度与本页一致。
  // 组织池余额取组织详情(quota 仅 Owner/Admin 返回),刷新走 refreshOrgDetail。
  const reloadPersonalQuota = useCallback(async () => {
    try {
      const res = await API.get('/api/user/self');
      const { success, data } = res.data;
      if (success) {
        setPersonalQuota(data.quota || 0);
        dispatch({ type: LOGIN, payload: data });
      }
    } catch (error) {
      // handled by interceptor
    }
  }, [dispatch]);

  const reloadBalance = useCallback(() => {
    if (isOrgContext) refreshOrgDetail();
    else reloadPersonalQuota();
  }, [isOrgContext, refreshOrgDetail, reloadPersonalQuota]);

  useEffect(() => {
    if (isOrgContext) return;
    reloadPersonalQuota();
  }, [isOrgContext, reloadPersonalQuota]);

  // 页面重新可见时重拉余额(覆盖支付网关新标签页付完款后切回本页的场景)。
  useEffect(() => {
    const onVisibility = () => {
      if (document.visibilityState === 'visible') reloadBalance();
    };
    document.addEventListener('visibilitychange', onVisibility);
    return () => document.removeEventListener('visibilitychange', onVisibility);
  }, [reloadBalance]);

  const quotaPerUnit = Number(localStorage.getItem('quota_per_unit')) || 500000;
  const quota = isOrgContext ? orgDetail?.quota : personalQuota;
  const balance = typeof quota === 'number' ? quota / quotaPerUnit : null;

  const requestedTab = searchParams.get('tab');
  const tab = requestedTab === 'orders' || (requestedTab === 'invoice' && invoiceEnabled) ? requestedTab : DEFAULT_TAB;
  // 旧深链 ?tab=topup 等价于 ?topup=1
  const wantTopup = searchParams.get('topup') === '1' || requestedTab === 'topup';

  const stripParams = useCallback(
    (keys) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          keys.forEach((k) => params.delete(k));
          return params;
        },
        { replace: true }
      );
    },
    [setSearchParams]
  );

  // 深链打开弹层后立刻抹掉参数,避免关掉弹层再刷新又弹回来。
  // 必须等支付渠道列表出结论,否则首帧的空列表会把有渠道的站点也导去兑换码弹层。
  useEffect(() => {
    if (!wantTopup || !paymentLoaded) return;
    if (canOnlineTopup) setTopupOpen(true);
    else setRedemptionOpen(true);
    stripParams(['topup', 'tab']);
  }, [wantTopup, paymentLoaded, canOnlineTopup, stripParams]);

  // URL 带非法/开关已关的 tab 时写回实际 Tab(replace,不污染历史)。
  useEffect(() => {
    if (isOrgContext || wantTopup || !requestedTab || requestedTab === tab) return;
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        params.set('tab', tab);
        return params;
      },
      { replace: true }
    );
  }, [isOrgContext, wantTopup, requestedTab, tab, setSearchParams]);

  const handleTabChange = (next) => {
    setSearchParams((prev) => {
      const params = new URLSearchParams(prev);
      params.set('tab', next);
      return params;
    });
  };

  // 整页刷新:余额 + 当前 Tab 列表。
  const handleRefresh = () => {
    reloadBalance();
    setReloadToken((v) => v + 1);
  };

  // 组织成员直访重定向回日志页(hooks 已在上方全部声明)。
  if (isOrgContext && !isOrgAdmin) return <Navigate to="/panel/log" replace />;

  const balanceBanner = (
    <div className="flex flex-col gap-1 rounded-xl bg-muted p-6">
      <div className="flex items-center gap-1.5">
        <span className="text-sm text-muted-foreground">{t('billingPage.availableBalance')}</span>
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                aria-label={t('billingPage.availableBalanceHint')}
                className="flex items-center text-muted-foreground transition-colors hover:text-foreground"
              >
                <Info className="size-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top" className="max-w-xs">
              {t('billingPage.availableBalanceHint')}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
      <span className="text-4xl font-semibold tabular-nums">{balance === null ? '—' : `$${balance.toFixed(2)}`}</span>
      <span className="text-xs text-muted-foreground">
        {isOrgContext ? t('billingPage.poolCaption') : t('billingPage.balanceCaption')}
      </span>
    </div>
  );

  const topupPanel = (
    <Card className="flex h-full flex-col">
      <CardHeader className="flex-row items-start gap-3 space-y-0 p-4">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Wallet className="size-4" />
        </span>
        <div className="flex flex-col gap-1">
          <CardTitle className="text-sm font-semibold">{t('topup')}</CardTitle>
          <p className="text-xs text-muted-foreground">{t('billingPage.topupDescription')}</p>
        </div>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col gap-4 p-4 pt-0">
        {canOnlineTopup ? (
          <Button className="w-full" onClick={() => setTopupOpen(true)}>
            {t('topup')}
          </Button>
        ) : (
          paymentLoaded && <p className="text-sm text-muted-foreground">{t('billingPage.onlineTopupUnavailable')}</p>
        )}
        <div className="mt-auto flex flex-wrap items-center gap-4 text-sm">
          <Link to="/panel/usage" className="text-muted-foreground transition-colors hover:text-foreground">
            {t('billingPage.viewUsage')} →
          </Link>
          <button
            type="button"
            onClick={() => setRedemptionOpen(true)}
            className="text-muted-foreground transition-colors hover:text-foreground"
          >
            {t('billingPage.useRedemption')}
          </button>
        </div>
      </CardContent>
    </Card>
  );

  const dialogs = (
    <>
      <TopupDialog
        open={topupOpen}
        onClose={() => setTopupOpen(false)}
        payment={payment}
        selectedPayment={selectedPayment}
        setSelectedPayment={setSelectedPayment}
        onSuccess={reloadBalance}
      />
      <RedemptionDialog open={redemptionOpen} onClose={() => setRedemptionOpen(false)} onSuccess={reloadBalance} />
    </>
  );

  // 页头右上角:整页刷新(余额 + 当前 Tab)。
  const pageActions = (
    <PageActions>
      <ResponsiveToolbarButton variant="outline" size="sm" icon={RefreshCw} label={t('billingPage.refresh')} onClick={handleRefresh} />
    </PageActions>
  );

  if (isOrgContext) {
    return (
      <div className="flex flex-col gap-4">
        {pageActions}
        <Alert variant="warning">
          <AlertTriangle />
          <AlertDescription>{t('org.topupPoolAlert')}</AlertDescription>
        </Alert>
        {balanceBanner}
        {topupPanel}
        {dialogs}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {pageActions}
      {balanceBanner}

      <div className={`grid gap-4 ${inviteEnabled ? 'md:grid-cols-2' : ''}`}>
        {topupPanel}
        {inviteEnabled && <InviteCard className="h-full" />}
      </div>

      <div className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold">{t('billingPage.recentRecords')}</h2>
          <Tabs value={tab} onValueChange={handleTabChange}>
            <TabsList>
              <TabsTrigger value="ledger">{t('billingPage.tabs.ledger')}</TabsTrigger>
              <TabsTrigger value="orders">{t('billingPage.tabs.orders')}</TabsTrigger>
              {invoiceEnabled && <TabsTrigger value="invoice">{t('billingPage.tabs.invoice')}</TabsTrigger>}
            </TabsList>
          </Tabs>
        </div>

        {tab === 'orders' && <OrdersTab payment={payment} reloadToken={reloadToken} />}
        {tab === 'invoice' && <Invoice reloadToken={reloadToken} />}
        {tab === DEFAULT_TAB && <Ledger reloadToken={reloadToken} />}
      </div>
      {dialogs}
    </div>
  );
}
