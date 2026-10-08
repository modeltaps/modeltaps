import PropTypes from 'prop-types';
import { useMemo, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { Dialog, DialogBody, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { showError } from 'utils/common';
import PayDialog from './PayDialog';
import { OnlineTopup } from './TopupSections';

// ==============================|| PANEL — TOPUP DIALOG ||============================== //
// 在线充值弹层:金额预设 / 自定义 + 支付方式 + 手续费与折扣预览,点「充值」后交给
// 现有 PayDialog 下单(校验与费率计算沿用原 TopupCard)。支付渠道列表由调用方
// (usePaymentMethods)持有,余额卡据此决定是否显示「充值」按钮。
// 订单支付完成时回调 onSuccess,让调用方重拉余额。

export default function TopupDialog({ open, onClose, payment, selectedPayment, setSelectedPayment, onSuccess }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);

  const [amount, setAmount] = useState(0);
  const [discountTotal, setDiscountTotal] = useState(0);
  const [payOpen, setPayOpen] = useState(false);
  const [disabledPay, setDisabledPay] = useState(false);

  const RechargeDiscount = useMemo(() => {
    if (!siteInfo.RechargeDiscount || siteInfo.RechargeDiscount === '') return {};
    try {
      return JSON.parse(siteInfo.RechargeDiscount);
    } catch (e) {
      return {};
    }
  }, [siteInfo.RechargeDiscount]);

  const handleSetAmount = (value) => {
    const num = Number(value);
    setAmount(num);
    const discount = RechargeDiscount[num] || 1;
    setDiscountTotal(num * discount);
  };

  const handleAmountChange = (event) => {
    const value = event.target.value;
    if (value === '') {
      setAmount('');
      setDiscountTotal(0);
      return;
    }
    handleSetAmount(value);
  };

  const calculateFee = () => {
    if (!selectedPayment) return 0;
    if (selectedPayment.fixed_fee > 0) return Number(selectedPayment.fixed_fee);
    const discount = RechargeDiscount[amount] || 1;
    const newAmount = amount * discount;
    return parseFloat(selectedPayment.percent_fee * Number(newAmount)).toFixed(2);
  };

  const calculateTotal = () => {
    if (amount === 0) return 0;
    const discount = RechargeDiscount[amount] || 1;
    const newAmount = amount * discount;
    let total = Number(newAmount) + Number(calculateFee());
    const currencyRate = selectedPayment?.currency_rate > 0 ? selectedPayment.currency_rate : 1;
    if (selectedPayment && selectedPayment.currency === 'CNY') {
      total = parseFloat((total * siteInfo.PaymentUSDRate * currencyRate).toFixed(2));
    } else {
      total = parseFloat((total * currencyRate).toFixed(2));
    }
    return total;
  };

  const handlePay = () => {
    if (!selectedPayment) {
      showError(t('topupCard.selectPaymentMethod'));
      return;
    }
    if (amount <= 0 || amount < siteInfo.PaymentMinAmount) {
      showError(`${t('topupCard.amountMinLimit')} ${siteInfo.PaymentMinAmount}`);
      return;
    }
    if (amount > 1000000) {
      showError(t('topupCard.amountMaxLimit'));
      return;
    }
    if (!/^[1-9]\d*$/.test(amount)) {
      showError(t('topupCard.positiveIntegerAmount'));
      return;
    }
    setDisabledPay(true);
    setPayOpen(true);
  };

  const onClosePayDialog = () => {
    setPayOpen(false);
    setDisabledPay(false);
  };

  return (
    <>
      <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('topupCard.onlineTopup')}</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <OnlineTopup
              t={t}
              payment={payment}
              selectedPayment={selectedPayment}
              setSelectedPayment={setSelectedPayment}
              RechargeDiscount={RechargeDiscount}
              amount={amount}
              discountTotal={discountTotal}
              handleSetAmount={handleSetAmount}
              handleAmountChange={handleAmountChange}
              calculateFee={calculateFee}
              calculateTotal={calculateTotal}
              siteInfo={siteInfo}
              handlePay={handlePay}
              disabledPay={disabledPay}
            />
          </DialogBody>
        </DialogContent>
      </Dialog>

      {selectedPayment && (
        <PayDialog open={payOpen} onClose={onClosePayDialog} amount={amount} uuid={selectedPayment.uuid} onSuccess={onSuccess} />
      )}
    </>
  );
}

TopupDialog.propTypes = {
  open: PropTypes.bool,
  onClose: PropTypes.func.isRequired,
  payment: PropTypes.array,
  selectedPayment: PropTypes.object,
  setSelectedPayment: PropTypes.func,
  onSuccess: PropTypes.func
};
