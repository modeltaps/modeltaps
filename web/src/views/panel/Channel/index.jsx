import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { Plus, RotateCcw, FlaskConical, DollarSign, Trash2, Layers, AlertTriangle, MoreVertical, Power, PowerOff } from 'lucide-react';

import { API } from 'utils/api';
import { getPageSize, savePageSize, getTableSort, saveTableSort } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import { Button } from '@/components/ui/button';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { toast } from '@/components/ui/sonner';
import { filterStateToParams, paramsToFilterState } from '@/components/filter-bar';
import { BatchBar, ListEmptyState, ListFooter, ListPage, ListToolbar, StatChip } from '@/components/list-page';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { getColumns } from './channelColumns';
import { buildChannelFilterFields, CHANNEL_FILTER_PARAM_KEYS } from './channelFilterFields';
import ChannelSheet from './ChannelSheet';
import ChannelCheckDialog from './ChannelCheckDialog';
import ConfirmDialog from './ConfirmDialog';
import TagRow, { TagCard } from './TagRow';
import BatchModal from './BatchModal';
import {
  fetchChannelData,
  manageChannel,
  fetchGroups,
  fetchTags,
  fetchModels,
  checkChannelDrift,
  dismissChannelNewModels,
  fetchChannelStatusCounts,
  CHANNEL_STATUS_CHIPS
} from './channelApi';

// 参数映射只依赖字段的 key/type/single/paramInclude(与选项无关),用静态描述避免
// 动态选项(分组/标签异步加载)改变 fetch 依赖而多次拉列表;动态字段仅供 FilterBar 渲染。
const CHANNEL_PARAM_FIELDS = buildChannelFilterFields({});

// 单选分面的 filterState 项(与 FilterBar 写入的形状一致)。
const enumFilterEntry = (key, value) => paramsToFilterState(CHANNEL_PARAM_FIELDS, new URLSearchParams({ [key]: value }))[key];

const STATUS_CHIP_STYLE = {
  enabled: { labelKey: 'channelPage.stats.enabled', tone: 'success' },
  manual: { labelKey: 'channelPage.stats.manual', tone: 'default' },
  auto: { labelKey: 'channelPage.stats.auto', tone: 'destructive' }
};

// 页头状态 chip:取值即 status 分面(1 启用 / 2 手动禁用 / 3 自动禁用)。
const STATUS_CHIPS = CHANNEL_STATUS_CHIPS.map((c) => ({ ...c, ...STATUS_CHIP_STYLE[c.id] }));

const SEARCH_DEBOUNCE_MS = 300;

