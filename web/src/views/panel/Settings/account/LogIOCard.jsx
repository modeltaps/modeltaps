import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';

import { API } from 'utils/api';
import { saveUserSetting } from 'utils/userSetting';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';

// ==============================|| SETTINGS — ACCOUNT / LOG IO CARD ||============================== //
// 个人 LogIO 留存开关,渲染在「数据与隐私」section:后端存三态(未设置 / on / off),
// 前端只呈现「生效值」开关——未设置时取站点默认 log_io_default_user,拨动后写入显式 on/off。
// 站点总闸门 log_io_enabled 关闭时整张卡不渲染。
// 日志明细弹窗以 ?highlight=log-io 深链到本卡片(高亮 hook 挂在 account 分组层)。

// 服务端三态可能回 "inherit"|"on"|"off" 字符串,也可能回 *bool;统一归一为哨兵字符串。
const triFromServer = (v) => (v === 'on' || v === true ? 'on' : v === 'off' || v === false ? 'off' : 'inherit');

const DEFAULT_MAX_BODY_KB = 1024;

export default function LogIOCard() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const siteEnabled = Boolean(siteInfo?.log_io_enabled);
  const siteDefault = Boolean(siteInfo?.log_io_default_user);
  const maxBodyKb = Number(siteInfo?.log_io_max_body_kb) || DEFAULT_MAX_BODY_KB;
  const [logIoDefault, setLogIoDefault] = useState('inherit');

  useEffect(() => {
    const loadUserSetting = async () => {
      try {
        const res = await API.get('/api/user/setting');
        const { success, data } = res.data || {};
        if (success && data) setLogIoDefault(triFromServer(data.log_io_default));
      } catch {
        /* 静默降级:拉取失败时维持哨兵默认 */
      }
    };
    loadUserSetting();
  }, []);

  if (!siteEnabled) return null;

  const checked = logIoDefault === 'inherit' ? siteDefault : logIoDefault === 'on';

  const toggle = (next) => {
    const value = next ? 'on' : 'off';
    setLogIoDefault(value);
    saveUserSetting({ log_io_default: value });
  };

  return (
    <Card data-highlight="log-io">
      <CardHeader>
        <CardTitle className="text-lg">{t('profilePage.logIOSection')}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label className="font-normal">{t('profilePage.logIOToggle')}</Label>
            <p className="text-xs text-muted-foreground">{t('profilePage.logIOHint', { kb: maxBodyKb })}</p>
          </div>
          <Switch checked={checked} onCheckedChange={toggle} />
        </div>
      </CardContent>
    </Card>
  );
}
