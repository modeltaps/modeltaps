import PrivacyCard from './Privacy';
import ProfileCard from './Profile';
import AppearanceCard from './Preferences';
import QuotaRemindCard from './QuotaRemindCard';
import { securityCards } from './Security';

// ==============================|| SETTINGS — PERSONAL SETTINGS GROUPS ||============================== //
// Cards of the four personal groups (ids and labels live in ../sections.js). `self` is the shared
// /api/user/self state, loaded once for the page.

export function accountGroupCards({ siteInfo, self }) {
  return {
    profile: [{ id: 'profile', render: () => <ProfileCard self={self} /> }],
    security: securityCards({ siteInfo, self }),
    privacy: [{ id: 'log-io', render: () => <PrivacyCard /> }],
    preferences: [
      { id: 'appearance', render: () => <AppearanceCard /> },
      { id: 'quota-remind', render: () => <QuotaRemindCard self={self} /> }
    ]
  };
}
