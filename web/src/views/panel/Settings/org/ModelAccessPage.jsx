import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Lock, Pencil, Plus, Trash2 } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { FormField } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Sheet, SheetBody, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { ActionCard, FormCard } from '@/components/settings/cards';
import ModelAccessField, { accessPayload, accessValueFrom } from '@/components/model/ModelAccessField';
import ModelLimitSelector, { toLimitModels } from 'views/panel/Token/ModelLimitSelector';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';

// ==============================|| ORG — MODEL ACCESS (ORG-15) ||============================== //
// 组织 → 模型访问 (owners and admins). The platform's approved models for the organization
// (read-only, set by the site admin), the organization default every member follows unless their
// own limits say otherwise, and the named lists that can be attached to the default, members and
// API keys. Every list stays inside the approved models.

const sameAccess = (a, b) => a.mode === b.mode && a.listId === b.listId && a.models.join('\n') === b.models.join('\n');

function ModelChips({ models }) {
  return (
    <div className="flex flex-wrap gap-1">
      {models.map((m) => (
        <Badge key={m} variant="secondary" className="font-mono text-[12px] font-normal">
          {m}
        </Badge>
      ))}
    </div>
  );
}

ModelChips.propTypes = { models: PropTypes.arrayOf(PropTypes.string).isRequired };

