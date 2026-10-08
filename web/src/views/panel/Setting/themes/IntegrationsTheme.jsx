import { useTranslation } from 'react-i18next';

import { ChatCard, MjNotifyCard, BrandIconCard } from '../OtherSettings';
import Telegram from '../../Telegram';
import { SettingsCard } from '../parts';

// ==============================|| SYSTEM SETTINGS — 集成 ||============================== //
// 内置聊天与聊天链接 / Telegram Bot(整页嵌入)/ Midjourney 回调 / 品牌图标。

export default function IntegrationsTheme({ ctx }) {
  const { t } = useTranslation();

  return (
    <div className="space-y-6">
      <ChatCard ctx={ctx} />
      <SettingsCard title={t('setting_index.cards.telegram')} description={t('telegramPage.infoMessage')} highlight="telegram">
        <Telegram embedded />
      </SettingsCard>
      <MjNotifyCard ctx={ctx} />
      <BrandIconCard ctx={ctx} />
    </div>
  );
}
