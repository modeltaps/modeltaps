import { Navigate, useLocation } from 'react-router';

import PaymentOrder from './PaymentOrder';

// ==============================|| PANEL — PAYMENT ORDERS (ADMIN) ||============================== //
// Order list only. Payment gateways are a collection card in platform settings (支付);
// the legacy `#gateway` deep link redirects there.

export default function Payment() {
  const location = useLocation();

  if (location.hash.replace('#', '') === 'gateway') {
    return <Navigate to="/panel/setting/payment?highlight=payment-gateways" replace />;
  }

  return (
    <div className="space-y-6">
      <PaymentOrder />
    </div>
  );
}
