import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import LogIOCard from './LogIOCard';

// ==============================|| SETTINGS — ACCOUNT / DATA & PRIVACY ||============================== //
// 「数据与隐私」section:当前只有请求 / 响应留存开关,后续的数据导出、注销等隐私项也落这里。
// 站点总闸门 log_io_enabled 关闭时 LogIOCard 自己返回 null,本页改为呈现一张只读的隐私声明卡,
// 说明平台只记录用量、不保存正文,避免整页空白。

export default function Privacy() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const siteEnabled = Boolean(siteInfo?.log_io_enabled);

  return (
    <div className="space-y-6">
      {siteEnabled ? (
        <LogIOCard />
      ) : (
        <Card data-highlight="log-io">
          <CardHeader>
            <CardTitle className="text-lg">{t('profilePage.logIOSection')}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">{t('settingsPage.privacy.siteOffHint')}</p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
