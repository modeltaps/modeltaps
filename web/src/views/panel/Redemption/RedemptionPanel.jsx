import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Copy, Download, MoreHorizontal, Ticket, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import QuotaInput from '@/components/QuotaInput';
import { Field, InfoRow, InfoRows, ObjectPanel, ObjectTile, Section } from '@/components/objects';
import { API } from 'utils/api';
import { renderQuota, timestamp2string } from 'utils/common';
import { SCOPES, inputToUnix, statusLabel, unixToInput } from './redemptionParts';

function Details({ code }) {
  const { t } = useTranslation();
  return (
    <InfoRows>
      <InfoRow label={t('redemptionPage.headLabels.status')}>{statusLabel(t, code.status)}</InfoRow>
      {code.status === 3 && <InfoRow label={t('redemptionPage.headLabels.quota')}>{renderQuota(code.quota)}</InfoRow>}
      <InfoRow label={t('redemptionPage.headLabels.createdTime')}>{timestamp2string(code.created_time)}</InfoRow>
      <InfoRow label={t('redemptionPage.headLabels.redeemedTime')}>
        {code.redeemed_time ? timestamp2string(code.redeemed_time) : t('redemptionPage.unredeemed')}
      </InfoRow>
      {code.status === 3 && (
        <InfoRow label={t('redemptionPage.headLabels.expiredTime')}>
          {code.expired_time > 0 ? timestamp2string(code.expired_time) : t('redemptionPage.neverExpire')}
        </InfoRow>
      )}
    </InfoRows>
  );
}

Details.propTypes = { code: PropTypes.object.isRequired };

function EditBody({ object: code, draft, setDraft }) {
  const { t } = useTranslation();
  const set = (key) => (value) => setDraft((d) => ({ ...d, [key]: value }));
  return (
    <>
      <Section>
        <Details code={code} />
      </Section>
      <Section title={t('redemptionPage.panel.edit')}>
        <Field label={t('redemptionPage.headLabels.name')} htmlFor="redemption-name">
          <Input id="redemption-name" value={draft.name} onChange={(e) => set('name')(e.target.value)} />
        </Field>
        <Field>
          <QuotaInput name="quota" label={t('redemptionPage.headLabels.quota')} value={draft.quota} onChange={set('quota')} />
        </Field>
        <Field label={t('redemptionPage.headLabels.scope')} htmlFor="redemption-scope" hint={t('redemptionPage.scopeHint')}>
          <Select value={draft.scope} onValueChange={set('scope')}>
            <SelectTrigger id="redemption-scope" className="w-64">
              <span className="truncate">{t(`redemptionPage.scopes.${draft.scope}`)}</span>
            </SelectTrigger>
            <SelectContent>
              {SCOPES.map((s) => (
                <SelectItem key={s} value={s}>
                  {t(`redemptionPage.scopes.${s}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field label={t('redemption_edit.expiredTime')} htmlFor="redemption-expired-time" hint={t('redemption_edit.expiredTimeHint')}>
          <Input
            id="redemption-expired-time"
            type="datetime-local"
            className="w-64"
            value={unixToInput(draft.expired_time)}
            onChange={(e) => set('expired_time')(inputToUnix(e.target.value))}
          />
        </Field>
      </Section>
    </>
  );
}

EditBody.propTypes = { object: PropTypes.object, draft: PropTypes.object, setDraft: PropTypes.func };

function ReadOnlyBody({ object: code }) {
  return (
    <Section>
      <Details code={code} />
    </Section>
  );
}

ReadOnlyBody.propTypes = { object: PropTypes.object };

const editTab = {
  key: 'details',
  make: (c) => ({ name: c.name || '', quota: c.quota ?? 0, scope: c.scope || 'any', expired_time: c.expired_time || 0 }),
  patch: (d, c, { t }) => {
    if (!d.name.trim()) return { error: t('validation.requiredName') };
    const quota = parseInt(d.quota, 10);
    if (!(quota >= 0)) return { error: t('redemption_edit.requiredQuota') };
    return { name: d.name.trim(), quota, scope: d.scope, expired_time: parseInt(d.expired_time, 10) || 0 };
  },
  Body: EditBody
};

const readOnlyTab = { key: 'details', Body: ReadOnlyBody };

// Right-side panel of one redemption code. Unused codes (enabled or disabled) are edited in one
// form saved at once; a redeemed code is read-only. The key, export and delete are in the header.
export default function RedemptionPanel({ codeId, onClose, onChanged, onToggleStatus, onCopyKey, onExportBatch, onDelete, reloadKey }) {
  const { t } = useTranslation();
  const [code, setCode] = useState(null);
  const [error, setError] = useState('');
  const editable = code?.status === 1 || code?.status === 2;
  const tabs = useMemo(() => [{ ...(editable ? editTab : readOnlyTab), label: t('redemptionPage.panel.details') }], [editable, t]);

  const load = useCallback(async () => {
    try {
      const res = await API.get(`/api/redemption/${codeId}`);
      const { success, message, data } = res.data;
      if (success) {
        setCode({ ...data, scope: data.scope || 'any' });
        setError('');
      } else setError(message || t('objects.notFound'));
    } catch (e) {
      setError(e.message);
    }
  }, [codeId, t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  return (
    <ObjectPanel
      titleId="redemption-panel-title"
      size="sm"
      object={code}
      error={error}
      icon={
        <ObjectTile>
          <Ticket />
        </ObjectTile>
      }
      title={code?.name}
      subtitle={code && `#${code.id} · ${code.key}`}
      subtitleMono
      actions={
        code && (
          <>
            {editable && (
              <Switch checked={code.status === 1} aria-label={t('common.enable')} onCheckedChange={() => onToggleStatus(code)} />
            )}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('objects.more')}>
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-52">
                <DropdownMenuItem onClick={() => onCopyKey(code)}>
                  <Copy className="size-4" />
                  {t('token_index.copy')}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => onExportBatch(code)}>
                  <Download className="size-4" />
                  {t('redemptionPage.exportBatch')}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem className="text-destructive" onClick={() => onDelete(code)}>
                  <Trash2 className="size-4" />
                  {t('common.delete')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )
      }
      tabs={tabs}
      tab="details"
      onTabChange={() => {}}
      onClose={onClose}
      onSave={async (patch) => {
        const res = await API.put('/api/redemption/', { ...code, ...patch, id: code.id });
        return { ...res.data, toast: res.data.success ? t('redemption_edit.editOk') : undefined };
      }}
      onSaved={async () => {
        await load();
        onChanged();
      }}
      ctx={{ t }}
      scope={t('redemptionPage.panel.scope')}
    />
  );
}

RedemptionPanel.propTypes = {
  codeId: PropTypes.number.isRequired,
  onClose: PropTypes.func.isRequired,
  onChanged: PropTypes.func.isRequired,
  onToggleStatus: PropTypes.func.isRequired,
  onCopyKey: PropTypes.func.isRequired,
  onExportBatch: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  reloadKey: PropTypes.number
};
