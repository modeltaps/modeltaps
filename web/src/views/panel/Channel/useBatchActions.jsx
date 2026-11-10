import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/ui/sonner';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { manageChannel, manageTag, updateChannel } from './channelApi';
import { splitList } from './channelGroups';
import { GroupOptions } from './channelParts';

// Runs `fn` for every row and returns how many failed (a rejected call or success: false).
async function eachRow(rows, fn) {
  const results = await Promise.allSettled(rows.map(fn));
  return results.filter((r) => r.status !== 'fulfilled' || !r.value?.success).length;
}

function FormDialog({ open, title, description, busy, onCancel, onConfirm, confirmLabel, children }) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={(o) => !o && !busy && onCancel()}>
      <DialogContent className="max-w-lg rounded-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        <DialogBody>{children}</DialogBody>
        <DialogFooter>
          <Button variant="outline" className="rounded-lg" disabled={busy} onClick={onCancel}>
            {t('common.cancel')}
          </Button>
          <Button className="rounded-lg" disabled={busy} onClick={onConfirm}>
            {busy && <Loader2 className="animate-spin" />}
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

FormDialog.propTypes = {
  open: PropTypes.bool,
  title: PropTypes.node,
  description: PropTypes.node,
  busy: PropTypes.bool,
  onCancel: PropTypes.func.isRequired,
  onConfirm: PropTypes.func.isRequired,
  confirmLabel: PropTypes.node,
  children: PropTypes.node
};

