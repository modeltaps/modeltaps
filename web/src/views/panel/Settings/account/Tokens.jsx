import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { API } from 'utils/api';
import { copy, showError } from 'utils/common';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Alert } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { TextRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / ACCESS TOKEN ||============================== //
// 系统管理用访问 Token(非 API 调用 Key)的生成与重置。

export default function Tokens() {
  const { t } = useTranslation();
  const [accessToken, setAccessToken] = useState('');

  const generateAccessToken = async () => {
    const res = await API.get('/api/user/token');
    const { success, message, data } = res.data;
    if (success) {
      setAccessToken(data);
      copy(data, t('profilePage.token'));
    } else {
      showError(message);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">{t('profilePage.token')}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <Alert variant="info">{t('profilePage.tokenNotice')}</Alert>
        {accessToken && (
          <TextRow
            id="access_token"
            label={t('profilePage.yourTokenIs')}
            value={accessToken}
            onChange={() => {}}
            readOnly
            copyable
            description={t('profilePage.keepSafe')}
          />
        )}
        <Button onClick={generateAccessToken}>{accessToken ? t('profilePage.resetToken') : t('profilePage.generateToken')}</Button>
      </CardContent>
    </Card>
  );
}
