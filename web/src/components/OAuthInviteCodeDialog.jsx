import { useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { API } from 'utils/api';
import { brandName } from 'utils/brand';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';

// ==============================|| OAUTH INVITE-CODE DIALOG (shadcn) ||============================== //

const PROVIDER_NAMES = {
  github: 'GitHub',
  linuxdo: 'Linux Do'
};

// OIDC 提供方的名字来自 /api/status 的 oidc_providers：first_party 就是本站账号，用站点名；
// 其余用 display_name，后端未下发时退回 slug。旧的 /oauth/oidc 回调没有 slug，按存量别名 oidc 查。
// 查不到提供方时也用站点名兜底，不暴露「OIDC」。
const oidcProviderName = (siteInfo, providerSlug) => {
  const providers = Array.isArray(siteInfo?.oidc_providers) ? siteInfo.oidc_providers : [];
  const target = providers.find((item) => item.slug === (providerSlug || 'oidc')) || (providers.length === 1 ? providers[0] : null);
  if (!target || target.first_party) return brandName(siteInfo?.system_name);
  return target.display_name || target.slug;
};

export default function OAuthInviteCodeDialog({ open, onClose, onConfirm, provider, providerSlug = '' }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const [inviteCode, setInviteCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const providerName =
    provider === 'oidc'
      ? oidcProviderName(siteInfo, providerSlug)
      : { lark: t('oauthInvite.larkName'), wechat: t('oauthInvite.wechatName') }[provider] || PROVIDER_NAMES[provider] || provider;

  const handleSubmit = async () => {
    if (!inviteCode.trim()) {
      setError(t('oauthInvite.enterCode'));
      return;
    }

    setLoading(true);
    setError('');

    try {
      const response = await API.post('/api/oauth/invite_code', {
        invite_code: inviteCode.trim()
      });

      const { success, message } = response.data;
      if (success) {
        onConfirm();
      } else {
        setError(message || t('oauthInvite.verifyFailed'));
      }
    } catch (err) {
      setError(t('oauthInvite.networkError'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('oauthInvite.title', { provider: providerName })}</DialogTitle>
          <DialogDescription>{t('oauthInvite.description')}</DialogDescription>
        </DialogHeader>

        <DialogBody className="space-y-4">
          {error && (
            <Alert variant="error">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-4">
            <Label htmlFor="oauth-invite-code">{t('oauthInvite.label')}</Label>
            <Input
              id="oauth-invite-code"
              value={inviteCode}
              onChange={(e) => {
                setInviteCode(e.target.value);
                setError('');
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleSubmit();
              }}
              placeholder={t('oauthInvite.placeholder')}
              disabled={loading}
              autoFocus
              required
            />
            <p className="text-xs text-muted-foreground">{t('oauthInvite.required')}</p>
          </div>
        </DialogBody>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={loading || !inviteCode.trim()}>
            {loading && <Loader2 className="mr-2 size-4 animate-spin" />}
            {loading ? t('oauthInvite.verifying') : t('oauthInvite.confirmContinue')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

OAuthInviteCodeDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  onConfirm: PropTypes.func.isRequired,
  provider: PropTypes.string.isRequired,
  providerSlug: PropTypes.string
};