// Batch actions on selected channels and tag-wide actions on a tag. Per-channel calls go
// through manageChannel / updateChannel (the backend has no batch endpoint for status, test,
// priority, weight or replacing groups); deleting uses the batch delete endpoint and tag
// actions the tag endpoints. `onDone()` runs after anything that changed data; `dialogs` must
// be mounted by the caller.
export default function useBatchActions({ groups, requestConfirm, onDone }) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [groupsDialog, setGroupsDialog] = useState(null); // { rows, value }
  const [numbersDialog, setNumbersDialog] = useState(null); // { rows, priority, weight }
  const [tagDialog, setTagDialog] = useState(null); // { tag, priority }

  const run = async (fn) => {
    setBusy(true);
    try {
      await fn();
    } catch (e) {
      toast.error(e.message);
    } finally {
      setBusy(false);
    }
  };
  const report = (failed, count) => {
    if (failed) toast.warning(t('channels.batch.partial', { failed, count }));
    else toast.success(t('channels.batch.done', { count }));
  };

  const setStatus = (rows, status) =>
    run(async () => {
      report(await eachRow(rows, (c) => manageChannel(c.id, 'status', status)), rows.length);
      onDone();
    });

  const test = (rows) =>
    run(async () => {
      const failed = await eachRow(rows, (c) => manageChannel(c.id, 'test', c.test_model));
      failed
        ? toast.warning(t('channels.pane.testPartial', { failed, count: rows.length }))
        : toast.success(t('channels.pane.testDone', { count: rows.length }));
      onDone();
    });

  const remove = (rows) =>
    requestConfirm(t('channels.batch.deleteTitle', { count: rows.length }), t('channels.batch.deleteConfirm', { count: rows.length }), () =>
      run(async () => {
        const { success, message } = await manageChannel(
          null,
          'batch_delete',
          rows.map((c) => c.id)
        );
        if (success) toast.success(t('channel_index.batchDeleteChannelsSuccess', { count: rows.length }));
        else toast.error(message);
        onDone(true);
      })
    );

  const openGroups = (rows) => {
    const shared = rows.map((c) => splitList(c.group)).reduce((a, g) => a.filter((x) => g.includes(x)));
    setGroupsDialog({ rows, value: shared.length ? shared : ['default'] });
  };
  const saveGroups = () =>
    run(async () => {
      const { rows, value } = groupsDialog;
      report(await eachRow(rows, (c) => updateChannel(c, { group: value.join(',') })), rows.length);
      setGroupsDialog(null);
      onDone();
    });

  const openNumbers = (rows) => setNumbersDialog({ rows, priority: '', weight: '' });
  const saveNumbers = () => {
    const { rows, priority, weight } = numbersDialog;
    const patch = {};
    if (String(priority).trim() !== '') patch.priority = Number(priority) || 0;
    if (String(weight).trim() !== '') patch.weight = Math.max(1, Number(weight) || 1);
    if (!Object.keys(patch).length) {
      setNumbersDialog(null);
      return;
    }
    run(async () => {
      report(await eachRow(rows, (c) => updateChannel(c, patch)), rows.length);
      setNumbersDialog(null);
      onDone();
    });
  };

  // Tag-wide actions, as the former tag rows did them.
  const tagStatus = (tag, action) => {
    const actionLabel = t(action === 'enable' ? 'channel_row.enable' : 'channel_row.disable');
    requestConfirm(
      t(action === 'enable' ? 'channel_row.enableTagChannels' : 'channel_row.disableTagChannels'),
      t('channel_row.tagChannelsConfirm', { action: actionLabel, tag }),
      () =>
        run(async () => {
          const { success, message } = await manageTag(tag, 'status', action);
          if (success) toast.success(t('channel_row.tagChannelsSuccess', { action: actionLabel }));
          else toast.error(t('channel_row.tagChannelsError', { action: actionLabel, message }));
          onDone();
        })
    );
  };
  const tagDelete = (tag, onDeleted) =>
    requestConfirm(t('channel_row.deleteTag'), t('channel_row.deleteTagConfirm', { tag }), () =>
      run(async () => {
        const { success, message } = await manageTag(tag, 'delete');
        if (success) {
          toast.success(t('channel_row.deleteTagSuccess', { tag }));
          onDeleted?.();
        } else toast.error(t('channel_row.deleteTagError', { message }));
        onDone(true);
      })
    );
  const openTagPriority = (tag) => setTagDialog({ tag, priority: '' });
  const saveTagPriority = () => {
    const priority = Number(tagDialog.priority);
    if (String(tagDialog.priority).trim() === '' || Number.isNaN(priority)) return;
    if (priority < 0) {
      toast.error(t('channel_row.priorityTip'));
      return;
    }
    run(async () => {
      const { success, message } = await manageTag(tagDialog.tag, 'priority', priority);
      if (success) {
        toast.success(t('channel_row.priorityUpdateSuccess'));
        setTagDialog(null);
      } else toast.error(t('channel_row.priorityUpdateError', { message }));
      onDone();
    });
  };

  const dialogs = (
    <>
      <FormDialog
        open={!!groupsDialog}
        title={t('channels.batch.groupsTitle', { count: groupsDialog?.rows.length || 0 })}
        description={t('channels.batch.groupsHint')}
        busy={busy}
        confirmLabel={t('channels.panel.save')}
        onCancel={() => setGroupsDialog(null)}
        onConfirm={saveGroups}
      >
        {groupsDialog && (
          <GroupOptions groups={groups} value={groupsDialog.value} onChange={(value) => setGroupsDialog({ ...groupsDialog, value })} />
        )}
      </FormDialog>
      <FormDialog
        open={!!numbersDialog}
        title={t('channels.batch.numbersTitle', { count: numbersDialog?.rows.length || 0 })}
        description={t('channels.batch.numbersHint')}
        busy={busy}
        confirmLabel={t('channels.panel.save')}
        onCancel={() => setNumbersDialog(null)}
        onConfirm={saveNumbers}
      >
        {numbersDialog && (
          <div className="grid grid-cols-2 gap-3">
            {['priority', 'weight'].map((k) => (
              <label key={k} className="space-y-1.5 text-xs text-muted-foreground">
                <span>{t(`channels.usage.${k}`)}</span>
                <Input
                  type="number"
                  min={k === 'weight' ? 1 : undefined}
                  className="tabular-nums text-foreground"
                  value={numbersDialog[k]}
                  placeholder={t('channels.batch.unchanged')}
                  onChange={(e) => setNumbersDialog({ ...numbersDialog, [k]: e.target.value })}
                />
              </label>
            ))}
          </div>
        )}
      </FormDialog>
      <FormDialog
        open={!!tagDialog}
        title={t('channels.tagActions.priorityTitle', { tag: tagDialog?.tag || '' })}
        description={t('channels.tagActions.priorityHint')}
        busy={busy}
        confirmLabel={t('channels.panel.save')}
        onCancel={() => setTagDialog(null)}
        onConfirm={saveTagPriority}
      >
        {tagDialog && (
          <Input
            type="number"
            min="0"
            autoFocus
            className="w-40 tabular-nums"
            value={tagDialog.priority}
            onChange={(e) => setTagDialog({ ...tagDialog, priority: e.target.value })}
          />
        )}
      </FormDialog>
    </>
  );

  return {
    busy,
    batch: {
      enable: (rows) => setStatus(rows, 1),
      disable: (rows) => setStatus(rows, 2),
      test,
      remove,
      groups: openGroups,
      numbers: openNumbers
    },
    tag: {
      enable: (tag) => tagStatus(tag, 'enable'),
      disable: (tag) => tagStatus(tag, 'disable'),
      priority: openTagPriority,
      remove: tagDelete
    },
    dialogs
  };
}
