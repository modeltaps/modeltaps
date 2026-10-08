import { lazy, useContext } from 'react';
import { UserContext } from 'contexts/UserContext';
import { useIsAdmin } from 'utils/common';
import Loadable from 'ui-component/Loadable';

const NotFoundView = Loadable(lazy(() => import('views/public/Error')));

// Frontend-only guard for admin routes (backend already enforces AdminAuth).
// Mirrors AuthGuard: render nothing until the user info has loaded so a slow
// session restore never flashes a 404. Non-admins get the existing 404 view
// rendered in place (URL preserved, the admin page never mounts so no API
// error toasts fire).
const AdminGuard = ({ children }) => {
  const { isUserLoaded } = useContext(UserContext);
  const isAdmin = useIsAdmin();

  if (!isUserLoaded) {
    return null;
  }

  if (!isAdmin) {
    return <NotFoundView />;
  }

  return children;
};

export default AdminGuard;
