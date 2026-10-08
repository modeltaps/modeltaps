import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import useHighlightTarget from 'hooks/useHighlightTarget';
import DangerSection from './DangerSection';

// ==============================|| SETTINGS — ORG GENERAL ||============================== //
// 组织资料卡:名称 / 头像 / 用量可见性 / 请求响应留存(Admin+),其后接危险操作卡
// (退出组织 / 转让所有权 / 解散组织),不再单列 section。
// 由 /panel/settings/org/:orgId/general 与组织工作台设置 Tab 共用。

// 服务端三态可能回 "inherit"|"on"|"off" 字符串,也可能回 *bool;统一归一为哨兵字符串。
const triFromServer = (v) => (v === 'on' || v === true ? 'on' : v === 'off' || v === false ? 'off' : 'inherit');

// 脏值比较统一归一为字符串,null / undefined / '' 视为相等(同 Setting/parts.jsx 的 isDirty)。
const sameValue = (a, b) => String(a ?? '') === String(b ?? '');

const DEFAULT_MAX_BODY_KB = 1024;

export default function GeneralSection({ orgId, role, detail, onRefresh }) {
  const { t } = useTranslation();
  const { refreshOrganizations, refreshOrgDetail } = useOrg();
  const isAdmin = role === 'owner' || role === 'admin';
  useHighlightTarget();
  const org = detail?.organization;
  const siteInfo = useSelector((state) => state.siteInfo);
  const logIoSiteEnabled = Boolean(siteInfo?.log_io_enabled);
  const logIoSiteDefault = Boolean(siteInfo?.organization_log_io_default);
  const logIoMaxBodyKb = Number(siteInfo?.log_io_max_body_kb) || DEFAULT_MAX_BODY_KB;

  const [name, setName] = useState('');
  const [avatarUrl, setAvatarUrl] = useState('');
  // null = 未显式设置(沿用站点默认);切换开关后变为明确布尔值
  const [usageVisible, setUsageVisible] = useState(null);
  // 组织留存三态哨兵(inherit=未设置,取站点默认 / on / off),经 PUT /api/org/:id 写入。
  const [logIoDefault, setLogIoDefault] = useState('inherit');
  const [saving, setSaving] = useState(false);
  // 表单基线:随 org 回流刷新,用于把保存按钮按脏值置灰
  const [baseline, setBaseline] = useState(null);

  useEffect(() => {
    if (!org) return;
    const next = {
      name: org.name || '',
      avatarUrl: org.avatar_url || '',
      usageVisible: org.setting?.usage_visible_to_members ?? null,
      logIoDefault: triFromServer(org.setting?.log_io_default)
    };
    setName(next.name);
    setAvatarUrl(next.avatarUrl);
    setUsageVisible(next.usageVisible);
    setLogIoDefault(next.logIoDefault);
    setBaseline(next);
  }, [org]);

  // 未取到组织资料前一律视为无改动
  const profileDirty =
    !!baseline &&
    (!sameValue(name.trim(), baseline.name.trim()) ||
      !sameValue(avatarUrl, baseline.avatarUrl) ||
      !sameValue(usageVisible, baseline.usageVisible) ||
      !sameValue(logIoDefault, baseline.logIoDefault));

  const saveProfile = async () => {
    if (!name.trim()) {
      showError(t('orgPage.settings.nameRequired'));
      return;
    }
    setSaving(true);
    try {
      const payload = { name: name.trim(), avatar_url: avatarUrl.trim() };
      if (usageVisible !== null) payload.usage_visible_to_members = usageVisible;
      // 未拨动过留存开关时不带该字段,后端保持原有三态
      if (!sameValue(logIoDefault, baseline?.logIoDefault)) payload.log_io_default = logIoDefault;
      const res = await API.put(`/api/org/${orgId}/`, payload);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.settings.saveSuccess'));
        // 不等 onRefresh 回流,先把基线设为刚提交的值,按钮立即回到禁用
        setName(payload.name);
        setAvatarUrl(payload.avatar_url);
        setBaseline({ name: payload.name, avatarUrl: payload.avatar_url, usageVisible, logIoDefault });
        onRefresh?.();
        refreshOrganizations();
        // 侧栏 / 仪表盘读的是 OrgContext.orgDetail,同步重拉一次即时生效(UX-25)
        refreshOrgDetail();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  if (!isAdmin) return null;

  return (
    <div className="space-y-6">
      <Card className="space-y-5 p-6">
        <h3 className="text-base font-semibold">{t('orgPage.settings.profile')}</h3>
        <div className="space-y-4">
          <Label htmlFor="org-settings-name">{t('orgPage.settings.name')}</Label>
          <Input id="org-settings-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-4">
          <Label htmlFor="org-settings-avatar">{t('orgPage.settings.avatarUrl')}</Label>
          <Input id="org-settings-avatar" value={avatarUrl} maxLength={500} onChange={(e) => setAvatarUrl(e.target.value)} />
        </div>
        <div className="flex items-center justify-between gap-4 py-1">
          <div className="space-y-0.5">
            <Label className="font-normal">{t('orgPage.settings.usageVisible')}</Label>
            <p className="text-xs text-muted-foreground">{t('orgPage.settings.usageVisibleHint')}</p>
          </div>
          <Switch checked={usageVisible === true} onCheckedChange={setUsageVisible} />
        </div>
        {logIoSiteEnabled && (
          <div data-highlight="log-io" className="flex items-center justify-between gap-4 py-1">
            <div className="space-y-0.5">
              <Label className="font-normal">{t('orgPage.settings.logIOToggle')}</Label>
              <p className="text-xs text-muted-foreground">{t('orgPage.settings.logIOHint', { kb: logIoMaxBodyKb })}</p>
            </div>
            <Switch
              checked={logIoDefault === 'inherit' ? logIoSiteDefault : logIoDefault === 'on'}
              onCheckedChange={(next) => setLogIoDefault(next ? 'on' : 'off')}
            />
          </div>
        )}
        <div className="flex justify-end">
          <Button onClick={saveProfile} disabled={saving || !profileDirty}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('orgPage.settings.save')}
          </Button>
        </div>
      </Card>
      <div id="danger">
        <DangerSection orgId={orgId} role={role} detail={detail} onRefresh={onRefresh} />
      </div>
    </div>
  );
}

GeneralSection.propTypes = {
  orgId: PropTypes.number.isRequired,
  role: PropTypes.string,
  detail: PropTypes.object,
  onRefresh: PropTypes.func
};