function ListSheet({ open, onOpenChange, orgId, list, models, approved, onSaved }) {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [restricted, setRestricted] = useState(false);
  const [entries, setEntries] = useState([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName(list?.name || '');
    setDescription(list?.description || '');
    setRestricted(Boolean(list?.restricted));
    setEntries(list?.models || []);
  }, [open, list]);

  const submit = async () => {
    if (!name.trim()) {
      showError(t('modelAccess.lists.nameRequired'));
      return;
    }
    if (entries.length === 0) {
      showError(t('modelAccess.lists.modelsRequired'));
      return;
    }
    setSaving(true);
    try {
      const body = { name: name.trim(), description: description.trim(), restricted, models: entries };
      const res = list
        ? await API.put(`/api/org/${orgId}/model_access/lists/${list.id}`, body)
        : await API.post(`/api/org/${orgId}/model_access/lists`, body);
      if (res.data.success) {
        showSuccess(t('common.operationSuccess'));
        onSaved();
        onOpenChange(false);
      } else showError(res.data.message);
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
          <SheetTitle>{list ? t('modelAccess.lists.editTitle') : t('modelAccess.lists.newTitle')}</SheetTitle>
          <SheetDescription>{t('modelAccess.lists.sheetHint')}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField id="access-list-name" label={t('modelAccess.lists.name')} required>
            <Input id="access-list-name" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} />
          </FormField>
          <FormField id="access-list-description" label={t('modelAccess.lists.description')}>
            <Textarea
              id="access-list-description"
              rows={2}
              maxLength={500}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </FormField>
          <FormField id="access-list-models" label={t('modelAccess.lists.models')} hint={t('modelAccess.patternHint')} required>
            <ModelLimitSelector
              id="access-list-models"
              models={models}
              value={entries}
              onChange={setEntries}
              parents={approved.length ? [approved] : []}
            />
          </FormField>
          <div className="flex items-start justify-between gap-4 border-t border-border pt-4">
            <div className="space-y-0.5">
              <label htmlFor="access-list-restricted" className="text-sm font-medium">
                {t('modelAccess.lists.restricted')}
              </label>
              <p className="text-xs text-muted-foreground">{t('modelAccess.lists.restrictedHint')}</p>
            </div>
            <Switch id="access-list-restricted" checked={restricted} onCheckedChange={setRestricted} />
          </div>
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving}>
            {t('common.save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

ListSheet.propTypes = {
  open: PropTypes.bool.isRequired,
  onOpenChange: PropTypes.func.isRequired,
  orgId: PropTypes.number.isRequired,
  list: PropTypes.object,
  models: PropTypes.array.isRequired,
  approved: PropTypes.arrayOf(PropTypes.string).isRequired,
  onSaved: PropTypes.func.isRequired
};

function usageText(t, usage) {
  if (!usage) return t('modelAccess.lists.unused');
  const parts = [];
  if (usage.org_default) parts.push(t('modelAccess.lists.usedByDefault'));
  if (usage.members) parts.push(t('modelAccess.lists.usedByMembers', { count: usage.members }));
  if (usage.tokens) parts.push(t('modelAccess.lists.usedByKeys', { count: usage.tokens }));
  return parts.length ? parts.join(' · ') : t('modelAccess.lists.unused');
}

export default function ModelAccessPage({ orgId }) {
  const { t } = useTranslation();
  const [data, setData] = useState(null);
  const [models, setModels] = useState([]);
  const [access, setAccess] = useState(accessValueFrom(0, []));
  const [baseline, setBaseline] = useState(accessValueFrom(0, []));
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState(null); // null closed, {} new, list edit
  const [removing, setRemoving] = useState(null);

  const load = useCallback(async () => {
    try {
      const res = await API.get(`/api/org/${orgId}/model_access`);
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      const next = res.data.data;
      setData(next);
      const value = accessValueFrom(next.scope.default_list_id, next.scope.default_models);
      setAccess(value);
      setBaseline(value);
    } catch (error) {
      showError(error);
    }
  }, [orgId]);

  useEffect(() => {
    setData(null);
    load();
    API.get('/api/available_model')
      .then((res) => setModels(res.data.success ? toLimitModels(res.data.data) : []))
      .catch(() => setModels([]));
  }, [load]);

  const approved = useMemo(() => data?.scope?.approved_models || [], [data]);
  const lists = data?.lists || [];

  const saveDefault = async () => {
    setSaving(true);
    try {
      const { listId, models: entries } = accessPayload(access);
      const res = await API.put(`/api/org/${orgId}/model_access/default`, { model_access_list_id: listId, models: entries });
      if (res.data.success) {
        showSuccess(t('common.operationSuccess'));
        await load();
      } else showError(res.data.message);
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  const confirmRemove = async () => {
    try {
      const res = await API.delete(`/api/org/${orgId}/model_access/lists/${removing.id}`);
      if (res.data.success) {
        showSuccess(t('common.operationSuccess'));
        setRemoving(null);
        await load();
      } else showError(res.data.message);
    } catch (error) {
      showError(error);
    }
  };

  if (!data) return null;

  return (
    <div className="max-w-4xl space-y-6">
      <ActionCard title={t('modelAccess.approved.title')} description={t('modelAccess.approved.description')}>
        {approved.length ? (
          <ModelChips models={approved} />
        ) : (
          <p className="text-sm text-muted-foreground">{t('modelAccess.approved.none')}</p>
        )}
      </ActionCard>

      <FormCard
        title={t('modelAccess.default.title')}
        description={t('modelAccess.default.description')}
        hint={t('modelAccess.default.hint')}
        dirty={!sameAccess(access, baseline)}
        saving={saving}
        onSave={saveDefault}
        onReset={() => setAccess(baseline)}
      >
        <ModelAccessField
          id="org-default-models"
          value={access}
          onChange={setAccess}
          lists={lists}
          models={models}
          parents={approved.length ? [approved] : []}
          inheritLabel={t('modelAccess.mode.noLimit')}
          inheritHint={approved.length ? t('modelAccess.orgDefaultApproved') : t('modelAccess.orgDefaultNone')}
        />
      </FormCard>

      <ActionCard
        title={t('modelAccess.lists.title')}
        description={t('modelAccess.lists.intro')}
        aside={
          <Button size="sm" variant="outline" onClick={() => setEditing({})}>
            <Plus className="size-4" />
            {t('modelAccess.lists.new')}
          </Button>
        }
      >
        {lists.length === 0 ? (
          <p className="py-2 text-sm text-muted-foreground">{t('modelAccess.lists.empty')}</p>
        ) : (
          <ul className="divide-y divide-border">
            {lists.map((list) => (
              <li key={list.id} className="flex items-start gap-3 py-3">
                <div className="min-w-0 flex-1 space-y-1.5">
                  <div className="flex items-center gap-2">
                    <span className="truncate text-sm font-medium">{list.name}</span>
                    {list.restricted && (
                      <Badge variant="outline" className="gap-1 font-normal">
                        <Lock className="size-3" />
                        {t('modelAccess.lists.restrictedBadge')}
                      </Badge>
                    )}
                    <span className="text-xs text-muted-foreground">{usageText(t, list.usage)}</span>
                  </div>
                  {list.description && <p className="text-xs text-muted-foreground">{list.description}</p>}
                  <ModelChips models={list.models} />
                </div>
                <div className="flex shrink-0 gap-1">
                  <Button variant="ghost" size="icon" className="size-8" aria-label={t('common.edit')} onClick={() => setEditing(list)}>
                    <Pencil className="size-4" />
                  </Button>
                  <Button variant="ghost" size="icon" className="size-8" aria-label={t('common.delete')} onClick={() => setRemoving(list)}>
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </ActionCard>

      <ListSheet
        open={editing !== null}
        onOpenChange={(o) => !o && setEditing(null)}
        orgId={orgId}
        list={editing?.id ? editing : null}
        models={models}
        approved={approved}
        onSaved={load}
      />

      <Dialog open={!!removing} onOpenChange={(o) => !o && setRemoving(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('modelAccess.lists.deleteTitle', { name: removing?.name })}</DialogTitle>
            <DialogDescription>{t('modelAccess.lists.deleteBody', { usage: usageText(t, removing?.usage) })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoving(null)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmRemove}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

ModelAccessPage.propTypes = {
  orgId: PropTypes.number.isRequired
};