export default function Channel() {
  const { t } = useTranslation();
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('channel'));
  const [order, setOrder] = useState(() => getTableSort('channel').order);
  const [orderBy, setOrderBy] = useState(() => getTableSort('channel').orderBy);
  const [listCount, setListCount] = useState(0);
  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(false);
  const [refreshFlag, setRefreshFlag] = useState(false);
  const [testingIds, setTestingIds] = useState(() => new Set());
  const [driftCheckingIds, setDriftCheckingIds] = useState(() => new Set());
  const [dismissingIds, setDismissingIds] = useState(() => new Set());
  const [driftCheckingAll, setDriftCheckingAll] = useState(false);

  const [searchParams, setSearchParams] = useSearchParams();
  const [groupOptions, setGroupOptions] = useState([]);
  const [modelOptions, setModelOptions] = useState([]);
  const [tags, setTags] = useState([]);

  // FilterBar 字段(text + 单选 enum);选项随分组/标签动态刷新(仅用于渲染)。
  const filterFields = useMemo(() => buildChannelFilterFields({ groupOptions, tags }), [groupOptions, tags]);
  const [filterState, setFilterState] = useState(() => paramsToFilterState(CHANNEL_PARAM_FIELDS, searchParams));
  // 搜索框对应后端 name 参数(名称 / 标签 LIKE);输入去抖后才参与取数。
  const [keyword, setKeyword] = useState(() => searchParams.get('name') || '');
  const [appliedKeyword, setAppliedKeyword] = useState(keyword);
  const [statusCounts, setStatusCounts] = useState({});
  const [batchBusy, setBatchBusy] = useState(false);

  const [rowSelection, setRowSelection] = useState({});
  // ?channel_id=<id>(如模型页渠道图标跳转)直接打开该渠道的编辑抽屉;参数随下方 URL 同步清除,刷新不重复弹出。
  const [linkedChannelId] = useState(() => parseInt(searchParams.get('channel_id'), 10) || 0);
  const [sheetOpen, setSheetOpen] = useState(linkedChannelId > 0);
  const [sheetTab, setSheetTab] = useState('basic');
  const [batchOpen, setBatchOpen] = useState(false);
  const [checkItem, setCheckItem] = useState(null);
  const [editId, setEditId] = useState(linkedChannelId);
  const [confirm, setConfirm] = useState({ open: false, title: '', content: '', onConfirm: null });

  const queryParams = useMemo(() => {
    const q = filterStateToParams(CHANNEL_PARAM_FIELDS, filterState);
    const name = appliedKeyword.trim();
    return name ? { ...q, name } : q;
  }, [filterState, appliedKeyword]);

  // reqId 守卫(UX-13):快速切页/切筛选时丢弃过期响应;channelApi.js 结构保留,不迁 usePaginatedList。
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  const fetchData = useCallback(
    () =>
      runGuardedFetch(guardRef.current, () => fetchChannelData(page, rowsPerPage, queryParams, order, orderBy), {
        onStart: () => setLoading(true),
        onResult: (data) => {
          if (data) {
            setListCount(data.total_count);
            setChannels(data.data || []);
          }
        },
        onFinally: () => setLoading(false)
      }),
    [page, rowsPerPage, queryParams, order, orderBy]
  );

  useEffect(() => {
    fetchData();
  }, [fetchData, refreshFlag]);

  // 计数沿用当前筛选(仅替换 status),保证每个 chip 的数字等于点击后的结果条数。
  useEffect(() => {
    let stale = false;
    fetchChannelStatusCounts(queryParams)
      .then((c) => !stale && setStatusCounts(c))
      .catch(() => {});
    return () => {
      stale = true;
    };
  }, [queryParams, refreshFlag]);

  useEffect(() => {
    if (keyword === appliedKeyword) return undefined;
    const timer = setTimeout(() => {
      setPage(0);
      setAppliedKeyword(keyword);
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [keyword, appliedKeyword]);

  useEffect(() => {
    fetchGroups().then(setGroupOptions);
    fetchTags().then(setTags);
    fetchModels().then(setModelOptions);
  }, []);

  // 已应用筛选同步到 URL(replace,刷新可恢复;先清空筛选键再按当前 state 重写)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        params.delete('channel_id');
        [...CHANNEL_FILTER_PARAM_KEYS, 'name'].forEach((k) => params.delete(k));
        for (const [k, v] of Object.entries(queryParams)) params.set(k, String(v));
        return params;
      },
      { replace: true }
    );
  }, [queryParams, setSearchParams]);

  const doRefresh = (reset) => {
    if (reset) {
      setOrder('desc');
      setOrderBy('id');
      saveTableSort('channel', 'desc', 'id');
      setFilterState({});
      setKeyword('');
      setAppliedKeyword('');
      setPage(0);
    }
    setRefreshFlag((f) => !f);
  };

  // 级联筛选受控回调:任何改动即时应用并回到第一页;清空同理。
  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
    setKeyword('');
    setAppliedKeyword('');
  };

  // 页头状态 chip:点击等价于设置「状态」分面(「全部」清除该分面)。
  const applyStatusChip = (value) => {
    setPage(0);
    setFilterState((prev) => {
      const next = { ...prev };
      if (value) next.status = enumFilterEntry('status', value);
      else delete next.status;
      return next;
    });
  };

  const onSortClick = (colId) => {
    const isAsc = orderBy === colId && order === 'asc';
    const newOrder = isAsc ? 'desc' : 'asc';
    setOrder(newOrder);
    setOrderBy(colId);
    saveTableSort('channel', newOrder, colId);
    setPage(0);
  };

  const handlers = {
    onEdit: (row, tab = 'basic') => {
      setEditId(row.id);
      setSheetTab(tab);
      setSheetOpen(true);
    },
    onToggleStatus: async (row) => {
      const next = row.status === 1 ? 2 : 1;
      const { success, message } = await manageChannel(row.id, 'status', next);
      if (success) {
        toast.success(t('userPage.operationSuccess'));
        setRefreshFlag((f) => !f);
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
    onCheckDrift: async (row) => {
      if (driftCheckingIds.has(row.id)) return; // 同一渠道检测进行中，忽略重复触发
      setDriftCheckingIds((prev) => new Set(prev).add(row.id));
      try {
        const { success, message, data } = await checkChannelDrift(row.id);
        if (success) {
          const count = data?.missing_models?.length ?? 0;
          count > 0
            ? toast.warning(t('channel_row.driftFound', { channel: row.name, count }))
            : toast.success(t('channel_row.driftNone', { channel: row.name }));
          setRefreshFlag((f) => !f);
        } else if (message) toast.error(message);
      } catch (e) {
        toast.error(e.message);
      } finally {
        setDriftCheckingIds((prev) => {
          const next = new Set(prev);
          next.delete(row.id);
          return next;
        });
      }
    },
    // 忽略上游新增模型标记：仅就地更新该行的 model_drift（后端返回最新结果），不整页刷新。
    onDismissNewModels: async (row) => {
      if (dismissingIds.has(row.id)) return;
      setDismissingIds((prev) => new Set(prev).add(row.id));
      try {
        const { success, message, data } = await dismissChannelNewModels(row.id);
        if (success) {
          setChannels((prev) => prev.map((c) => (c.id === row.id ? { ...c, model_drift: data } : c)));
          toast.success(t('channel_row.newModelsDismissed', { channel: row.name }));
        } else if (message) toast.error(message);
      } catch (e) {
        toast.error(e.message);
      } finally {
        setDismissingIds((prev) => {
          const next = new Set(prev);
          next.delete(row.id);
          return next;
        });
      }
    },
    onTest: async (row) => {
      if (testingIds.has(row.id)) return; // 同一行测试进行中，忽略重复触发
      setTestingIds((prev) => new Set(prev).add(row.id));
      try {
        const { success, time, message, model } = await manageChannel(row.id, 'test', row.test_model);
        if (success) {
          toast.success(t('channel_row.modelTestSuccess', { channel: row.name, model, time: time.toFixed(2) }));
          setRefreshFlag((f) => !f);
        } else if (message) toast.error(message);
      } finally {
        setTestingIds((prev) => {
          const next = new Set(prev);
          next.delete(row.id);
          return next;
        });
      }
    },
    onRefreshBalance: async (row) => {
      try {
        const res = await API.get(`/api/channel/update_balance/${row.id}`);
        const { success, message } = res.data;
        success ? toast.success(t('channel_row.updateOk')) : toast.error(message);
      } catch (e) {
        toast.error(e.message);
      }
    },
    onCopy: async (row) => {
      const { success, message } = await manageChannel(row.id, 'copy');
      if (success) {
        toast.success(t('userPage.operationSuccess'));
        setRefreshFlag((f) => !f);
      } else if (message) toast.error(message);
    },
    onDelete: (row) => {
      setConfirm({
        open: true,
        title: `${t('common.delete')} #${row.id}`,
        onConfirm: async () => {
          const { success, message } = await manageChannel(row.id, 'delete');
          if (success) {
            toast.success(t('userPage.operationSuccess'));
            setRefreshFlag((f) => !f);
          } else if (message) toast.error(message);
        }
      });
    }
  };

  const columns = useMemo(
    () => getColumns({ t, handlers, testingIds, driftCheckingIds, dismissingIds }),
    [t, testingIds, driftCheckingIds, dismissingIds]
  );
  const table = useTable({
    features: appTableFeatures,
    data: channels,
    columns,
    state: { rowSelection },
    getRowId: (row) => String(row.id),
    enableRowSelection: (row) => !row.original.tag,
    onRowSelectionChange: setRowSelection,
    manualSorting: true,
    manualPagination: true
  });

  const selectedIds = Object.keys(rowSelection).map((id) => Number(id));

  const batchDelete = () => {
    if (!selectedIds.length) return toast.error(t('channel_index.pleaseSelectChannels'));
    setConfirm({
      open: true,
      title: t('channel_index.batchDeleteChannels'),
      onConfirm: async () => {
        const { success, message } = await manageChannel(null, 'batch_delete', selectedIds);
        if (success) {
          toast.success(t('channel_index.batchDeleteChannelsSuccess', { count: selectedIds.length }));
          setRowSelection({});
          setRefreshFlag((f) => !f);
        } else if (message) toast.error(message);
      }
    });
  };

  // 批量启用 / 禁用:逐个调用单渠道状态接口并汇总失败数。
  const batchSetStatus = async (status) => {
    if (!selectedIds.length || batchBusy) return;
    setBatchBusy(true);
    try {
      const results = await Promise.allSettled(selectedIds.map((id) => manageChannel(id, 'status', status)));
      const failed = results.filter((r) => r.status !== 'fulfilled' || !r.value?.success).length;
      if (failed > 0) toast.error(t('channelPage.batchStatusFailed', { count: failed }));
      else toast.success(t('userPage.operationSuccess'));
      setRowSelection({});
      setRefreshFlag((f) => !f);
    } finally {
      setBatchBusy(false);
    }
  };

  const bulk = (title, fn) => setConfirm({ open: true, title, content: '', onConfirm: fn });

  const requestConfirm = (title, content, fn) => setConfirm({ open: true, title, content, onConfirm: fn });

  const testAll = async () => {
    try {
      const res = await API.get('/api/channel/test');
      res.data.success ? toast.info(t('channel_row.testAllChannel')) : toast.error(res.data.message);
    } catch (e) {
      toast.error(e.message);
    }
  };
  const updateAllBalance = async () => {
    try {
      const res = await API.get('/api/channel/update_balance');
      res.data.success ? toast.info(t('channel_row.updateChannelBalance')) : toast.error(res.data.message);
    } catch (e) {
      toast.error(e.message);
    }
  };
  const checkAllDrift = async () => {
    if (driftCheckingAll) return;
    setDriftCheckingAll(true);
    try {
      const { success, message, data } = await checkChannelDrift();
      if (success) {
        toast.info(t('channel_index.checkDriftDone', { checked: data?.checked ?? 0, drifted: data?.drifted ?? 0 }));
        setRefreshFlag((f) => !f);
      } else if (message) toast.error(message);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setDriftCheckingAll(false);
    }
  };
  const deleteDisabled = async () => {
    try {
      const res = await API.delete('/api/channel/disabled');
      if (res.data.success) {
        toast.success(t('channel_row.delChannelCount', { count: res.data.data }));
        setRefreshFlag((f) => !f);
      } else toast.error(res.data.message);
    } catch (e) {
      toast.error(e.message);
    }
  };

  const activeStatus = queryParams.status != null ? String(queryParams.status) : null;
  const hasFilters = Object.keys(queryParams).length > 0;

  const statChips = [
    {
      id: 'all',
      label: t('channelPage.stats.all'),
      count: statusCounts.all,
      active: !activeStatus,
      onClick: () => applyStatusChip(null)
    },
    ...STATUS_CHIPS.map((c) => ({
      id: c.id,
      label: t(c.labelKey),
      count: statusCounts[c.id],
      tone: c.tone,
      active: activeStatus === c.status,
      onClick: () => applyStatusChip(c.status)
    }))
  ];

  const openCreate = () => {
    setEditId(0);
    setSheetTab('basic');
    setSheetOpen(true);
  };

  // 窄屏卡片:选择框在标题旁、操作在右上,其余字段单列「标签 / 值」。
  const renderCard = (original) => {
    if (original.tag) {
      return <TagCard item={original} requestConfirm={requestConfirm} onRefresh={() => setRefreshFlag((f) => !f)} />;
    }
    const row = table.getRow(String(original.id));
    const cellOf = (id) => row.getVisibleCells().find((c) => c.column.id === id);
    const render = (cell) => (cell ? flexRender(cell.column.columnDef.cell, cell.getContext()) : null);
    const fields = row.getVisibleCells().filter((c) => !['select', 'name', 'actions'].includes(c.column.id));
    return (
      <>
        <div className="flex items-center gap-2">
          {render(cellOf('select'))}
          <div className="min-w-0 flex-1">{render(cellOf('name'))}</div>
          {render(cellOf('actions'))}
        </div>
        {fields.length > 0 && (
          <dl className="mt-2 grid grid-cols-[6.5rem_minmax(0,1fr)] items-center gap-x-3 gap-y-1.5 pl-6">
            {fields.map((cell) => (
              <Fragment key={cell.id}>
                <dt className="text-xs font-medium text-muted-foreground">{flexRender(cell.column.columnDef.header, cell.getContext())}</dt>
                <dd className="flex min-w-0 flex-wrap items-center gap-1 text-sm">{render(cell)}</dd>
              </Fragment>
            ))}
          </dl>
        )}
      </>
    );
  };

  let body;
  if (!loading && channels.length === 0) {
    body = hasFilters ? (
      <ListEmptyState variant="noMatch" onClearFilters={handleClearFilters} />
    ) : (
      <ListEmptyState
        title={t('channelPage.emptyTitle')}
        description={t('channelPage.emptyDescription')}
        action={
          <Button size="sm" onClick={openCreate}>
            <Plus className="size-4" /> {t('channel_index.newChannel')}
          </Button>
        }
      />
    );
  } else {
    body = (
      <>
        <div className="hidden md:block">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id}>
                  {hg.headers.map((header) => {
                    const canSort =
                      header.column.columnDef.enableSorting !== false &&
                      !['select', 'actions', 'used', 'group', 'models', 'enabled'].includes(header.column.id);
                    return (
                      <TableHead
                        key={header.id}
                        onClick={canSort ? () => onSortClick(header.column.id) : undefined}
                        className={`whitespace-nowrap ${canSort ? 'cursor-pointer select-none' : ''} ${header.column.columnDef.meta?.className ?? ''}`}
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {orderBy === header.column.id ? (order === 'desc' ? ' ↓' : ' ↑') : ''}
                      </TableHead>
                    );
                  })}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {channels.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length} className="h-24 text-center text-muted-foreground">
                    …
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) =>
                  row.original.tag ? (
                    <TagRow
                      key={`tag-${row.original.tag}`}
                      item={row.original}
                      colSpan={columns.length}
                      requestConfirm={requestConfirm}
                      onRefresh={() => setRefreshFlag((f) => !f)}
                    />
                  ) : (
                    <TableRow key={row.id} data-state={row.getIsSelected() ? 'selected' : undefined}>
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id} className={cell.column.columnDef.meta?.className}>
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </TableCell>
                      ))}
                    </TableRow>
                  )
                )
              )}
            </TableBody>
          </Table>
        </div>
        <DataCards
          table={table}
          className="md:hidden"
          renderCard={renderCard}
          empty={t('common.noData', { defaultValue: 'No data' })}
          searching={loading}
        />
      </>
    );
  }

  return (
    <>
      <div>
        {/* 页头主操作:刷新(图标) → 更多(全量维护动作) → 新建。 */}
        <PageActions>
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => doRefresh(true)}
                  aria-label={t('channel_index.refreshClearSearchConditions')}
                >
                  <RotateCcw className="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('channel_index.refreshClearSearchConditions')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          {/* 更多:批量处理 + 全量维护动作。 */}
          <DropdownMenu>
            <TooltipProvider delayDuration={150}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm" aria-label={t('common.more')}>
                      <MoreVertical className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent>{t('common.more')}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setBatchOpen(true)}>
                <Layers className="size-4" />
                <span>{t('channel_index.batchProcessing')}</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => bulk(t('channel_index.testAllChannels'), testAll)}>
                <FlaskConical className="size-4" />
                <span>{t('channel_index.testAllChannels')}</span>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => bulk(t('channel_index.updateEnabledBalance'), updateAllBalance)}>
                <DollarSign className="size-4" />
                <span>{t('channel_index.updateEnabledBalance')}</span>
              </DropdownMenuItem>
              <DropdownMenuItem disabled={driftCheckingAll} onClick={() => bulk(t('channel_index.checkAllDrift'), checkAllDrift)}>
                <AlertTriangle className="size-4" />
                <span>{t('channel_index.checkAllDrift')}</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="text-destructive hover:text-destructive focus:text-destructive"
                onClick={() => bulk(t('channel_index.deleteDisabledChannels'), deleteDisabled)}
              >
                <Trash2 className="size-4" />
                <span>{t('channel_index.deleteDisabledChannels')}</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <ResponsiveToolbarButton primary size="sm" icon={Plus} label={t('channel_index.newChannel')} onClick={openCreate} />
        </PageActions>

        <ListPage
          className="max-md:[&>[data-slot=list-page-footer]]:static"
          header={
            <div className="flex flex-wrap items-center gap-1">
              {statChips.map(({ id, ...chip }) => (
                <StatChip key={id} {...chip} />
              ))}
            </div>
          }
          toolbar={
            <ListToolbar
              className="max-sm:[&>div:first-child]:w-auto max-sm:[&>div:first-child]:min-w-0 max-sm:[&>div:first-child]:flex-1"
              search={{ value: keyword, onChange: setKeyword, placeholder: t('channelPage.searchPlaceholder') }}
              filters={{ fields: filterFields, state: filterState, onChange: handleFilterChange, onClearAll: handleClearFilters }}
            />
          }
          selectedCount={selectedIds.length}
          batchBar={
            <BatchBar
              count={selectedIds.length}
              onClear={() => setRowSelection({})}
              actions={
                <>
                  <Button size="sm" variant="outline" disabled={batchBusy} onClick={() => batchSetStatus(1)}>
                    <Power className="size-4" />
                    {t('channel_row.enable')}
                  </Button>
                  <Button size="sm" variant="outline" disabled={batchBusy} onClick={() => batchSetStatus(2)}>
                    <PowerOff className="size-4" />
                    {t('channel_row.disable')}
                  </Button>
                  <Button size="sm" variant="outline" className="max-sm:hidden" onClick={() => setBatchOpen(true)}>
                    <Layers className="size-4" />
                    {t('channel_index.batchProcessing')}
                  </Button>
                  <Button size="sm" variant="outline" className="text-destructive" disabled={batchBusy} onClick={batchDelete}>
                    <Trash2 className="size-4" />
                    {t('common.delete')}
                  </Button>
                </>
              }
            />
          }
          footer={
            <ListFooter
              total={listCount}
              page={page}
              pageSize={rowsPerPage}
              selectedCount={selectedIds.length}
              onPageChange={setPage}
              onPageSizeChange={(n) => {
                setRowsPerPage(n);
                savePageSize('channel', n);
                setPage(0);
              }}
            />
          }
        >
          {body}
        </ListPage>

        <ChannelSheet
          open={sheetOpen}
          channelId={editId || 0}
          initialTab={sheetTab}
          groupOptions={groupOptions}
          onClose={() => setSheetOpen(false)}
          onSaved={() => {
            setSheetOpen(false);
            setRefreshFlag((f) => !f);
          }}
        />

        <ChannelCheckDialog open={!!checkItem} item={checkItem} onClose={() => setCheckItem(null)} />

        <BatchModal open={batchOpen} onOpenChange={setBatchOpen} groupOptions={groupOptions} modelOptions={modelOptions} />

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
      </div>
    </>
  );
}
