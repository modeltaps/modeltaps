import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { toast } from '@/components/ui/sonner';

const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);

// Draft of the open tab of an object panel. A tab is `{ key, make, patch, Body }`:
// `make(object, ctx)` builds the tab's draft, `patch(draft, object, ctx)` turns it into what to
// save (or `{ error }`). A tab without `patch` is read-only and never dirty.
//
// The draft is rebuilt whenever the tab or the stored object changes, except that a reload of
// the same tab keeps unsaved edits (a status toggle elsewhere must not wipe what is being
// typed). Leaving the page, closing the panel or switching tabs with unsaved changes asks first.
export default function useTabDrafts({ object, tab, ctx, onTabChange, onClose, onSave, onSaved }) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(null);
  // Tab the draft belongs to: right after a tab switch the old draft is still around for a render.
  const [draftTab, setDraftTab] = useState(null);
  const [base, setBase] = useState(null);
  const [version, setVersion] = useState(0);
  const [saving, setSaving] = useState(false);

  const resetTab = useRef(null);
  const dirtyRef = useRef(false);

  useEffect(() => {
    if (!object) return;
    if (resetTab.current === tab.key && dirtyRef.current) return;
    // Only the object and the tab key reset the draft (ctx and the tab definitions may be rebuilt
    // on every render): a list refresh must not wipe what is being typed.
    const d = tab.make ? tab.make(object, ctx) : {};
    resetTab.current = tab.key;
    setDraftTab(tab.key);
    setDraft(d);
    setBase(d);
    setVersion((v) => v + 1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [object, tab.key]);

  const ready = Boolean(object && draft && draftTab === tab.key);
  const dirty = Boolean(ready && tab.patch && base && !same(draft, base));
  useEffect(() => {
    dirtyRef.current = dirty;
  }, [dirty]);

  useEffect(() => {
    if (!dirty) return undefined;
    const onBeforeUnload = (e) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  const requestClose = () => {
    if (!dirty || window.confirm(t('objects.panel.discardConfirm'))) onClose();
  };
  const switchTab = (key) => {
    if (key === tab.key) return;
    if (!dirty || window.confirm(t('objects.panel.switchConfirm'))) onTabChange(key);
  };
  const discard = () => {
    setDraft(base);
    setVersion((v) => v + 1);
  };

  const save = async () => {
    const patch = tab.patch(draft, object, ctx);
    if (patch?.error) {
      toast.error(patch.error);
      return;
    }
    setSaving(true);
    try {
      const res = await onSave(patch, { tab: tab.key, draft });
      if (res?.success) {
        toast.success(res.toast || t('objects.panel.saved'));
        resetTab.current = null;
        await onSaved?.(res);
      } else if (res?.message) toast.error(res.message);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  return { ready, dirty, draft, setDraft, version, saving, requestClose, switchTab, discard, save };
}
