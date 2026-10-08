import { useTranslation } from 'react-i18next';

import { verifyJSON } from 'utils/common';
import { SettingsCard, TextRow, SaveButton, CollapsibleHelp } from './parts';

// 支付参数 card: USD rate, minimum amount, fixed-amount recharge discounts and the
// payment callback address (moved here from 通用 · 服务器设置).
// Option keys preserved from v1 OperationSetting paymentSettings.

export default function PaymentSettings({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, saveKeys, loading, isDirty } = ctx;
  const op = (k) => t(`setting_index.operationSettings.paymentSettings.${k}`);
  const sys = (k) => t(`setting_index.systemSettings.${k}`);
  const keys = ['PaymentUSDRate', 'PaymentMinAmount', 'RechargeDiscount', 'PaymentCallbackAddress'];

  return (
    <SettingsCard title={op('title')} description={op('description')} highlight="payment">
      <CollapsibleHelp trigger={op('help.trigger')} lines={[op('help.line1'), op('help.line2'), op('help.line3')]} />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <TextRow
          id="PaymentUSDRate"
          type="number"
          label={op('usdRate.label')}
          value={inputs.PaymentUSDRate}
          onChange={(v) => setField('PaymentUSDRate', v)}
          placeholder={op('usdRate.placeholder')}
          description={op('usdRate.description')}
          disabled={loading}
        />
        <TextRow
          id="PaymentMinAmount"
          type="number"
          label={op('minAmount.label')}
          value={inputs.PaymentMinAmount}
          onChange={(v) => setField('PaymentMinAmount', v)}
          placeholder={op('minAmount.placeholder')}
          disabled={loading}
        />
      </div>
      <TextRow
        id="PaymentCallbackAddress"
        label={sys('generalSettings.paymentCallbackAddress')}
        value={inputs.PaymentCallbackAddress}
        onChange={(v) => setField('PaymentCallbackAddress', v)}
        placeholder={sys('generalSettings.paymentCallbackAddressPlaceholder')}
        description={sys('generalSettings.paymentCallbackAddressDescription')}
        disabled={loading}
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
        disabled={loading}
      />
      <CollapsibleHelp trigger={op('discountHelp.trigger')} lines={[op('discountHelp.line1'), op('discountHelp.line2')]} />
      <SaveButton
        loading={loading}
        disabled={!isDirty(keys)}
        onClick={() =>
          saveKeys(keys, {
            validate: (i) => (i.RechargeDiscount && !verifyJSON(i.RechargeDiscount) ? op('invalidJson') : null)
          })
        }
      >
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}
