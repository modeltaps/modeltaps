import { useSelector } from 'react-redux';

import { adminLoginEnabled, isExternalAccountSystem } from 'utils/authAvailability';
import LoginMethodsCard from './LoginMethodsCard';
import IdentifiersCard from './IdentifiersCard';
import SessionsCard from './SessionsCard';
import AdminEmergencyCard from './AdminEmergencyCard';
import useSelfUser from './useSelfUser';

// ==============================|| SETTINGS — ACCOUNT / SIGN-IN & SECURITY ||============================== //
// 四个区块按账号体系增减,不换页面:登录验证 → 账号标识 → 登录会话 → 管理员应急登录(仅外部模式下的 root)。
// 每个区块第一句话说清「这是什么、你要做什么」;术语统一用「通行密钥」,不出现 WebAuthn / 供应商名。

export default function Security() {
  const siteInfo = useSelector((state) => state.siteInfo);
  const { inputs, reloadUser } = useSelfUser();
  const showEmergency = isExternalAccountSystem(siteInfo) && inputs.role === 100 && adminLoginEnabled(siteInfo);

  return (
    <div className="space-y-6">
      <LoginMethodsCard siteInfo={siteInfo} inputs={inputs} reloadUser={reloadUser} />
      <IdentifiersCard siteInfo={siteInfo} inputs={inputs} reloadUser={reloadUser} />
      <SessionsCard />
      {showEmergency && <AdminEmergencyCard siteInfo={siteInfo} inputs={inputs} reloadUser={reloadUser} />}
    </div>
  );
}
