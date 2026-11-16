import { Navigate } from 'react-router';

import { useIsRoot } from 'utils/common';
import SystemLogs from './SystemLogs';
import { workbench } from 'routes/paths';

// ==============================|| PANEL — SYSTEM LOGS (root) ||============================== //
// Root-only, like the `/api/system_info` routes behind it (RootAuth) and its menu entry.

export default function SystemInfo() {
  const isRoot = useIsRoot();

  if (!isRoot) return <Navigate to={workbench.dashboard()} replace />;

  return <SystemLogs />;
}
