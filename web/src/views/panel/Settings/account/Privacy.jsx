import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';

import { ActionCard } from '@/components/settings/cards';
import LogIOCard from './LogIOCard';

// ==============================|| SETTINGS — ACCOUNT / DATA & PRIVACY ||============================== //
// Request / response retention. When the site switch log_io_enabled is off, a read-only notice says
// that only usage is recorded, so the group is never empty.

export default function PrivacyCard() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  if (siteInfo?.log_io_enabled) return <LogIOCard />;
  return (
    <ActionCard title={t('profilePage.logIOSection')} highlight="log-io">
      <p className="pb-2 text-sm text-muted-foreground">{t('settingsPage.privacy.siteOffHint')}</p>
    </ActionCard>
  );
}
