import { useCallback, useEffect, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Power, Trash2 } from 'lucide-react';

import { API } from 'utils/api';
import { showError, showSuccess, verifyJSON } from 'utils/common';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { toast } from '@/components/ui/sonner';
import { ActionCard, ActionRow, CollectionCard, StatusDot } from '@/components/settings/cards';
import PaymentIcon from '../../Payment/PaymentIcon';
import PaymentSheet from '../../Payment/PaymentSheet';
import ConfirmDialog from '../../Payment/ConfirmDialog';
import { fetchGateways, gatewayTypeLabel, managePayment } from '../../Payment/paymentApi';
import { CollapsibleHelp, TextRow } from '../parts';
import OptionCard, { cardText } from './OptionCard';

// ==============================|| PLATFORM SETTINGS — 支付 ||============================== //
// Payment settings, the payment gateways and the monthly invoice actions.

export function PaymentSettingsCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const op = (k) => t(`setting_index.operationSettings.paymentSettings.${k}`);
  const sys = (k) => t(`setting_index.systemSettings.${k}`);
  const validate = (i) => (i.RechargeDiscount && !verifyJSON(i.RechargeDiscount) ? op('invalidJson') : null);

  return (
    <OptionCard ctx={ctx} id="payment" description={op('description')} options={{ validate }}>
      <CollapsibleHelp trigger={op('help.trigger')} lines={[op('help.line1'), op('help.line2'), op('help.line3')]} />
      <div className="grid gap-5 sm:grid-cols-2">
        <TextRow
          id="PaymentUSDRate"
          type="number"
          label={op('usdRate.label')}
          value={inputs.PaymentUSDRate}
          onChange={(v) => setField('PaymentUSDRate', v)}
          placeholder={op('usdRate.placeholder')}
          description={op('usdRate.description')}
        />
        <TextRow
          id="PaymentMinAmount"
          type="number"
          label={op('minAmount.label')}
          value={inputs.PaymentMinAmount}
          onChange={(v) => setField('PaymentMinAmount', v)}
          placeholder={op('minAmount.placeholder')}
        />
      </div>
      <TextRow
        id="PaymentCallbackAddress"
        label={sys('generalSettings.paymentCallbackAddress')}
        value={inputs.PaymentCallbackAddress}
        onChange={(v) => setField('PaymentCallbackAddress', v)}
        placeholder={sys('generalSettings.paymentCallbackAddressPlaceholder')}
        description={sys('generalSettings.paymentCallbackAddressDescription')}
      />
      <TextRow
        id="RechargeDiscount"
        label={op('discount.label')}
        value={inputs.RechargeDiscount}
        onChange={(v) => setField('RechargeDiscount', v)}
        placeholder={op('discount.placeholder')}
        description={op('discount.description')}
        multiline
        rows={6}
      />
      <CollapsibleHelp trigger={op('discountHelp.trigger')} lines={[op('discountHelp.line1'), op('discountHelp.line2')]} />
    </OptionCard>
  );
}

// The settings card lists the first page of gateways, most recent first.
const GATEWAY_PAGE_SIZE = 100;

