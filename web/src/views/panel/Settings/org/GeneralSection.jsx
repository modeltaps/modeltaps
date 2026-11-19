import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';

import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { FormCard, OptionList } from '@/components/settings/cards';
import { Skeleton } from '@/components/ui/skeleton';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import { SelectRow, ToggleRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ORG GENERAL ||============================== //
// Organization profile card (Admin+): name, avatar, usage visibility and request / response
// retention, saved by the card footer. The danger zone is its own group (DangerSection).

// The retention policy of the organization's API keys (ORG-10): each key decides, or every key
// locked on or off. Never chosen means each key decides.
const LOG_IO_POLICIES = ['member', 'on', 'off'];
const policyFromServer = (v) => (LOG_IO_POLICIES.includes(v) ? v : 'member');

// 脏值比较统一归一为字符串,null / undefined / '' 视为相等(as isDirty in Setting/parts.jsx)。
const sameValue = (a, b) => String(a ?? '') === String(b ?? '');

const DEFAULT_MAX_BODY_KB = 1024;

export default function GeneralSection({ orgId, role, detail, onRefresh }) {
  const { t } = useTranslation();
  const { refreshOrganizations, refreshOrgDetail } = useOrg();
  const isAdmin = role === 'owner' || role === 'admin';
  const org = detail?.organization;
  const siteInfo = useSelector((state) => state.siteInfo);
  const logIoSiteEnabled = Boolean(siteInfo?.log_io_enabled);
  const logIoMaxBodyKb = Number(siteInfo?.log_io_max_body_kb) || DEFAULT_MAX_BODY_KB;

  const [name, setName] = useState('');
  const [avatarUrl, setAvatarUrl] = useState('');
  // 成员可见全员用量:组织上的普通开关,未设置即关闭
  const [usageVisible, setUsageVisible] = useState(false);
  // 组织留存策略(member / on / off),经 PUT /api/org/:id 写入。
  const [logIoPolicy, setLogIoPolicy] = useState('member');
  const [saving, setSaving] = useState(false);
  // 表单基线:随 org 回流刷新,用于把保存按钮按脏值置灰
  const [baseline, setBaseline] = useState(null);

  useEffect(() => {
    if (!org) return;
    const next = {
      name: org.name || '',
      avatarUrl: org.avatar_url || '',
      usageVisible: org.setting?.usage_visible_to_members === true,
      logIoPolicy: policyFromServer(org.setting?.log_io_policy)
    };
    setName(next.name);
    setAvatarUrl(next.avatarUrl);
    setUsageVisible(next.usageVisible);
    setLogIoPolicy(next.logIoPolicy);
    setBaseline(next);
  }, [org]);

  // 未取到组织信息前一律视为无改动
  const profileDirty =
    !!baseline &&
    (!sameValue(name.trim(), baseline.name.trim()) ||
      !sameValue(avatarUrl, baseline.avatarUrl) ||
      !sameValue(usageVisible, baseline.usageVisible) ||
      !sameValue(logIoPolicy, baseline.logIoPolicy));

  const saveProfile = async () => {
    if (!name.trim()) {
      showError(t('orgPage.settings.nameRequired'));
      return;
    }
    setSaving(true);
    try {
      const payload = { name: name.trim(), avatar_url: avatarUrl.trim() };
      if (!sameValue(usageVisible, baseline?.usageVisible)) payload.usage_visible_to_members = usageVisible;
      // 未改动留存策略时不带该字段,后端保持原值
      if (!sameValue(logIoPolicy, baseline?.logIoPolicy)) payload.log_io_policy = logIoPolicy;
      const res = await API.put(`/api/org/${orgId}/`, payload);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.settings.saveSuccess'));
        // 不等 onRefresh 回流,先把基线设为刚提交的值,按钮立即回到禁用
        setName(payload.name);
        setAvatarUrl(payload.avatar_url);
        setBaseline({ name: payload.name, avatarUrl: payload.avatar_url, usageVisible, logIoPolicy });
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
  // Until the detail arrives: a skeleton the size of the card, never empty inputs that could be saved.
  if (!baseline) return <Skeleton className="h-[240px] rounded-2xl" aria-busy="true" />;

  const reset = () => {
    if (!baseline) return;
    setName(baseline.name);
    setAvatarUrl(baseline.avatarUrl);
    setUsageVisible(baseline.usageVisible);
    setLogIoPolicy(baseline.logIoPolicy);
  };

  return (
    <FormCard
      title={t('orgPage.settings.profile')}
      description={t('settingsUi.org.cards.profile.description')}
      hint={t('settingsUi.org.cards.profile.hint')}
      highlight="profile"
      dirty={profileDirty}
      saving={saving}
      onSave={saveProfile}
      onReset={reset}
    >
      <div className="grid gap-5 sm:grid-cols-2">
        <FormField id="org-settings-name" label={t('orgPage.settings.name')}>
          <Input id="org-settings-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} />
        </FormField>
        <FormField id="org-settings-avatar" label={t('orgPage.settings.avatarUrl')}>
          {/* A small preview next to the address, so a wrong link shows at once. */}
          <div className="flex items-center gap-2">
            <Input id="org-settings-avatar" value={avatarUrl} maxLength={500} onChange={(e) => setAvatarUrl(e.target.value)} />
            {avatarUrl.trim() && (
              <img
                src={avatarUrl.trim()}
                alt=""
                className="size-9 shrink-0 rounded-md border border-border object-cover"
                onError={(e) => {
                  e.currentTarget.style.visibility = 'hidden';
                }}
                onLoad={(e) => {
                  e.currentTarget.style.visibility = 'visible';
                }}
              />
            )}
          </div>
        </FormField>
      </div>
      {/* With retention off site-wide there is one option: no inner frame then. */}
      <OptionList className={logIoSiteEnabled ? undefined : 'divide-y-0 rounded-none border-0 [&>div]:px-0 [&>div]:py-0'}>
        <ToggleRow
          id="org-usage-visible"
          label={t('orgPage.settings.usageVisible')}
          description={t('orgPage.settings.usageVisibleHint')}
          checked={usageVisible}
          onCheckedChange={setUsageVisible}
        />
        {logIoSiteEnabled && (
          <div data-highlight="log-io" className="px-4 py-3">
            <SelectRow
              id="org-log-io"
              label={t('orgPage.settings.logIOPolicy')}
              description={t(`orgPage.settings.logIOPolicyHint_${logIoPolicy}`, { kb: logIoMaxBodyKb })}
              value={logIoPolicy}
              onChange={setLogIoPolicy}
              options={LOG_IO_POLICIES.map((p) => ({ value: p, label: t(`orgPage.settings.logIOPolicy_${p}`) }))}
            />
          </div>
        )}
      </OptionList>
    </FormCard>
  );
}

GeneralSection.propTypes = {
  orgId: PropTypes.number.isRequired,
  role: PropTypes.string,
  detail: PropTypes.object,
  onRefresh: PropTypes.func
};
