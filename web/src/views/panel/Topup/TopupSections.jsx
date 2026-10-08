import PropTypes from 'prop-types';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { cn } from '@/lib/utils';
import PaymentIcon from '../Payment/PaymentIcon';

function SummaryRow({ label, value }) {
  return (
    <div className="flex items-center justify-between gap-4 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  );
}

SummaryRow.propTypes = { label: PropTypes.node, value: PropTypes.node };

export function OnlineTopup(props) {
  const {
    t,
    payment,
    selectedPayment,
    setSelectedPayment,
    RechargeDiscount,
    amount,
    discountTotal,
    handleSetAmount,
    handleAmountChange,
    calculateFee,
    calculateTotal,
    siteInfo,
    handlePay,
    disabledPay
  } = props;

  const feeSuffix = selectedPayment
    ? selectedPayment.fixed_fee > 0
      ? `(${t('topupCard.fixed')})`
      : selectedPayment.percent_fee > 0
        ? `(${selectedPayment.percent_fee * 100}%)`
        : ''
    : '';

  const renderActualAmount = () => {
    if (!selectedPayment) return null;
    const currencyRate = selectedPayment.currency_rate > 0 ? selectedPayment.currency_rate : 1;
    const isCNY = selectedPayment.currency === 'CNY';
    const total = calculateTotal();
    // CNY 网关：以人民币实付金额为主，附注美元面额与本次使用的收款汇率
    if (isCNY) {
      const usdTotal = parseFloat((Number(discountTotal || amount) + Number(calculateFee())).toFixed(2));
      const note = [
        `${t('topupCard.usdDenomination')}: $${usdTotal}`,
        t('topupCard.rateNote', { rate: siteInfo.PaymentUSDRate }),
        currencyRate !== 1 ? `${t('topupCard.rate')}: ${currencyRate}` : ''
      ]
        .filter(Boolean)
        .join(' · ');
      return (
        <span className="text-right">
          <span className="font-medium">¥{total}</span>
          <span className="block text-xs text-muted-foreground">{note}</span>
        </span>
      );
    }
    return (
      <span className="text-right">
        <span className="font-medium">
          {total} {selectedPayment.currency || 'USD'}
        </span>
        {currencyRate !== 1 && <span className="block text-xs text-muted-foreground">{`${t('topupCard.rate')}: ${currencyRate}`}</span>}
      </span>
    );
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        {payment.map((item, index) => (
          <Button
            key={index}
            variant="outline"
            size="lg"
            className={cn('w-full justify-center gap-2', selectedPayment === item && 'border-foreground')}
            onClick={() => setSelectedPayment(item)}
          >
            <PaymentIcon icon={item.icon} size={24} />
            {item.name}
          </Button>
        ))}
      </div>

      {Object.keys(RechargeDiscount).length > 0 && (
        <div className="flex flex-wrap gap-2">
          {Object.entries(RechargeDiscount).map(([key, value]) => (
            <div key={key} className="relative">
              {value !== 1 && (
                <Badge variant="destructive" className="absolute -right-2 -top-2 z-10">
                  {t('topupCard.discountBadge', { percent: Math.round((1 - value) * 100) })}
                </Badge>
              )}
              <Button
                variant="outline"
                className={cn(amount === Number(key) && 'border-foreground')}
                onClick={() => handleSetAmount(key)}
              >
                ${key}
              </Button>
            </div>
          ))}
        </div>
      )}

      <div className="grid gap-2">
        <Label htmlFor="topup-amount">{t('topupCard.amount')}</Label>
        <Input id="topup-amount" type="number" value={amount} onChange={handleAmountChange} />
      </div>

      <Separator />
      <div className="flex flex-col gap-2">
        <SummaryRow label={`${t('topupCard.topupAmount')}:`} value={`$${Number(amount)}`} />
        {discountTotal !== amount && <SummaryRow label={`${t('topupCard.discountedPrice')}:`} value={`$${discountTotal}`} />}
        {selectedPayment && (selectedPayment.percent_fee > 0 || selectedPayment.fixed_fee > 0) && (
          <SummaryRow label={`${t('topupCard.fee')}: ${feeSuffix}`} value={`$${calculateFee()}`} />
        )}
        <SummaryRow label={`${t('topupCard.actualAmountToPay')}:`} value={renderActualAmount()} />
      </div>
      <Separator />

      <Button className="w-full" onClick={handlePay} disabled={disabledPay}>
        {t('topupCard.topup')}
      </Button>
    </div>
  );
}

OnlineTopup.propTypes = {
  t: PropTypes.func.isRequired,
  payment: PropTypes.array,
  selectedPayment: PropTypes.object,
  setSelectedPayment: PropTypes.func,
  RechargeDiscount: PropTypes.object,
  amount: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  discountTotal: PropTypes.number,
  handleSetAmount: PropTypes.func,
  handleAmountChange: PropTypes.func,
  calculateFee: PropTypes.func,
  calculateTotal: PropTypes.func,
  siteInfo: PropTypes.object,
  handlePay: PropTypes.func,
  disabledPay: PropTypes.bool
};

export function RedemptionTopup({ t, redemptionCode, setRedemptionCode, topUp, isSubmitting, siteInfo, openTopUpLink }) {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-2">
        <Label htmlFor="redemption-key">{t('topupCard.inputLabel')}</Label>
        <div className="flex items-center gap-2">
          <Input
            id="redemption-key"
            type="text"
            value={redemptionCode}
            onChange={(e) => setRedemptionCode(e.target.value)}
            placeholder={t('topupCard.inputPlaceholder')}
          />
          <Button onClick={topUp} disabled={isSubmitting} className="shrink-0">
            {isSubmitting ? t('topupCard.exchangeButton.submitting') : t('topupCard.exchangeButton.default')}
          </Button>
        </div>
      </div>

      {siteInfo.top_up_link && (
        <div className="flex flex-col items-center gap-3 pt-2">
          <p className="text-sm text-muted-foreground">{t('topupCard.noRedemptionCodeText')}</p>
          <Button onClick={openTopUpLink}>{t('topupCard.getRedemptionCode')}</Button>
        </div>
      )}
    </div>
  );
}

RedemptionTopup.propTypes = {
  t: PropTypes.func.isRequired,
  redemptionCode: PropTypes.string,
  setRedemptionCode: PropTypes.func,
  topUp: PropTypes.func,
  isSubmitting: PropTypes.bool,
  siteInfo: PropTypes.object,
  openTopUpLink: PropTypes.func
};
