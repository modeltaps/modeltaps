import { QuotaCard, InviteRewardCard, QuotaResetCard, BillingRulesCard } from '../GeneralSettings';
import { InvoiceCard } from '../OtherSettings';
import PaymentSettings from '../PaymentSettings';
import PaymentGateway from '../../Payment/PaymentGateway';

// ==============================|| SYSTEM SETTINGS — 计费与支付 ||============================== //
// 额度 / 邀请奖励 / 额度重置周期 / 计费规则 / 支付参数 / 支付网关 / 发票。

export default function BillingTheme({ ctx }) {
  return (
    <div className="space-y-6">
      <QuotaCard ctx={ctx} />
      <InviteRewardCard ctx={ctx} />
      <QuotaResetCard ctx={ctx} />
      <BillingRulesCard ctx={ctx} />
      <PaymentSettings ctx={ctx} />
      {/* 支付网关自带列表工具条与卡片,只包一层高亮锚点。 */}
      <div data-highlight="payment-gateways">
        <PaymentGateway />
      </div>
      <InvoiceCard ctx={ctx} />
    </div>
  );
}
