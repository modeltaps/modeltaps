import { useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ExternalLink, Loader2 } from 'lucide-react';

import { API } from 'utils/api';
import { showSuccess } from 'utils/common';
import { brandName } from 'utils/brand';
import { externalProvider, isExternalAccountSystem } from 'utils/authAvailability';
import { useOrg } from 'contexts/OrgContext';
import { accountSettingsUrl } from 'views/panel/Settings/sections';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Alert, AlertDescription } from '@/components/ui/alert';

// ==============================|| ORG — CREATE ORGANIZATION DIALOG ||============================== //
// 后端要求创建者有一条已验证的联系通道(邮箱或手机号,root 免检),这里在提交前先拦一道,
// 并按账号体系给出去向:本站能验邮箱就链到账号安全页;本站没有邮件服务就让用户找管理员;
// 账号由登录服务托管就深链到登录服务的账号设置页。文案一律说「登录服务」,不出现「外部模式」。

export default function CreateOrgDialog({ open, onClose }) {
  const { t } = useTranslation();
  const { refreshOrganizations, switchOrg } = useOrg();
  const user = useSelector((state) => state.account.user);
  const siteInfo = useSelector((state) => state.siteInfo);
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // 用户信息还没回来时不拦(否则打开即禁用),root 与已有联系方式的人照旧。
  const needsContact = Boolean(user) && user.role !== 100 && !user.email && !user.phone_number;

  const handleClose = () => {
    if (loading) return;
    setName('');
    setError('');
    onClose();
  };

  const handleSubmit = async () => {
    const trimmed = name.trim();
    if (!trimmed || needsContact) return;
    setLoading(true);
    setError('');
    try {
      const res = await API.post('/api/org/', { name: trimmed });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('org.createSuccess'));
        await refreshOrganizations();
        if (data?.id) switchOrg(data.id);
        setName('');
        onClose();
      } else {
        setError(message || t('common.error'));
      }
    } catch {
      // 全局响应拦截器已提示网络错误
    } finally {
      setLoading(false);
    }
  };

  const renderContactGuide = () => {
    if (!needsContact) return null;

    if (isExternalAccountSystem(siteInfo)) {
      const provider = externalProvider(siteInfo) || {};
      const href = provider.identity_url || provider.account_settings_url || '';
      return (
        <Alert variant="warning">
          <AlertDescription>
            <p>{t('org.createNeedsContactManaged', { system_name: brandName(siteInfo?.system_name) })}</p>
            {href && (
              <div className="mt-3">
                <Button asChild variant="outline" size="sm">
                  <a href={href} target="_blank" rel="noopener noreferrer">
                    {t('org.goAccountSettings')}
                    <ExternalLink className="opacity-60" aria-hidden="true" />
                  </a>
                </Button>
              </div>
            )}
          </AlertDescription>
        </Alert>
      );
    }

    // 内置账号:SMTP 没配好时本站给不出绑定入口,只能让管理员在后台代填。
    if (siteInfo?.smtp_configured === false) {
      return (
        <Alert variant="warning">
          <AlertDescription>{t('org.createNeedsContactNoSmtp')}</AlertDescription>
        </Alert>
      );
    }

    return (
      <Alert variant="warning">
        <AlertDescription>
          <p>{t('org.createNeedsContact')}</p>
          <div className="mt-3">
            <Button asChild variant="outline" size="sm">
              <Link to={accountSettingsUrl('security')} onClick={handleClose}>
                {t('org.goBindEmail')}
              </Link>
            </Button>
          </div>
        </AlertDescription>
      </Alert>
    );
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) handleClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('org.createOrg')}</DialogTitle>
          <DialogDescription>{t('org.createOrgDesc')}</DialogDescription>
        </DialogHeader>

        <DialogBody className="space-y-4">
          {renderContactGuide()}

          {error && (
            <Alert variant="error">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <FormField label={t('org.orgName')} htmlFor="org-name">
            <Input
              id="org-name"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setError('');
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleSubmit();
              }}
              placeholder={t('org.orgNamePlaceholder')}
              maxLength={100}
              disabled={loading}
              autoFocus
              required
            />
          </FormField>
        </DialogBody>

        <DialogFooter>
          <Button variant="ghost" onClick={handleClose} disabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={loading || !name.trim() || needsContact}>
            {loading && <Loader2 className="mr-2 size-4 animate-spin" />}
            {t('common.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

CreateOrgDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired
};
