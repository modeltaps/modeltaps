import { lazy, useContext } from 'react';
import { UserContext } from 'contexts/UserContext';
import { useIsRoot } from 'utils/common';
import Loadable from 'ui-component/Loadable';

const NotFoundView = Loadable(lazy(() => import('views/public/Error')));

// Frontend-only guard for root routes (backend already enforces RootAuth).
// Mirrors AdminGuard: render nothing until the user info has loaded so a slow
// session restore never flashes a 404. Non-root users get the existing 404 view
// rendered in place (URL preserved, the root page never mounts so no API
// error toasts fire).
const RootGuard = ({ children }) => {
  const { isUserLoaded } = useContext(UserContext);
  const isRoot = useIsRoot();

  if (!isUserLoaded) {
    return null;
  }

  if (!isRoot) {
    return <NotFoundView />;
  }

  return children;
};

export default RootGuard;
