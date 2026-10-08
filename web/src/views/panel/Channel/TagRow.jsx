import PropTypes from 'prop-types';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown, ChevronRight, FlaskConical, Trash2 } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { toast } from '@/components/ui/sonner';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { manageChannel, fetchTagChannels, manageTag } from './channelApi';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { KeyMissingBadge, ModelCountChip, NumberCell, ResponseTime, StatusDot } from './channelColumns';

// Aggregated tag row. The backend channel list returns one representative row
// per tag (when filter_tag is 0/2); this component renders it as a tag-level
// row with enable/disable, priority editing and tag deletion, and expands into
// a sub-table of the tag's channels (`GET /api/channel_tag/:tag/list`).
// Ported from v1 `component/TableRow.jsx` tag branches + `TagTableRow.jsx`.
export default function TagRow({ item, colSpan, requestConfirm, onRefresh }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [channels, setChannels] = useState([]);

  const loadChannels = useCallback(async () => {
    setLoading(true);
    try {
      setChannels(await fetchTagChannels(item.tag));
    } catch (error) {
      toast.error(t('channel_row.getTagChannelsError', { message: error.message }));
    }
    setLoading(false);
  }, [item.tag, t]);

  useEffect(() => {
    if (open) loadChannels();
  }, [open, loadChannels]);

  const refreshAll = () => {
    onRefresh();
    if (open) loadChannels();
  };

  const { changeTagStatus, deleteTag } = useTagActions({ item, requestConfirm, onStatusChanged: refreshAll, onDeleted: onRefresh });

  const updateTagPriority = async (value) => {
    const priority = Number(value);
    if (Number.isNaN(priority)) return;
    if (priority < 0) {
      toast.error(t('channel_row.priorityTip'));
      return;
    }
    const { success, message } = await manageTag(item.tag, 'priority', priority);
    if (success) {
      toast.success(t('channel_row.priorityUpdateSuccess'));
      refreshAll();
    } else {
      toast.error(t('channel_row.priorityUpdateError', { message }));
    }
  };

  const toggleChannelStatus = async (channel) => {
    const next = channel.status === 1 ? 2 : 1;
    const { success, message } = await manageChannel(channel.id, 'status', next);
    if (success) {
      toast.success(t('userPage.operationSuccess'));
      setChannels((prev) => prev.map((c) => (c.id === channel.id ? { ...c, status: next } : c)));
    } else if (message) toast.error(message);
  };

  const updateChannelPriority = async (channel, value) => {
    const { success, message } = await manageChannel(channel.id, 'priority', value);
    success ? toast.success(t('userPage.operationSuccess')) : toast.error(message);
  };

  const testChannel = async (channel) => {
    const { success, time, message, model } = await manageChannel(channel.id, 'test', channel.test_model);
    if (success) {
      toast.success(t('channel_row.modelTestSuccess', { channel: channel.name, model, time: time.toFixed(2) }));
      loadChannels();
    } else if (message) toast.error(message);
  };

  const deleteChannel = (channel) => {
    requestConfirm(`${t('common.delete')} #${channel.id}`, '', async () => {
      const { success, message } = await manageChannel(channel.id, 'delete');
      if (success) {
        toast.success(t('userPage.operationSuccess'));
        refreshAll();
      } else if (message) toast.error(message);
    });
  };

  const models = useMemo(
    () =>
      (item.models || '')
        .split(',')
        .map((m) => m.trim())
        .filter(Boolean)
        .sort(),
    [item.models]
  );

  const copyModel = (model) => {
    try {
      navigator.clipboard.writeText(model);
      toast.success(`${model} ✓`);
    } catch (e) {
      toast.error(`${t('channel_edit.copyModels')}: ${model}`);
    }
  };

  const typeOpt = CHANNEL_OPTIONS[item.type];

  return (
    <>
      <TableRow className="bg-muted/30">
        <TableCell>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label={t('channel_row.tagChannelList')}
            onClick={() => setOpen((o) => !o)}
          >
            {open ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
          </Button>
        </TableCell>
        <TableCell>
          <Badge variant="secondary">{t('channel_row.tag')}</Badge>
        </TableCell>
        <TableCell>
          <div className="flex min-w-0 items-center gap-2.5">
            <ProviderIcon type={item.type} name={item.name} baseUrl={item.base_url} className="size-6 flex-none" />
            <div className="flex min-w-0 flex-col">
              <span className="max-w-[16rem] truncate font-medium" title={item.tag}>
                {item.tag}
              </span>
              <span className="text-xs text-muted-foreground">{typeOpt ? typeOpt.text : t('common.unknown')}</span>
            </div>
          </div>
        </TableCell>
        <TableCell>
          <div className="flex flex-wrap gap-1">
            {(item.group || '')
              .split(',')
              .filter(Boolean)
              .map((g) => (
                <Badge key={g} variant="secondary">
                  {g}
                </Badge>
              ))}
          </div>
        </TableCell>
        <TableCell>
          <ModelCountChip count={models.length} />
        </TableCell>
        <TableCell>
          <NumberCell value={item.priority} onCommit={updateTagPriority} />
        </TableCell>
        <TableCell>
          <span className="text-xs text-muted-foreground">-</span>
        </TableCell>
        <TableCell>
          <span className="text-xs text-muted-foreground">-</span>
        </TableCell>
        <TableCell>
          <span className="text-xs text-muted-foreground">-</span>
        </TableCell>
        <TableCell colSpan={2}>
          <div className="flex items-center gap-1">
            <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => changeTagStatus('enable')}>
              {t('channel_row.enable')}
            </Button>
            <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => changeTagStatus('disable')}>
              {t('channel_row.disable')}
            </Button>
          </div>
        </TableCell>
        <TableCell className="sticky right-0 bg-card z-10">
          <Button
            variant="ghost"
            size="icon"
            className="size-8 text-destructive hover:text-destructive"
            aria-label={t('channel_row.deleteTagAndChannels')}
            title={t('channel_row.deleteTagAndChannels')}
            onClick={deleteTag}
          >
            <Trash2 className="size-4" />
          </Button>
        </TableCell>
      </TableRow>

      {open && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={colSpan} className="bg-muted/20 p-0">
            <div className="px-4 py-3">
              {models.length > 0 && (
                <div className="mb-3 flex flex-wrap items-center gap-1.5">
                  <span className="text-sm font-medium text-muted-foreground">{t('channel_row.canModels')}</span>
                  {models.map((model) => (
                    <Badge
                      key={model}
                      variant="secondary"
                      className="cursor-pointer font-normal"
                      title={t('channel_edit.copyModels')}
                      onClick={() => copyModel(model)}
                    >
                      {model}
                    </Badge>
                  ))}
                </div>
              )}
              <div className="mb-2 text-sm font-medium text-muted-foreground">
                {t('channel_row.tagChannelList')} ({channels.length})
              </div>
              {loading ? (
                <div className="py-6 text-center text-sm text-muted-foreground">…</div>
              ) : channels.length === 0 ? (
                <div className="py-6 text-center text-sm text-muted-foreground">{t('channel_row.noTagChannels')}</div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>ID</TableHead>
                      <TableHead>{t('channel_index.name')}</TableHead>
                      <TableHead>{t('channel_index.status')}</TableHead>
                      <TableHead>{t('channel_index.responseTime')}</TableHead>
                      <TableHead>{t('channel_index.priority')}</TableHead>
                      <TableHead>{t('channel_index.actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {channels.map((channel) => (
                      <TableRow key={`tag-channel-${channel.id}`}>
                        <TableCell>
                          <span className="font-mono text-xs">{channel.id}</span>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <span className="font-medium">{channel.name}</span>
                            {channel.key_status?.configured && (
                              <span className="font-mono text-xs text-muted-foreground">{channel.key_status.masked.join(' ')}</span>
                            )}
                            <KeyMissingBadge t={t} keyStatus={channel.key_status} />
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <Switch checked={channel.status === 1} onCheckedChange={() => toggleChannelStatus(channel)} />
                            <StatusDot t={t} status={channel.status} />
                          </div>
                        </TableCell>
                        <TableCell>
                          <ResponseTime ms={channel.response_time} />
                        </TableCell>
                        <TableCell>
                          <NumberCell value={channel.priority} onCommit={(v) => updateChannelPriority(channel, v)} />
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-0.5">
                            <Button
                              variant="ghost"
                              size="icon"
                              className="size-8"
                              aria-label={t('channel_row.test')}
                              title={t('channel_row.test')}
                              onClick={() => testChannel(channel)}
                            >
                              <FlaskConical className="size-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="size-8 text-destructive hover:text-destructive"
                              aria-label={t('common.delete')}
                              title={t('common.delete')}
                              onClick={() => deleteChannel(channel)}
                            >
                              <Trash2 className="size-4" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>
          </TableCell>
        </TableRow>
      )}
    </>
  );
}

TagRow.propTypes = {
  item: PropTypes.object,
  colSpan: PropTypes.number,
  requestConfirm: PropTypes.func,
  onRefresh: PropTypes.func
};

// 标签级启用 / 禁用 / 删除(表格行与窄屏卡片共用)。
function useTagActions({ item, requestConfirm, onStatusChanged, onDeleted }) {
  const { t } = useTranslation();

  const changeTagStatus = (action) => {
    const actionLabel = t(action === 'enable' ? 'channel_row.enable' : 'channel_row.disable');
    requestConfirm(
      t(action === 'enable' ? 'channel_row.enableTagChannels' : 'channel_row.disableTagChannels'),
      t('channel_row.tagChannelsConfirm', { action: actionLabel, tag: item.tag }),
      async () => {
        const { success, message } = await manageTag(item.tag, 'status', action);
        if (success) {
          toast.success(t('channel_row.tagChannelsSuccess', { action: actionLabel }));
          onStatusChanged();
        } else {
          toast.error(t('channel_row.tagChannelsError', { action: actionLabel, message }));
        }
      }
    );
  };

  const deleteTag = () => {
    requestConfirm(t('channel_row.deleteTag'), t('channel_row.deleteTagConfirm', { tag: item.tag }), async () => {
      const { success, message } = await manageTag(item.tag, 'delete');
      if (success) {
        toast.success(t('channel_row.deleteTagSuccess', { tag: item.tag }));
        onDeleted();
      } else {
        toast.error(t('channel_row.deleteTagError', { message }));
      }
    });
  };

  return { changeTagStatus, deleteTag };
}

// 窄屏标签卡片:标签名 + 分组 / 模型数,标签级启用 / 禁用 / 删除;成员渠道在桌面端展开管理。
export function TagCard({ item, requestConfirm, onRefresh }) {
  const { t } = useTranslation();
  const { changeTagStatus, deleteTag } = useTagActions({ item, requestConfirm, onStatusChanged: onRefresh, onDeleted: onRefresh });
  const typeOpt = CHANNEL_OPTIONS[item.type];
  const groups = (item.group || '').split(',').filter(Boolean);
  const modelCount = (item.models || '').split(',').filter((m) => m.trim()).length;
  return (
    <>
      <div className="flex items-center gap-2">
        <ProviderIcon type={item.type} name={item.name} baseUrl={item.base_url} className="size-6 flex-none" />
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="truncate font-medium" title={item.tag}>
            {item.tag}
          </span>
          <span className="text-xs text-muted-foreground">{typeOpt ? typeOpt.text : t('common.unknown')}</span>
        </div>
        <Badge variant="secondary">{t('channel_row.tag')}</Badge>
        <Button
          variant="ghost"
          size="icon"
          className="size-8 text-destructive hover:text-destructive"
          aria-label={t('channel_row.deleteTagAndChannels')}
          title={t('channel_row.deleteTagAndChannels')}
          onClick={deleteTag}
        >
          <Trash2 className="size-4" />
        </Button>
      </div>
      <dl className="mt-2 grid grid-cols-[6.5rem_minmax(0,1fr)] items-center gap-x-3 gap-y-1.5">
        <dt className="text-xs font-medium text-muted-foreground">{t('channel_index.group')}</dt>
        <dd className="flex min-w-0 flex-wrap items-center gap-1 text-sm">
          {groups.map((g) => (
            <Badge key={g} variant="secondary">
              {g}
            </Badge>
          ))}
        </dd>
        <dt className="text-xs font-medium text-muted-foreground">{t('channel_index.model')}</dt>
        <dd className="flex min-w-0 items-center text-sm">
          <ModelCountChip count={modelCount} />
        </dd>
        <dt className="text-xs font-medium text-muted-foreground">{t('channel_index.status')}</dt>
        <dd className="flex min-w-0 items-center gap-1 text-sm">
          <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => changeTagStatus('enable')}>
            {t('channel_row.enable')}
          </Button>
          <Button variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => changeTagStatus('disable')}>
            {t('channel_row.disable')}
          </Button>
        </dd>
      </dl>
    </>
  );
}

TagCard.propTypes = {
  item: PropTypes.object.isRequired,
  requestConfirm: PropTypes.func.isRequired,
  onRefresh: PropTypes.func.isRequired
};
