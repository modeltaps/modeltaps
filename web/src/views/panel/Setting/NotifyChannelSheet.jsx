import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { KeyRound, Loader2, Send } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { FormField } from '@/components/ui/form-field';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { Checkbox } from '@/components/ui/checkbox';
import {
  NOTIFY_CHANNEL_TYPES,
  NOTIFY_KINDS,
  NotifyChannelIcon,
  channelDraftProblem,
  channelKinds,
  channelPayload,
  channelTypeOf
} from './notifyChannels';

const nk = (k) => `settingsUi.platform.notifyChannels.${k}`;

const emptyDraft = (type = 'email') => ({ id: 0, name: '', type, settings: {}, clear: [], enabled: true, kinds: [], sort: 0 });

// Creates or edits one notification channel. Secrets are never sent back: a stored one shows as
// a masked echo, stays when its input is left empty, and can be cleared when it is optional.
// "Send test" uses the form as it is, saved or not.
export default function NotifyChannelSheet({ open, onOpenChange, editing, onSaved }) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(emptyDraft());
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const isEdit = !!editing;
  const stored = editing?.secrets || {};
  const meta = channelTypeOf(draft.type);

  useEffect(() => {
    if (!open) return;
    setDraft(
      editing
        ? {
            id: editing.id,
            name: editing.name || '',
            type: editing.type,
            settings: { ...(editing.settings || {}) },
            clear: [],
            enabled: !!editing.enabled,
            kinds: channelKinds(editing),
            sort: editing.sort ?? 0
          }
        : emptyDraft()
    );
  }, [open, editing]);

  const setSetting = (key, value) => setDraft((d) => ({ ...d, settings: { ...d.settings, [key]: value } }));
  const toggleClear = (key) =>
    setDraft((d) => ({ ...d, clear: d.clear.includes(key) ? d.clear.filter((k) => k !== key) : [...d.clear, key] }));

  // Which notices the channel takes: all kinds, or only the ticked ones.
  const someKinds = draft.kinds.length > 0 || draft.pickKinds;
  const setKindMode = (some) => setDraft((d) => ({ ...d, kinds: some ? d.kinds : [], pickKinds: some }));
  const toggleKind = (kind, on) =>
    setDraft((d) => {
      const kinds = on ? [...d.kinds, kind] : d.kinds.filter((k) => k !== kind);
      return { ...d, kinds: NOTIFY_KINDS.filter((k) => kinds.includes(k)), pickKinds: true };
    });

  const problem = channelDraftProblem(draft, stored) || (draft.pickKinds && draft.kinds.length === 0 ? 'kinds' : null);
  const problemText =
    problem &&
    (problem === 'signed'
      ? t(nk('form.signedHint'))
      : problem === 'kinds'
        ? t(nk('form.kindsRequired'))
        : t(nk('form.required'), {
            field: t(nk(`fields.${problem}.label_${draft.type}`), { defaultValue: '' }) || t(nk(`fields.${problem}.label`))
          }));

  const send = async (path, method) => {
    const res = await API[method](path, channelPayload(draft));
    if (!res.data?.success) showError(res.data?.message);
    return !!res.data?.success;
  };

  const submit = async () => {
    if (saving || problem) return;
    setSaving(true);
    try {
      const ok = await send(isEdit ? `/api/notify_channel/${editing.id}` : '/api/notify_channel/', isEdit ? 'put' : 'post');
      if (ok) {
        toast.success(t(nk('saved')));
        onSaved?.();
      }
    } catch (e) {
      /* surfaced by the API interceptor */
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    if (testing || problem) return;
    setTesting(true);
    try {
      if (await send('/api/notify_channel/test', 'post')) toast.success(t(nk('testOk')));
    } catch (e) {
      /* the channel's own error is surfaced by the API interceptor */
    } finally {
      setTesting(false);
    }
  };

  const field = (f) => {
    const id = `notify-channel-${f.key}`;
    const cleared = draft.clear.includes(f.key);
    const masked = f.secret && stored[f.key] && !cleared ? stored[f.key] : '';
    const hintKey = nk(`fields.${f.key}.hint`);
    const typeHintKey = nk(`fields.${f.key}.hint_${draft.type}`);
    const hint = t(typeHintKey, { defaultValue: '' }) || t(hintKey, { defaultValue: '' });
    // A field key can mean different things per type (url: PushDeer's own server, the generic
    // webhook's target); `label_<type>` / `placeholder_<type>` override the shared text.
    const label = t(nk(`fields.${f.key}.label_${draft.type}`), { defaultValue: '' }) || t(nk(`fields.${f.key}.label`));
    const placeholder =
      t(nk(`fields.${f.key}.placeholder_${draft.type}`), { defaultValue: '' }) ||
      t(nk(`fields.${f.key}.placeholder`), { defaultValue: '' });
    return (
      <FormField key={f.key} id={id} label={label} required={f.required} hint={hint || undefined}>
        {(masked || cleared) && (
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <KeyRound className="size-3.5" />
            {cleared ? t(nk('form.cleared')) : t('common.secretConfigured', { masked })}
            {!f.required && (
              <button type="button" className="ml-1 text-info hover:underline" onClick={() => toggleClear(f.key)}>
                {cleared ? t(nk('form.undoClear')) : t(nk('form.clear'))}
              </button>
            )}
          </p>
        )}
        <Input
          id={id}
          type={f.secret ? 'password' : 'text'}
          autoComplete={f.secret ? 'new-password' : 'off'}
          value={draft.settings[f.key] || ''}
          disabled={cleared}
          placeholder={masked ? t(nk('form.secretKeep')) : placeholder}
          onChange={(e) => setSetting(f.key, e.target.value)}
        />
      </FormField>
    );
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{isEdit ? t('common.edit') : t(nk('add'))}</SheetTitle>
          <SheetDescription>{t(nk('sheetDescription'))}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField id="notify-channel-type" label={t(nk('form.type'))}>
            <div id="notify-channel-type" role="radiogroup" className="grid grid-cols-3 gap-2">
              {NOTIFY_CHANNEL_TYPES.filter((item) => !isEdit || item.type === draft.type).map((item) => {
                const current = item.type === draft.type;
                return (
                  <button
                    key={item.type}
                    type="button"
                    role="radio"
                    aria-checked={current}
                    disabled={isEdit}
                    onClick={() => setDraft((d) => ({ ...emptyDraft(item.type), name: d.name, enabled: d.enabled, kinds: d.kinds }))}
                    className={cn(
                      'flex items-center gap-2 rounded-md border px-3 py-2 text-left text-sm outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring/50',
                      current ? 'border-foreground bg-field font-medium ring-1 ring-foreground' : 'border-border hover:bg-field',
                      isEdit && 'cursor-default'
                    )}
                  >
                    <NotifyChannelIcon type={item.type} />
                    <span className="truncate">{t(nk(`types.${item.type}`))}</span>
                  </button>
                );
              })}
            </div>
          </FormField>

          <FormField id="notify-channel-name" label={t(nk('form.name'))} hint={t(nk('form.nameHint'))}>
            <Input
              id="notify-channel-name"
              value={draft.name}
              maxLength={64}
              placeholder={t(nk(`types.${draft.type}`))}
              onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))}
            />
          </FormField>

          {meta?.fields.map(field)}
          {meta?.signed && <p className="text-xs text-muted-foreground">{t(nk('form.signedHint'))}</p>}
          {draft.type === 'webhook' && <p className="text-xs leading-relaxed text-muted-foreground">{t(nk('form.webhookHint'))}</p>}

          <FormField id="notify-channel-kinds" label={t(nk('form.kinds'))}>
            <div id="notify-channel-kinds" role="radiogroup" className="grid grid-cols-2 gap-2">
              {[false, true].map((some) => (
                <button
                  key={String(some)}
                  type="button"
                  role="radio"
                  aria-checked={someKinds === some}
                  onClick={() => setKindMode(some)}
                  className={cn(
                    'rounded-md border px-3 py-2 text-left text-sm outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring/50',
                    someKinds === some ? 'border-foreground bg-field font-medium ring-1 ring-foreground' : 'border-border hover:bg-field'
                  )}
                >
                  {t(nk(some ? 'form.kindsSome' : 'form.kindsAll'))}
                </button>
              ))}
            </div>
            {someKinds && (
              <div className="grid gap-2 rounded-md border border-border p-3 sm:grid-cols-2">
                {NOTIFY_KINDS.map((kind) => (
                  <label key={kind} className="flex items-center gap-2 text-sm">
                    <Checkbox checked={draft.kinds.includes(kind)} onCheckedChange={(on) => toggleKind(kind, !!on)} />
                    {t(`setting_index.notifications.kind.${kind}`)}
                  </label>
                ))}
              </div>
            )}
          </FormField>

          <FormField id="notify-channel-enabled" label={t(nk('form.enabled'))} row>
            <div className="flex justify-end">
              <Switch
                id="notify-channel-enabled"
                checked={draft.enabled}
                onCheckedChange={(v) => setDraft((d) => ({ ...d, enabled: v }))}
              />
            </div>
          </FormField>
        </SheetBody>
        <SheetFooter>
          {problemText && <p className="mr-auto self-center text-xs text-muted-foreground">{problemText}</p>}
          <Button variant="outline" onClick={test} disabled={testing || !!problem}>
            {testing ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
            {t(nk('test'))}
          </Button>
          <Button onClick={submit} disabled={saving || !!problem}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

NotifyChannelSheet.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  editing: PropTypes.object,
  onSaved: PropTypes.func
};
