import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { API } from 'utils/api';
import { toast } from '@/components/ui/sonner';
import ChannelCheckDialog from './ChannelCheckDialog';
import ChannelTestDialog from './ChannelTestDialog';
import ConfirmDialog from './ConfirmDialog';
import { manageChannel, checkChannelDrift, dismissChannelNewModels } from './channelApi';

const addId = (set, id) => new Set(set).add(id);
const dropId = (set, id) => {
  const next = new Set(set);
  next.delete(id);
  return next;
};

// Channel actions shared by the list and the detail page: test, multi-model test, check,
// drift check, dismiss new models, balance refresh, copy, delete, status / priority / weight.
// `onChanged()` runs after anything that changes stored data; `onDriftDismissed(row, drift)`
// gets the fresh drift result; `onDeleted(row)` runs after a delete. The returned `dialogs`
// node renders the check / test / confirm dialogs and must be mounted by the caller.
export default function useChannelActions({ onChanged, onDriftDismissed, onDeleted } = {}) {
  const { t } = useTranslation();
  const [testingIds, setTestingIds] = useState(() => new Set());
  const [driftCheckingIds, setDriftCheckingIds] = useState(() => new Set());
  const [dismissingIds, setDismissingIds] = useState(() => new Set());
  const [refreshingIds, setRefreshingIds] = useState(() => new Set());
  const [checkItem, setCheckItem] = useState(null);
  const [testItem, setTestItem] = useState(null);
  const [confirm, setConfirm] = useState({ open: false, title: '', content: '', onConfirm: null });

  const changed = () => onChanged?.();
  const requestConfirm = (title, content, fn) => setConfirm({ open: true, title, content, onConfirm: fn });

  const handlers = {
    onToggleStatus: async (row) => {
      const next = row.status === 1 ? 2 : 1;
      const { success, message } = await manageChannel(row.id, 'status', next);
      if (success) {
        toast.success(t('userPage.operationSuccess'));
        changed();
      } else if (message) toast.error(message);
    },
    onUpdatePriority: async (row, value) => {
      const { success, message } = await manageChannel(row.id, 'priority', value);
      success ? toast.success(t('userPage.operationSuccess')) : toast.error(message);
    },
    onUpdateWeight: async (row, value) => {
      const { success, message } = await manageChannel(row.id, 'weight', value);
      success ? toast.success(t('userPage.operationSuccess')) : toast.error(message);
    },
    onCheck: (row) => setCheckItem(row),
    onMultiTest: (row) => setTestItem(row),
    onCheckDrift: async (row) => {
      if (driftCheckingIds.has(row.id)) return;
      setDriftCheckingIds((prev) => addId(prev, row.id));
      try {
        const { success, message, data } = await checkChannelDrift(row.id);
        if (success) {
          const count = data?.missing_models?.length ?? 0;
          count > 0
            ? toast.warning(t('channel_row.driftFound', { channel: row.name, count }))
            : toast.success(t('channel_row.driftNone', { channel: row.name }));
          changed();
        } else if (message) toast.error(message);
      } catch (e) {
        toast.error(e.message);
      } finally {
        setDriftCheckingIds((prev) => dropId(prev, row.id));
      }
    },
    onDismissNewModels: async (row) => {
      if (dismissingIds.has(row.id)) return;
      setDismissingIds((prev) => addId(prev, row.id));
      try {
        const { success, message, data } = await dismissChannelNewModels(row.id);
        if (success) {
          onDriftDismissed?.(row, data);
          toast.success(t('channel_row.newModelsDismissed', { channel: row.name }));
        } else if (message) toast.error(message);
      } catch (e) {
        toast.error(e.message);
      } finally {
        setDismissingIds((prev) => dropId(prev, row.id));
      }
    },
    onTest: async (row) => {
      if (testingIds.has(row.id)) return;
      setTestingIds((prev) => addId(prev, row.id));
      try {
        const { success, time, message, model } = await manageChannel(row.id, 'test', row.test_model);
        if (success) {
          toast.success(t('channel_row.modelTestSuccess', { channel: row.name, model, time: time.toFixed(2) }));
          changed();
        } else if (message) toast.error(message);
      } finally {
        setTestingIds((prev) => dropId(prev, row.id));
      }
    },
    onRefreshBalance: async (row) => {
      if (refreshingIds.has(row.id)) return;
      setRefreshingIds((prev) => addId(prev, row.id));
      try {
        const res = await API.get(`/api/channel/update_balance/${row.id}`);
        const { success, message } = res.data;
        if (success) {
          toast.success(t('channel_row.updateOk'));
          changed();
        } else toast.error(message);
      } catch (e) {
        toast.error(e.message);
      } finally {
        setRefreshingIds((prev) => dropId(prev, row.id));
      }
    },
    onCopy: async (row) => {
      const { success, message } = await manageChannel(row.id, 'copy');
      if (success) {
        toast.success(t('userPage.operationSuccess'));
        changed();
      } else if (message) toast.error(message);
    },
    onDelete: (row) => {
      requestConfirm(`${t('common.delete')} #${row.id}`, '', async () => {
        const { success, message } = await manageChannel(row.id, 'delete');
        if (success) {
          toast.success(t('userPage.operationSuccess'));
          onDeleted ? onDeleted(row) : changed();
        } else if (message) toast.error(message);
      });
    }
  };

  const dialogs = (
    <>
      <ChannelCheckDialog open={!!checkItem} item={checkItem} onClose={() => setCheckItem(null)} />
      <ChannelTestDialog open={!!testItem} item={testItem} onClose={() => setTestItem(null)} onFinished={changed} />
      <ConfirmDialog
        open={confirm.open}
        title={confirm.title}
        content={confirm.content}
        onCancel={() => setConfirm((c) => ({ ...c, open: false }))}
        onConfirm={() => {
          confirm.onConfirm?.();
          setConfirm((c) => ({ ...c, open: false }));
        }}
      />
    </>
  );

  return { handlers, testingIds, driftCheckingIds, dismissingIds, refreshingIds, requestConfirm, dialogs };
}
