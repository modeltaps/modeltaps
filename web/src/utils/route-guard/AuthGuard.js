import { useSelector } from 'react-redux';
import { useEffect, useContext } from 'react';
import { UserContext } from 'contexts/UserContext';
import { useLocation, useNavigate } from 'react-router';

const AuthGuard = ({ children }) => {
  const account = useSelector((state) => state.account);
  const { isUserLoaded } = useContext(UserContext);
  const navigate = useNavigate();
  const location = useLocation();

  // Visitors who open a panel page directly (e.g. the home page's "Get API key" link) come back to
  // it after signing in.
  useEffect(() => {
    if (isUserLoaded && !account.user) {
      navigate(`/login?redirect=${encodeURIComponent(location.pathname + location.search)}`);
      return;
    }
  }, [account, navigate, isUserLoaded, location.pathname, location.search]);

  // 在用户信息加载完成前不渲染子组件
  if (!isUserLoaded) {
    return null;
  }

  return children;
};

export default AuthGuard;
