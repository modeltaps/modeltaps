import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { showError } from 'utils/common';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { FormField } from '@/components/ui/form-field';

// ==============================|| PLATFORM SETTINGS — CHAT LINK SHEET ||============================== //
// Edits one entry of the ChatLinks option ({ id, name, url, show, sort }); `onSave` receives the
// entry and writes the whole option back, so the backend contract is unchanged.

const EMPTY = { name: '', url: '', show: true, sort: '0' };

export function parseChatLinks(value) {
  if (!value || !String(value).trim()) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed.filter((it) => it && typeof it === 'object') : [];
  } catch (e) {
    return [];
  }
}

export const serializeChatLinks = (links) => JSON.stringify(links, null, 2);

export default function ChatLinkSheet({ open, editing, onOpenChange, onSave }) {
  const { t } = useTranslation();
  const ed = (k) => t(`setting_index.operationSettings.chatLinkSettings.editor.${k}`);
  const [draft, setDraft] = useState(EMPTY);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setDraft(
      editing
        ? {
            name: editing.name ?? '',
            url: editing.url ?? '',
            show: editing.show !== false,
            sort: editing.sort === '' || editing.sort == null ? '' : String(editing.sort)
          }
        : EMPTY
    );
  }, [open, editing]);

  const set = (patch) => setDraft((d) => ({ ...d, ...patch }));

  const submit = async () => {
    if (!draft.name) return showError(ed('nameRequired'));
    if (!draft.url) return showError(ed('urlRequired'));
    if (draft.sort !== '' && !/^[0-9]\d*$/.test(String(draft.sort))) return showError(ed('sortInvalid'));
    setSaving(true);
    try {
      const ok = await onSave({
        ...(editing || {}),
        name: draft.name,
        url: draft.url,
        show: !!draft.show,
        sort: draft.sort === '' ? '' : Number(draft.sort)
      });
      if (ok) onOpenChange(false);
    } finally {
      setSaving(false);
    }
    return undefined;
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-md" onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{editing ? t('settingsUi.platform.chatLinks.edit') : t('settingsUi.platform.chatLinks.add')}</SheetTitle>
          <SheetDescription>{t('setting_index.operationSettings.chatLinkSettings.description')}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField id="chat-link-name" label={ed('name')} required>
            <Input id="chat-link-name" value={draft.name} onChange={(e) => set({ name: e.target.value })} />
          </FormField>
          <FormField id="chat-link-url" label={ed('url')} required>
            <Input id="chat-link-url" className="font-mono" value={draft.url} onChange={(e) => set({ url: e.target.value })} />
          </FormField>
          <FormField id="chat-link-sort" label={ed('sort')} hint={t('setting_index.operationSettings.chatLinkSettings.help.line3')}>
            <Input id="chat-link-sort" type="number" min="0" value={draft.sort} onChange={(e) => set({ sort: e.target.value })} />
          </FormField>
          <div className="flex items-center justify-between gap-4">
            <Label htmlFor="chat-link-show">{ed('show')}</Label>
            <Switch id="chat-link-show" checked={draft.show} onCheckedChange={(v) => set({ show: v })} />
          </div>
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

ChatLinkSheet.propTypes = {
  open: PropTypes.bool,
  editing: PropTypes.object,
  onOpenChange: PropTypes.func.isRequired,
  onSave: PropTypes.func.isRequired
};
