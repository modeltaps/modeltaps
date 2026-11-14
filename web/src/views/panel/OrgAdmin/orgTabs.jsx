import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { useEffect, useMemo, useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import QuotaInput from '@/components/QuotaInput';
import { Field, InfoRow, InfoRows, Section, Segmented, StatRow } from '@/components/objects';
import { SpendAmount, renderQuota, timestamp2string } from 'utils/common';
import { API } from 'utils/api';
import { allowlistPermits } from 'utils/modelAllowlist';
import ModelLimitSelector, { toLimitModels } from 'views/panel/Token/ModelLimitSelector';

// Tabs of the organization panel (see components/objects/ObjectPanel). Overview is read-only;
// Balance applies one adjustment of the pool and Members sets the member limit, each through its
// own endpoint.

// 成员上限三种模式:default=跟随站点默认(提交 null),unlimited=不限制(0),custom=指定正整数
const MAX_MEMBER_MODES = [
  { value: 'default', labelKey: 'orgAdmin.maxMembersModeDefault' },
  { value: 'unlimited', labelKey: 'orgAdmin.maxMembersModeUnlimited' },
  { value: 'custom', labelKey: 'orgAdmin.maxMembersModeCustom' }
];

export function memberLimit(org) {
  return org.effective_max_members > 0 ? org.effective_max_members : '∞';
}

function OverviewBody({ object: org }) {
  const { t } = useTranslation();
  return (
    <>
      <StatRow
        items={[
          { label: t('orgAdmin.columns.quota'), value: renderQuota(org.quota || 0, 2) },
          { label: t('orgAdmin.columns.usedQuota'), value: <SpendAmount quota={org.used_quota} /> },
          {
            label: t('orgAdmin.columns.members'),
            value: `${org.member_count} / ${memberLimit(org)}`,
            hint: org.max_members == null ? t('orgAdmin.panel.followsDefault') : null
          }
        ]}
      />
      <Section title={t('orgAdmin.panel.details')}>
        <InfoRows>
          <InfoRow label="ID" mono>
            {org.id}
          </InfoRow>
          <InfoRow label={t('orgAdmin.panel.slug')} mono>
            {org.slug}
          </InfoRow>
          <InfoRow label={t('orgAdmin.columns.owner')} mono>
            {org.owner_id ? `#${org.owner_id}` : '-'}
          </InfoRow>
          <InfoRow label={t('orgAdmin.columns.status')}>{org.status === 1 ? t('common.enable') : t('common.disable')}</InfoRow>
          <InfoRow label={t('orgAdmin.columns.createdTime')}>{timestamp2string(org.created_time)}</InfoRow>
        </InfoRows>
      </Section>
    </>
  );
}

const overviewTab = { key: 'overview', Body: OverviewBody };

function BalanceBody({ object: org, draft, setDraft }) {
  const { t } = useTranslation();
  return (
    <>
      <Field label={t('orgAdmin.columns.quota')}>
        <p className="text-lg font-medium tabular-nums">{renderQuota(org.quota || 0, 2)}</p>
      </Field>
      <Field hint={t('orgAdmin.quotaDesc')}>
        <QuotaInput
          id="org-quota-delta"
          label={t('orgAdmin.quotaLabel')}
          value={draft.delta}
          onChange={(v) => setDraft((d) => ({ ...d, delta: v }))}
          maxDeduct={org.quota}
        />
      </Field>
      <Field label={t('orgAdmin.remarkLabel')} htmlFor="org-quota-remark" optional>
        <Textarea
          id="org-quota-remark"
          rows={2}
          value={draft.remark}
          onChange={(e) => setDraft((d) => ({ ...d, remark: e.target.value }))}
          placeholder={t('orgAdmin.remarkPlaceholder')}
        />
      </Field>
    </>
  );
}

const balanceTab = {
  key: 'balance',
  make: () => ({ delta: '', remark: '' }),
  patch: (d, org, { t }) => {
    const quota = Number(d.delta);
    if (!Number.isInteger(quota) || quota === 0) return { error: t('orgAdmin.quotaNonZero') };
    return { quota, remark: d.remark.trim() };
  },
  Body: BalanceBody
};

function MembersBody({ object: org, draft, setDraft }) {
  const { t } = useTranslation();
  return (
    <>
      <Field label={t('orgAdmin.maxMembers')} hint={t('orgAdmin.maxMembersDesc', { count: org.member_count ?? 0 })}>
        <Segmented
          label={t('orgAdmin.maxMembers')}
          value={draft.mode}
          onChange={(mode) => setDraft((d) => ({ ...d, mode }))}
          options={MAX_MEMBER_MODES.map((m) => ({ value: m.value, label: t(m.labelKey) }))}
        />
      </Field>
      {draft.mode === 'custom' && (
        <Field label={t('orgAdmin.maxMembersValueLabel')} htmlFor="org-max-members-value">
          <Input
            id="org-max-members-value"
            type="number"
            min={1}
            className="w-40 tabular-nums"
            value={draft.value}
            onChange={(e) => setDraft((d) => ({ ...d, value: e.target.value }))}
          />
        </Field>
      )}
    </>
  );
}

const membersTab = {
  key: 'members',
  make: (org) => {
    if (org.max_members == null) return { mode: 'default', value: '' };
    if (org.max_members === 0) return { mode: 'unlimited', value: '' };
    return { mode: 'custom', value: String(org.max_members) };
  },
  patch: (d, org, { t }) => {
    if (d.mode === 'default') return { max_members: null };
    if (d.mode === 'unlimited') return { max_members: 0 };
    const n = Number(d.value);
    if (!Number.isInteger(n) || n < 1) return { error: t('orgAdmin.maxMembersPositive') };
    return { max_members: n };
  },
  Body: MembersBody
};

// Models released within this many days that the approved models do not cover are listed as new,
// so a release can be reviewed (or picked up by a prefix rule) the week it ships.
const NEW_MODEL_DAYS = 45;

function ApprovedModelsBody({ draft, setDraft }) {
  const { t } = useTranslation();
  const [models, setModels] = useState([]);
  const [since] = useState(() => Date.now() / 1000 - NEW_MODEL_DAYS * 86400);
  useEffect(() => {
    let alive = true;
    API.get('/api/available_model')
      .then((res) => alive && setModels(res.data.success ? toLimitModels(res.data.data) : []))
      .catch(() => alive && setModels([]));
    return () => {
      alive = false;
    };
  }, []);
  const fresh = useMemo(() => {
    if (draft.models.length === 0) return [];
    return models
      .filter((m) => m.released_at && m.released_at >= since && !allowlistPermits(draft.models, m.id))
      .sort((a, b) => b.released_at - a.released_at)
      .map((m) => m.id);
  }, [models, draft.models, since]);
  return (
    <>
      <Field label={t('modelAccess.approved.title')} hint={t('modelAccess.approved.adminHint')}>
        <ModelLimitSelector
          id="org-approved-models"
          models={models}
          value={draft.models}
          onChange={(ids) => setDraft((d) => ({ ...d, models: ids }))}
        />
        <p className="mt-1.5 text-xs text-muted-foreground">
          {draft.models.length ? t('modelAccess.patternHint') : t('modelAccess.approved.none')}
        </p>
      </Field>
      {fresh.length > 0 && (
        <Field label={t('modelAccess.approved.newTitle')} hint={t('modelAccess.approved.newHint', { days: NEW_MODEL_DAYS })}>
          <div className="flex flex-wrap items-center gap-1">
            {fresh.map((id) => (
              <Badge key={id} variant="secondary" className="font-mono text-[12px] font-normal">
                {id}
              </Badge>
            ))}
            <Button
              variant="outline"
              size="sm"
              className="ml-1 h-7 rounded-lg"
              onClick={() => setDraft((d) => ({ ...d, models: [...d.models, ...fresh] }))}
            >
              {t('modelAccess.approved.approveAll')}
            </Button>
          </div>
        </Field>
      )}
    </>
  );
}

const approvedTab = {
  key: 'models',
  make: (org) => ({ models: org.approved_models || [] }),
  patch: (d) => ({ models: d.models }),
  Body: ApprovedModelsBody
};

export const ORG_TABS = [overviewTab, balanceTab, membersTab, approvedTab];
export const ORG_TAB_KEYS = ORG_TABS.map((x) => x.key);

const bodyProps = { object: PropTypes.object, draft: PropTypes.object, setDraft: PropTypes.func, ctx: PropTypes.object };
OverviewBody.propTypes = bodyProps;
BalanceBody.propTypes = bodyProps;
MembersBody.propTypes = bodyProps;
ApprovedModelsBody.propTypes = bodyProps;
