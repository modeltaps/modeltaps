import { useTranslation } from 'react-i18next';

import useTheme from 'hooks/useTheme';
import i18nList from 'i18n/i18nList';
import { setAppLanguage } from 'utils/userSetting';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

// ==============================|| SETTINGS — ACCOUNT / PREFERENCES ||============================== //
// 主题与语言:原先挂在侧边栏用户菜单的子菜单,这里作为设置面「个人账号」组的常驻页面。
// 两项都即改即生效并自行持久化(主题写 localStorage + 服务端偏好,语言由 i18next 的
// language detector 落 localStorage),因此没有保存按钮。

export default function Preferences() {
  const { t, i18n } = useTranslation();
  const { theme, setTheme } = useTheme();
  // 语言列表以后还会变长,因此语言用下拉而不是分段控件;主题固定三项仍用 Tabs。
  const currentLanguage = i18nList.find((item) => item.lng === i18n.language) || i18nList[0];

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">{t('theme.title', { defaultValue: 'Theme' })}</CardTitle>
          <CardDescription>{t('settingsPage.preferences.themeHint')}</CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs value={theme} onValueChange={setTheme}>
            <TabsList>
              <TabsTrigger value="light">{t('theme.light', { defaultValue: 'Light' })}</TabsTrigger>
              <TabsTrigger value="dark">{t('theme.dark', { defaultValue: 'Dark' })}</TabsTrigger>
              <TabsTrigger value="system">{t('theme.auto', { defaultValue: 'System' })}</TabsTrigger>
            </TabsList>
          </Tabs>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">{t('language.select', { defaultValue: 'Language' })}</CardTitle>
          <CardDescription>{t('settingsPage.preferences.languageHint')}</CardDescription>
        </CardHeader>
        <CardContent>
          <Select value={currentLanguage.lng} onValueChange={setAppLanguage}>
            <SelectTrigger className="w-56 max-w-full" aria-label={t('language.select', { defaultValue: 'Language' })}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {i18nList.map((item) => (
                <SelectItem key={item.lng} value={item.lng}>
                  {item.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </CardContent>
      </Card>
    </div>
  );
}
