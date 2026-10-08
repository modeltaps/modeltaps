import { Navigate, useLocation } from 'react-router';

import PaymentOrder from './PaymentOrder';

// ==============================|| PANEL — PAYMENT ORDERS (ADMIN) ||============================== //
// Order list only. Gateway CRUD moved to the「计费与支付」settings theme (IA-2);
// the legacy `#gateway` deep link redirects there, mirroring the Setting `?tab=`
// fallback.

export default function Payment() {
  const location = useLocation();

  if (location.hash.replace('#', '') === 'gateway') {
    return <Navigate to="/panel/setting/billing?highlight=payment-gateways" replace />;
  }

  return (
    <div className="space-y-6">
      <PaymentOrder />
    </div>
  );
}