export function PaymentGatewaysCard() {
  const { t } = useTranslation();
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [sheet, setSheet] = useState({ open: false, id: 0 });
  const [deleteTarget, setDeleteTarget] = useState(null);
  const head = (k) => t(`paymentGatewayPage.tableHeaders.${k}`);

  const load = useCallback(async () => {
    setLoading(true);
    const data = await fetchGateways(0, GATEWAY_PAGE_SIZE, {}, 'desc', 'created_at');
    if (data) setRows(data.data || []);
    setLoading(false);
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const run = async (id, action, value) => {
    const { success, message } = await managePayment(id, action, value);
    if (success) {
      toast.success(t('userPage.operationSuccess'));
      load();
    } else if (message) {
      toast.error(message);
    }
  };

  return (
    <>
      <CollectionCard
        title={cardText(t, 'payment-gateways', 'title')}
        description={cardText(t, 'payment-gateways', 'description')}
        highlight="payment-gateways"
        addLabel={t('paymentGatewayPage.createPayment')}
        onAdd={() => setSheet({ open: true, id: 0 })}
        loading={loading}
        rows={rows}
        onRowClick={(row) => setSheet({ open: true, id: row.id })}
        columns={[
          {
            key: 'name',
            header: head('name'),
            cell: (row) => (
              <span className="flex items-center gap-2">
                <PaymentIcon icon={row.icon} size={20} />
                <span className="font-medium">{row.name}</span>
              </span>
            )
          },
          { key: 'type', header: head('type'), cell: (row) => gatewayTypeLabel(row.type, t) || t('common.unknown') },
          {
            key: 'fee',
            header: `${head('fixedFee')} / ${head('percentFee')}`,
            cell: (row) => (
              <span className="tabular-nums text-muted-foreground">
                {row.fixed_fee} / {row.percent_fee}
              </span>
            )
          },
          { key: 'sort', header: head('sort'), className: 'tabular-nums', cell: (row) => row.sort },
          { key: 'enable', header: head('enable'), cell: (row) => <StatusDot on={!!row.enable} /> }
        ]}
        menu={(row) => [
          {
            label: row.enable ? t('settingsUi.collection.disable') : t('settingsUi.collection.enable'),
            icon: Power,
            onClick: () => run(row.id, 'status', !row.enable)
          },
          { label: t('common.delete'), icon: Trash2, destructive: true, separator: true, onClick: () => setDeleteTarget(row) }
        ]}
      />
      <PaymentSheet
        open={sheet.open}
        paymentId={sheet.id}
        onClose={() => setSheet((s) => ({ ...s, open: false }))}
        onSaved={() => {
          setSheet((s) => ({ ...s, open: false }));
          load();
        }}
      />
      <ConfirmDialog
        open={!!deleteTarget}
        title={t('payment_row.delPayment')}
        content={`${t('payment_row.delPaymentTip')} ${deleteTarget?.name || ''}？`}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={() => {
          const target = deleteTarget;
          setDeleteTarget(null);
          if (target) run(target.id, 'delete');
        }}
      />
    </>
  );
}

// Monthly invoices, only when the site lets users request them.
export function InvoiceCard() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const op = (k, opts) => t(`setting_index.operationSettings.${k}`, opts);
  const [month, setMonth] = useState('');

  if (!siteInfo?.UserInvoiceMonth) return null;

  const invoiceAction = async (kind) => {
    if (!month) {
      showError(op('invoice.selectMonth'));
      return false;
    }
    const res = await API.post(`/api/option/invoice/${kind}/${month}-01`);
    const { success, message } = res.data;
    if (success) showSuccess(kind === 'gen' ? op('invoice.genSuccess') : op('invoice.updateSuccess'));
    else showError((kind === 'gen' ? op('invoice.genFailed') : op('invoice.updateFailed')) + message);
    return success;
  };

  const monthField = (
    <FormField id="invoiceMonth" label={op('invoice.genTime')}>
      <Input id="invoiceMonth" type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
    </FormField>
  );

  return (
    <ActionCard
      title={cardText(t, 'invoice', 'title') || op('invoice.title')}
      description={cardText(t, 'invoice', 'description')}
      highlight="invoice"
    >
      <ActionRow
        title={op('invoice.genMonthInvoice')}
        description={t('settingsUi.platform.actions.invoiceGenDescription')}
        actionLabel={op('invoice.genMonthInvoice')}
        confirm={{ body: monthField, disabled: !month, onOpen: () => setMonth('') }}
        onAction={() => invoiceAction('gen')}
      />
      <ActionRow
        title={op('invoice.updateMonthInvoice')}
        description={t('settingsUi.platform.actions.invoiceUpdateDescription')}
        actionLabel={op('invoice.updateMonthInvoice')}
        confirm={{ body: monthField, disabled: !month, onOpen: () => setMonth('') }}
        onAction={() => invoiceAction('update')}
      />
    </ActionCard>
  );
}
