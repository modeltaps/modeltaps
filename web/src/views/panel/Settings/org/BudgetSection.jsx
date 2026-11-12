import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { FormField } from '@/components/ui/form-field';
import { FormCard } from '@/components/settings/cards';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { renderSpend, showError, showSuccess } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';

// ==============================|| SETTINGS — ORG BUDGET ||============================== //
// Team budget card: budget / budget_used are returned to Admin+ only; an empty period sentinel
// means no budget. Part of the organization settings 常规 group.

// 脏值比较统一归一为字符串,null / undefined / '' 视为相等(as isDirty in Setting/parts.jsx)。
const sameValue = (a, b) => String(a ?? '') === String(b ?? '');

export default function BudgetSection({ orgId, role, detail, onRefresh }) {
  const { t } = useTranslation();
  const { refreshOrgDetail } = useOrg();
  const isAdmin = role === 'owner' || role === 'admin';
  const org = detail?.organization;

  const [budgetPeriod, setBudgetPeriod] = useState('__none');
  const [budgetLimit, setBudgetLimit] = useState('');
  const [savingBudget, setSavingBudget] = useState(false);
  // 表单基线:随 detail 回流刷新,用于把保存按钮按脏值置灰
  const [baseline, setBaseline] = useState(null);

  useEffect(() => {
    if (!detail) return;
    const b = detail.budget;
    const next = {
      budgetPeriod: b?.period && b?.limit > 0 ? b.period : '__none',
      budgetLimit: b?.limit > 0 ? b.limit : ''
    };
    setBudgetPeriod(next.budgetPeriod);
    setBudgetLimit(next.budgetLimit);
    setBaseline(next);
  }, [detail]);

  // 未取到组织详情前一律视为无改动;选「不限」与原本无预算都归一为 __none/'',视为相等
  const budgetDirty =
    !!baseline &&
    (!sameValue(budgetPeriod, baseline.budgetPeriod) || (budgetPeriod !== '__none' && !sameValue(budgetLimit, baseline.budgetLimit)));

  // 组织预算单独提交:name/avatar_url 为后端必填且会被覆写,回填服务端当前值,
  // 避免与个人信息卡未保存的草稿相互污染。limit<=0 即后端语义的「清除限制」。
  const saveBudget = async () => {
    const hasPeriod = budgetPeriod !== '__none';
    const limit = Math.trunc(Number(budgetLimit) || 0);
    if (hasPeriod && limit <= 0) {
      showError(t('orgPage.limits.limitRequired'));
      return;
    }
    setSavingBudget(true);
    try {
      const payload = {
        name: org?.name || '',
        avatar_url: org?.avatar_url || '',
        budget: { period: hasPeriod ? budgetPeriod : 'monthly', limit: hasPeriod ? limit : 0 }
      };
      const res = await API.put(`/api/org/${orgId}/`, payload);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.settings.saveSuccess'));
        // 不等 onRefresh 回流,先把基线设为刚提交的值,按钮立即回到禁用
        setBudgetLimit(hasPeriod ? limit : '');
        setBaseline({ budgetPeriod, budgetLimit: hasPeriod ? limit : '' });
        onRefresh?.();
        refreshOrgDetail();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSavingBudget(false);
    }
  };

  if (!isAdmin) return null;

  return (
    <FormCard
      title={t('orgPage.settings.teamBudget')}
      description={t('orgPage.settings.teamBudgetHint')}
      hint={t('settingsUi.org.cards.budget.hint')}
      highlight="team-budget"
      dirty={budgetDirty}
      saving={savingBudget}
      onSave={saveBudget}
      onReset={() => {
        if (!baseline) return;
        setBudgetPeriod(baseline.budgetPeriod);
        setBudgetLimit(baseline.budgetLimit);
      }}
    >
      <div className="grid gap-5 sm:grid-cols-2">
        <FormField label={t('orgPage.limits.budget')}>
          <Select value={budgetPeriod} onValueChange={setBudgetPeriod}>
            <SelectTrigger>
              <span className="truncate">
                {budgetPeriod === '__none' ? t('orgPage.limits.budgetNone') : t(`orgPage.limits.period_${budgetPeriod}`)}
              </span>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__none">{t('orgPage.limits.budgetNone')}</SelectItem>
              <SelectItem value="daily">{t('orgPage.limits.period_daily')}</SelectItem>
              <SelectItem value="weekly">{t('orgPage.limits.period_weekly')}</SelectItem>
              <SelectItem value="monthly">{t('orgPage.limits.period_monthly')}</SelectItem>
            </SelectContent>
          </Select>
        </FormField>
        {budgetPeriod !== '__none' && (
          <QuotaInput
            id="org-team-budget-limit"
            label={t('orgPage.limits.limitLabel')}
            value={budgetLimit}
            onChange={setBudgetLimit}
            allowEmpty
            helperText={t('orgPage.limits.budgetUsed', { used: renderSpend(detail?.budget_used || 0) })}
          />
        )}
      </div>
    </FormCard>
  );
}

BudgetSection.propTypes = {
  orgId: PropTypes.number.isRequired,
  role: PropTypes.string,
  detail: PropTypes.object,
  onRefresh: PropTypes.func
};
