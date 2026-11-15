import { Navigate, useLocation } from 'react-router';

import PaymentOrder from './PaymentOrder';
import { admin } from 'routes/paths';

// ==============================|| PANEL — PAYMENT ORDERS (ADMIN) ||============================== //
// Order list only. Payment gateways are a collection card in platform settings (支付);
// the legacy `#gateway` deep link redirects there.

export default function Payment() {
  const location = useLocation();

  if (location.hash.replace('#', '') === 'gateway') {
    return <Navigate to={`${admin.settings('payment')}?highlight=payment-gateways`} replace />;
  }

  return (
    <div className="space-y-6">
      <PaymentOrder />
    </div>
  );
}
