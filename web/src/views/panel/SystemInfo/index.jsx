import { Navigate } from 'react-router';

import { useIsAdmin, useIsRoot } from 'utils/common';
import SystemLogs from './SystemLogs';
import SystemLogQuery from './SystemLogQuery';

// ==============================|| PANEL — SYSTEM INFO (admin) ||============================== //
// shadcn/Tailwind port of v1 `views/SystemInfo`. Admin-only; renders the live
// system log viewer backed by the unchanged `/api/system_info/log` API. The
// advanced log query panel below is root-only (backend enforces RootAuth).

export default function SystemInfo() {
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();

  if (!isAdmin) return <Navigate to="/panel/dashboard" replace />;

  return (
    <div className="space-y-6">
      <SystemLogs />
      {isRoot && <SystemLogQuery />}
    </div>
  );
}
