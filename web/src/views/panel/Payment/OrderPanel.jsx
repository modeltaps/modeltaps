import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Copy, ReceiptText, UserRound } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/ui/sonner';
import { InfoRow, InfoRows, ObjectPanel, ObjectTile, Section, StatRow } from '@/components/objects';
import { timestamp2string } from 'utils/common';
import { ORDER_STATUS, fetchOrders } from './paymentApi';

function OrderBody({ object: order }) {
  const { t } = useTranslation();
  const status = ORDER_STATUS[order.status];
  return (
    <>
      <StatRow
        items={[
          { label: t('orderlogPage.tableHeaders.amount'), value: `$${order.amount}` },
          { label: t('orderlogPage.tableHeaders.order_amount'), value: `${order.order_amount} ${order.order_currency || ''}`.trim() },
          { label: t('orderlogPage.tableHeaders.quota'), value: order.quota }
        ]}
      />
      <Section title={t('orderlogPage.panel.details')}>
        <InfoRows>
          <InfoRow label={t('orderlogPage.tableHeaders.status')}>
            <Badge variant={status?.variant || 'secondary'}>{status ? t(status.labelKey) : t('common.unknown')}</Badge>
          </InfoRow>
          <InfoRow label={t('orderlogPage.tableHeaders.created_at')}>{timestamp2string(order.created_at)}</InfoRow>
          <InfoRow label={t('orderlogPage.tableHeaders.trade_no')} mono>
            {order.trade_no}
          </InfoRow>
          <InfoRow label={t('orderlogPage.tableHeaders.gateway_no')} mono>
            {order.gateway_no || '-'}
          </InfoRow>
          <InfoRow label={t('orderlogPage.tableHeaders.gateway_id')} mono>
            {order.gateway_id}
          </InfoRow>
          <InfoRow label={t('orderlogPage.tableHeaders.user_id')} mono>
            {order.user_id}
          </InfoRow>
        </InfoRows>
      </Section>
      <Section title={t('orderlogPage.panel.amounts')}>
        <InfoRows>
          <InfoRow label={t('orderlogPage.tableHeaders.fee')}>${order.fee}</InfoRow>
          <InfoRow label={t('orderlogPage.tableHeaders.discount')}>
            {order.discount} {order.order_currency}
          </InfoRow>
          {/* 历史订单未记录汇率(0)时显示 -,避免误导 */}
          <InfoRow label={t('orderlogPage.tableHeaders.exchange_rate')}>{order.exchange_rate > 0 ? order.exchange_rate : '-'}</InfoRow>
        </InfoRows>
      </Section>
    </>
  );
}

OrderBody.propTypes = { object: PropTypes.object };

const orderTab = { key: 'details', Body: OrderBody };

// Right-side panel of one payment order, addressed by its trade number. Orders are records of a
// payment, so the panel is read-only; the header copies the trade number and links to the user.
export default function OrderPanel({ tradeNo, rows, onClose }) {
  const { t } = useTranslation();
  const fromList = rows.find((r) => r.trade_no === tradeNo) || null;
  const [fetched, setFetched] = useState(null);
  const [error, setError] = useState('');
  const order = fromList || fetched;
  const tabs = useMemo(() => [{ ...orderTab, label: t('orderlogPage.panel.details') }], [t]);

  // A link to an order that is not on the current page looks it up by its trade number.
  useEffect(() => {
    if (fromList) return undefined;
    let alive = true;
    fetchOrders(0, 10, { trade_no: tradeNo }).then((data) => {
      if (!alive) return;
      const hit = (data?.data || []).find((r) => r.trade_no === tradeNo);
      if (hit) setFetched(hit);
      else setError(t('objects.notFound'));
    });
    return () => {
      alive = false;
    };
  }, [fromList, tradeNo, t]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(order.trade_no);
      toast.success(t('orderlogPage.panel.copied'));
    } catch {
      toast.error(order.trade_no);
    }
  };

  return (
    <ObjectPanel
      titleId="order-panel-title"
      size="sm"
      object={order}
      error={error}
      icon={
        <ObjectTile>
          <ReceiptText />
        </ObjectTile>
      }
      title={order && `$${order.amount}`}
      subtitle={order?.trade_no}
      subtitleMono
      actions={
        order && (
          <>
            <Button variant="outline" size="sm" className="rounded-lg" onClick={copy}>
              <Copy />
              {t('orderlogPage.panel.copyTradeNo')}
            </Button>
            <Button variant="ghost" size="icon" className="size-8 rounded-lg" asChild aria-label={t('orderlogPage.panel.viewUser')}>
              <Link to={`/panel/user?id=${order.user_id}`} title={t('orderlogPage.panel.viewUser')}>
                <UserRound className="size-4" />
              </Link>
            </Button>
          </>
        )
      }
      tabs={tabs}
      tab="details"
      onTabChange={() => {}}
      onClose={onClose}
    />
  );
}

OrderPanel.propTypes = { tradeNo: PropTypes.string.isRequired, rows: PropTypes.array.isRequired, onClose: PropTypes.func.isRequired };
