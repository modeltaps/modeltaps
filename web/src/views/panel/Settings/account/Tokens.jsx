import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { KeyRound } from 'lucide-react';

import { API } from 'utils/api';
import { copy, showError } from 'utils/common';
import { ActionCard, ActionRow } from '@/components/settings/cards';
import { TextRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / ACCESS TOKEN ||============================== //
// The access token for the management API (not an API key for model calls): generated or reset as
// a one-off action. Resetting invalidates the previous token, so it asks first.

export default function AccessTokenCard() {
  const { t } = useTranslation();
  const [accessToken, setAccessToken] = useState('');

  const generate = async () => {
    const res = await API.get('/api/user/token');
    const { success, message, data } = res.data;
    if (success) {
      setAccessToken(data);
      copy(data, t('profilePage.token'));
    } else {
      showError(message);
    }
    return success;
  };

  return (
    <ActionCard
      title={t('settingsUi.personal.cards.accessToken.title')}
      description={t('profilePage.tokenNotice')}
      highlight="access-token"
    >
      <ActionRow
        icon={KeyRound}
        title={t('settingsUi.personal.cards.accessToken.title')}
        description={t('settingsUi.personal.cards.accessToken.rowHint')}
        actionLabel={t('settingsUi.personal.cards.accessToken.reset')}
        confirm={{
          title: t('profilePage.resetToken'),
          description: t('settingsUi.personal.cards.accessToken.confirm'),
          label: t('profilePage.generateToken')
        }}
        onAction={generate}
      />
      {accessToken && (
        <div className="pb-2 pt-1">
          <TextRow
            id="access_token"
            label={t('profilePage.yourTokenIs')}
            value={accessToken}
            onChange={() => {}}
            readOnly
            copyable
            description={t('profilePage.keepSafe')}
          />
        </div>
      )}
    </ActionCard>
  );
}
