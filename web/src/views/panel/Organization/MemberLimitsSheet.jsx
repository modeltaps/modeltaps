import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { FormField } from '@/components/ui/form-field';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { renderSpend, showError, showSuccess } from 'utils/common';
import ModelAccessField, { accessPayload, accessValueFrom } from '@/components/model/ModelAccessField';
import { toLimitModels } from '../Token/ModelLimitSelector';

// ==============================|| ORGANIZATION — MEMBER BUDGET & MODEL WHITELIST (T5) ||============================== //
// Sheet editor for org_members.setting: periodic budget (daily/weekly/monthly,
// UTC boundaries) + model access (ORG-15): follow the org default, a named list, or an inline list,
// all inside the platform's approved models for the organization.

export default function MemberLimitsSheet({ open, onOpenChange, orgId, member, onSaved }) {
  const { t } = useTranslation();
  const [period, setPeriod] = useState('__none');
  const [limit, setLimit] = useState('');
  const [access, setAccess] = useState(accessValueFrom(0, []));
  const [scope, setScope] = useState(null);
  const [lists, setLists] = useState([]);
  const [budgetUsed, setBudgetUsed] = useState(0);
  const [models, setModels] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [limitError, setLimitError] = useState('');

  useEffect(() => {
    if (!open || !member) return;
    // 每次打开先重置，避免残留上一成员的预算 / 白名单
    setLimitError('');
    setPeriod('__none');
    setLimit('');
    setAccess(accessValueFrom(0, []));
    setScope(null);
    setLists([]);
    setBudgetUsed(0);
    setModels([]);
    setLoading(true);
    Promise.all([
      API.get(`/api/org/${orgId}/members/${member.user_id}/limits`),
      API.get('/api/available_model'),
      API.get(`/api/org/${orgId}/model_access`, { params: { user_id: member.user_id } })
    ])
      .then(([limitsRes, modelsRes, accessRes]) => {
        const { success, message, data } = limitsRes.data;
        if (!success) {
          showError(message || t('usagePage.loadFailed'));
          onOpenChange(false);
          return;
        }
        if (!modelsRes.data.success || !accessRes.data.success) {
          showError(modelsRes.data.message || accessRes.data.message || t('usagePage.loadFailed'));
          onOpenChange(false);
          return;
        }
        setScope(accessRes.data.data.scope);
        setLists(accessRes.data.data.lists || []);
        setPeriod(data?.budget?.period || '__none');
        setLimit(data?.budget?.limit ?? '');
        setAccess(accessValueFrom(data?.model_access_list_id || 0, data?.model_whitelist || []));
        setBudgetUsed(data?.budget_used || 0);
        const d = modelsRes.data.data;
        setModels(toLimitModels(d));
      })
      .catch((error) => {
        showError(error);
        onOpenChange(false);
      })
      .finally(() => setLoading(false));
  }, [open, orgId, member]);

  const submit = async () => {
    let budget = null;
    if (period !== '__none') {
      const lim = parseInt(limit, 10);
      if (!Number.isFinite(lim) || lim <= 0) {
        setLimitError(t('orgPage.limits.limitRequired'));
        return;
      }
      budget = { period, limit: lim };
    }
    setSaving(true);
    try {
      const { listId, models: whitelist } = accessPayload(access);
      const res = await API.put(`/api/org/${orgId}/members/${member.user_id}/limits`, {
        budget,
        model_whitelist: whitelist,
        model_access_list_id: listId
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.limits.saveSuccess'));
        onSaved?.();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{t('orgPage.limits.title')}</SheetTitle>
          <SheetDescription>{member?.display_name || member?.username || `#${member?.user_id}`}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          {loading ? (
            <div className="flex justify-center p-8">
              <Loader2 className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <>
              <FormField label={t('orgPage.limits.budget')}>
                <Select
                  value={period}
                  onValueChange={(v) => {
                    setLimitError('');
                    setPeriod(v);
                  }}
                >
                  <SelectTrigger>
                    <span className="truncate">
                      {period === '__none' ? t('orgPage.limits.budgetNone') : t(`orgPage.limits.period_${period}`)}
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
              {period !== '__none' && (
                <FormField id="member-budget-limit" label={t('orgPage.limits.limitLabel')} required error={limitError}>
                  <QuotaInput
                    id="member-budget-limit"
                    name="budget_limit"
                    value={limit}
                    error={Boolean(limitError)}
                    helperText={t('orgPage.limits.budgetUsed', { used: renderSpend(budgetUsed) })}
                    onChange={(v) => {
                      setLimitError('');
                      setLimit(v);
                    }}
                  />
                </FormField>
              )}
              <FormField className="border-t border-border pt-4" label={t('orgPage.limits.whitelist')}>
                <ModelAccessField
                  id="member-model-limit"
                  value={access}
                  onChange={setAccess}
                  lists={lists}
                  models={models}
                  parents={scope?.approved_models?.length ? [scope.approved_models] : []}
                  inheritLabel={t('modelAccess.mode.followOrg')}
                  inheritHint={describeOrgDefault(t, scope, lists)}
                />
              </FormField>
            </>
          )}
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving || loading}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

// One line on what "follow the organization default" means right now.
export function describeOrgDefault(t, scope, lists) {
  if (!scope) return '';
  if (scope.default_list_id > 0) {
    const list = lists.find((l) => l.id === scope.default_list_id);
    return t('modelAccess.orgDefaultList', { name: list?.name || `#${scope.default_list_id}` });
  }
  if (scope.default_models?.length) return t('modelAccess.orgDefaultModels', { models: scope.default_models.join(', ') });
  return scope.approved_models?.length ? t('modelAccess.orgDefaultApproved') : t('modelAccess.orgDefaultNone');
}

MemberLimitsSheet.propTypes = {
  open: PropTypes.bool.isRequired,
  onOpenChange: PropTypes.func.isRequired,
  orgId: PropTypes.number.isRequired,
  member: PropTypes.object,
  onSaved: PropTypes.func
};
