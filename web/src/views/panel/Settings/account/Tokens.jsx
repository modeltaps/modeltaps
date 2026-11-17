import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { KeyRound, TriangleAlert } from 'lucide-react';

import { API } from 'utils/api';
import { copy, showError } from 'utils/common';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { ActionCard, ActionRow } from '@/components/settings/cards';
import { workbench } from 'routes/paths';
import { TextRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / ACCESS TOKEN ||============================== //
// The deprecated access token for the management API (not an API key for model calls). Management
// keys replace it (AGENT-4): it still works for one release, and regenerating it here shows the
// new token once, since only its hash is stored. Resetting invalidates the previous token, so it
// asks first.

export default function AccessTokenCard() {
  const { t } = useTranslation();
  const [accessToken, setAccessToken] = useState('');

  const generate = async () => {
    const res = await API.post('/api/user/token');
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
    <ActionCard title={t('settingsUi.personal.cards.accessToken.title')} highlight="access-token">
      <Alert variant="warning" className="mb-2" data-testid="access-token-deprecated">
        <TriangleAlert />
        <AlertDescription>
          {t('settingsUi.personal.cards.accessToken.deprecated')}{' '}
          <Link to={`${workbench.keys()}?new=1`} className="font-medium underline underline-offset-2">
            {t('settingsUi.personal.cards.accessToken.openKeys')}
          </Link>
        </AlertDescription>
      </Alert>
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
            description={t('settingsUi.personal.cards.accessToken.onceNote')}
          />
        </div>
      )}
    </ActionCard>
  );
}
