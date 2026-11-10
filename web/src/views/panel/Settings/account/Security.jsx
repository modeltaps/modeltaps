import { adminLoginEnabled, isExternalAccountSystem } from 'utils/authAvailability';
import LoginMethodsCard from './LoginMethodsCard';
import IdentifiersCard from './IdentifiersCard';
import SessionsCard from './SessionsCard';
import AdminEmergencyCard from './AdminEmergencyCard';
import AccessTokenCard from './Tokens';

// ==============================|| SETTINGS — ACCOUNT / SIGN-IN & SECURITY ||============================== //
// The cards of 登录与安全, which change with the account system rather than the page: sign-in
// verification → account identifiers → login sessions → access token → admin emergency login
// (root only, external account system). Each card says first what it is and what to do; the term
// is always 通行密钥, never WebAuthn or a vendor name.

export const showEmergencyCard = (siteInfo, inputs) =>
  isExternalAccountSystem(siteInfo) && inputs?.role === 100 && adminLoginEnabled(siteInfo);

// [{ id, render }] for SettingsPage; `self` is the shared /api/user/self state.
export function securityCards({ siteInfo, self }) {
  const { inputs, reloadUser } = self;
  const props = { siteInfo, inputs, reloadUser };
  return [
    { id: 'login-verification', render: () => <LoginMethodsCard {...props} /> },
    { id: 'identifiers', render: () => <IdentifiersCard {...props} /> },
    { id: 'sessions', render: () => <SessionsCard /> },
    { id: 'access-token', render: () => <AccessTokenCard /> },
    { id: 'emergency', render: () => (showEmergencyCard(siteInfo, inputs) ? <AdminEmergencyCard {...props} /> : null) }
  ];
}
