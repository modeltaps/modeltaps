import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  ChevronDown,
  Columns3,
  DollarSign,
  Download,
  DownloadCloud,
  Eye,
  EyeOff,
  Info,
  MoreHorizontal,
  Plus,
  RefreshCw,
  RotateCcw,
  Trash2,
  Upload
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { toast } from '@/components/ui/sonner';
import { filterStateToParams, paramsToFilterState } from '@/components/filter-bar';
import { BatchBar, ListEmptyState, ListFooter, ListPage, ListToolbar, StatChip } from '@/components/list-page';
import PageActions from '@/components/chrome/PageActions';
import { API } from 'utils/api';
import { downloadTextAsFile, showError } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import PricingSheet from '../Pricing/PricingSheet';
import CheckUpdatesDialog from '../Pricing/CheckUpdatesDialog';
import {
  STATUS_FILTERS,
  TOGGLEABLE_COLUMNS,
  attachChannelDetails,
  billingType,
  buildCatalogExport,
  catalogStats,
  indexChannelsByModel,
  isFullyUsable,
  isVisibleToUsers,
  loadColumnVisibility,
  loadPriceUnit,
  matchesStatusFilter,
  mergeCatalogRows,
  saveColumnVisibility,
  savePriceUnit
} from '../Pricing/modelCatalog';
import ModelTable from './ModelTable';
import ModelInfoSheet from './ModelInfoSheet';
import ImportDialog from './ImportDialog';
import { ENDPOINT_LABEL_KEYS, ENDPOINT_VALUES, isUnlabeled, safeJsonArray } from './modelInfoHelpers';

// 工具栏分面:厂商 / 状态 / 可见性 / 计费方式 / 接口 + 「仅看未标注」。全部为前端本地过滤,
// 单选枚举的 paramInclude 即字段键,刷新可由 URL 恢复;状态分面与页头统计 chip 共用取值。
const singleEnum = (key, labelKey, options) => ({
  key,
  labelKey,
  type: 'enum',
  single: true,
  supportsExclude: false,
  paramInclude: key,
  options
});

const STAT_TONES = { usable: 'success', nochannel: 'warning', unpriced: 'warning', hidden: 'default' };

const buildFilterFields = (vendors) => [
  singleEnum('vendor_id', 'modelInfoPage.vendor', [
    ...Object.values(vendors).map((v) => ({ value: String(v.id), label: v.name })),
    { value: '0', labelKey: 'modelInfoPage.vendorUnknown' }
  ]),
  singleEnum(
    'status',
    'modelInfoPage.state',
    STATUS_FILTERS.map((s) => ({ value: s, labelKey: `modelsPage.stats.${s}` }))
  ),
  singleEnum('visible', 'modelsPage.visibility', [
    { value: 'true', labelKey: 'modelsPage.visibleYes' },
    { value: 'false', labelKey: 'modelsPage.visibleNo' }
  ]),
  singleEnum('billing', 'modelsPage.billing', [
    { value: 'tokens', labelKey: 'modelsPage.billingTokens' },
    { value: 'times', labelKey: 'modelsPage.billingTimes' },
    { value: 'none', labelKey: 'modelsPage.billingNone' }
  ]),
  singleEnum(
    'endpoint',
    'modelInfoPage.endpoints',
    ENDPOINT_VALUES.map((e) => ({ value: e, labelKey: ENDPOINT_LABEL_KEYS[e] }))
  ),
  singleEnum('labels', 'modelsPage.labeling', [{ value: 'unlabeled', labelKey: 'modelInfoPage.onlyUnlabeled' }])
];

const MODELINFO_FILTER_PARAM_KEYS = ['vendor_id', 'status', 'visible', 'billing', 'endpoint', 'labels'];
// 参数映射只依赖 key/type/single/paramInclude,与动态厂商选项无关,URL 解析用静态描述即可。
const MODELINFO_FILTER_FIELDS = buildFilterFields({});

// 单选分面的 filterState 项(与 FilterBar 写入的形状一致)。
const enumFilterEntry = (key, value) => paramsToFilterState(MODELINFO_FILTER_FIELDS, new URLSearchParams({ [key]: value }))[key];

export function filterCatalogRows(rows, { keyword = '', params = {}, onlyUsable = false, aliasesByTarget = {} }) {
  const k = keyword.trim().toLowerCase();
  return rows.filter((row) => {
    if (onlyUsable && !isFullyUsable(row)) return false;
    if (params.status && !matchesStatusFilter(row, params.status)) return false;
    if (params.vendor_id && String(row.vendor_id || 0) !== params.vendor_id) return false;
    if (params.visible && String(isVisibleToUsers(row)) !== params.visible) return false;
    if (params.billing && billingType(row) !== params.billing) return false;
    if (params.endpoint && !safeJsonArray(row.endpoints).includes(params.endpoint)) return false;
    if (params.labels === 'unlabeled' && !(row.catalogId > 0 && isUnlabeled(row))) return false;
    if (!k) return true;
    return (
      row.model?.toLowerCase().includes(k) ||
      row.name?.toLowerCase().includes(k) ||
      row.alias_of?.toLowerCase().includes(k) ||
      (aliasesByTarget[row.model] || []).some((alias) => alias.toLowerCase().includes(k)) ||
      row.tags?.toLowerCase().includes(k)
    );
  });
}

const CHANNEL_PAGE_SIZE = 100;
const CLOSED_PRICE_SHEET = { open: false, item: null };

// 无目录行的模型需要渠道名单来判断可路由性:分页拉取全部渠道(单页上限 100)。
async function fetchAllChannels() {
  const all = [];
  for (let page = 1; ; page += 1) {
    const res = await API.get('/api/channel/', { params: { page, size: CHANNEL_PAGE_SIZE } });
    const { success, data } = res.data;
    if (!success) break;
    const list = data?.data || [];
    all.push(...list);
    if (list.length < CHANNEL_PAGE_SIZE || all.length >= (data?.total_count || 0)) break;
  }
  return all;
}

function ColumnMenu({ visibility, onChange }) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" aria-label={t('modelInfoPage.columns')}>
          <Columns3 className="size-4" />
          <span className="hidden sm:inline">{t('modelsPage.columnsShort')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-[60vh] w-48 overflow-y-auto">
        <DropdownMenuLabel>{t('modelInfoPage.columns')}</DropdownMenuLabel>
        {TOGGLEABLE_COLUMNS.map((column) => (
          <label key={column.id} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-muted">
            <Checkbox
              checked={!!visibility[column.id]}
              onCheckedChange={(checked) => onChange({ ...visibility, [column.id]: !!checked })}
            />
            {t(column.labelKey)}
          </label>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

ColumnMenu.propTypes = { visibility: PropTypes.object.isRequired, onChange: PropTypes.func.isRequired };

export default function ModelInfo() {
  const { t } = useTranslation();
  const [modelInfos, setModelInfos] = useState([]);
  const [prices, setPrices] = useState([]);
  const [modelList, setModelList] = useState([]);
  const [channelsByModel, setChannelsByModel] = useState({});
  const [channelList, setChannelList] = useState([]);
  const [vendors, setVendors] = useState({});
  const [ownedby, setOwnedby] = useState([]);
  const [refreshFlag, setRefreshFlag] = useState(false);
  const [loading, setLoading] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(MODELINFO_FILTER_FIELDS, searchParams));
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('model_info'));
  const [keyword, setKeyword] = useState(() => searchParams.get('keyword') || '');
  const [onlyUsable, setOnlyUsable] = useState(() => searchParams.get('usable') === '1');
  const [selectedKeys, setSelectedKeys] = useState(() => new Set());
  const [hiding, setHiding] = useState(false);
  const [unit, setUnit] = useState(() => loadPriceUnit());
  const [columnVisibility, setColumnVisibility] = useState(() => loadColumnVisibility());

  const [sheetOpen, setSheetOpen] = useState(false);
  const [sheetTab, setSheetTab] = useState('profile');
  const [editRow, setEditRow] = useState(null);
  const [batchDelete, setBatchDelete] = useState(null);
  const [importOpen, setImportOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [priceSheet, setPriceSheet] = useState(CLOSED_PRICE_SHEET);
  const [deletePriceTarget, setDeletePriceTarget] = useState(null);
  const [checkOpen, setCheckOpen] = useState(false);
  const [billingInfoOpen, setBillingInfoOpen] = useState(false);

  // reqId 守卫(UX-13):快速连点刷新时丢弃过期响应;本页为一次性拉全量 + 前端过滤/分页,不迁 usePaginatedList。
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  // 目录走管理员接口 /api/model_info/catalog(含 state 与 bound_channels 计算字段),与价格表、
  // 渠道可提供模型按模型名合并;渠道列表用于判断无目录行模型的可路由性,并补全渠道列的图标 / 提示信息。
  const fetchData = useCallback(
    () =>
      runGuardedFetch(
        guardRef.current,
        async () => {
          const [cres, pres, mres] = await Promise.all([
            API.get('/api/model_info/catalog'),
            API.get('/api/prices'),
            API.get('/api/prices/model_list')
          ]);
          const catalog = cres.data.success ? cres.data.data || [] : [];
          const priceRows = pres.data.success ? pres.data.data || [] : [];
          const models = mres.data.success ? mres.data.data || [] : [];
          const channels = await fetchAllChannels();
          return { cres, catalog, priceRows, models, channels };
        },
        {
          onStart: () => setLoading(true),
          onResult: ({ cres, catalog, priceRows, models, channels }) => {
            if (!cres.data.success) showError(cres.data.message);
            setModelInfos(catalog);
            setPrices(priceRows);
            setModelList(models);
            setChannelList(channels);
            setChannelsByModel(indexChannelsByModel(channels));
          },
          onError: (error) => console.error(error),
          onFinally: () => setLoading(false)
        }
      ),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [refreshFlag]
  );

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  useEffect(() => {
    API.get('/api/model_ownedby/')
      .then((res) => {
        const { success, data } = res.data;
        if (!success) return;
        setVendors(Object.fromEntries((data || []).map((v) => [v.id, v])));
      })
      .catch(() => {});
    // 价格表单的供应商类型选项。
    API.get('/api/ownedby')
      .then((res) => {
        const { success, data } = res.data;
        if (!success) return;
        setOwnedby(Object.keys(data).map((k) => ({ value: parseInt(k, 10), label: data[k]?.name || '' })));
      })
      .catch(() => {});
  }, []);

  // 已应用筛选同步到 URL(replace,刷新可恢复;先清空筛选键再按当前 state 重写)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        [...MODELINFO_FILTER_PARAM_KEYS, 'keyword', 'usable'].forEach((k) => params.delete(k));
        const q = filterStateToParams(MODELINFO_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        if (keyword) params.set('keyword', keyword);
        if (onlyUsable) params.set('usable', '1');
        return params;
      },
      { replace: true }
    );
  }, [filterState, keyword, onlyUsable, setSearchParams]);

  const handleRefresh = () => setRefreshFlag((f) => !f);

  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleKeywordChange = (value) => {
    setPage(0);
    setKeyword(value);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
    setKeyword('');
    setOnlyUsable(false);
  };

  // 页头统计 chip:点击等价于设置「状态」分面(「全部」清除该分面)。
  const applyStatusChip = (value) => {
    setPage(0);
    setFilterState((prev) => {
      const next = { ...prev };
      if (value) next.status = enumFilterEntry('status', value);
      else delete next.status;
      return next;
    });
  };

  const handleColumnVisibilityChange = (next) => {
    setColumnVisibility(next);
    saveColumnVisibility(next);
  };

  const handleUnitChange = useCallback((next) => {
    setUnit(next);
    savePriceUnit(next);
  }, []);

  // 厂商选项随 /api/model_ownedby/ 变化;字段定义交给 FilterBar 展示。
  const filterFields = useMemo(() => buildFilterFields(vendors), [vendors]);

  const existingModels = useMemo(() => modelInfos.map((info) => info.model), [modelInfos]);

  // 别名索引:主名 → 指向它的别名行,用于关键字搜索兼容别名。
  const aliasesByTarget = useMemo(() => {
    const map = {};
    modelInfos.forEach((info) => {
      if (!info.alias_of) return;
      (map[info.alias_of] ||= []).push(info.model);
    });
    return map;
  }, [modelInfos]);

  const rows = useMemo(
    () => attachChannelDetails(mergeCatalogRows({ catalog: modelInfos, prices, modelList, channelsByModel }), channelList),
    [modelInfos, prices, modelList, channelsByModel, channelList]
  );

  // 本地过滤条件(搜索框 + 分面 + 「只看完整可用」)。
  const params = useMemo(() => filterStateToParams(MODELINFO_FILTER_FIELDS, filterState), [filterState]);

  const stats = useMemo(() => catalogStats(rows), [rows]);
  const unlabeledCount = useMemo(() => modelInfos.filter(isUnlabeled).length, [modelInfos]);

  const filtered = useMemo(
    () => filterCatalogRows(rows, { keyword, params, onlyUsable, aliasesByTarget }),
    [rows, keyword, params, onlyUsable, aliasesByTarget]
  );

  const lastPage = Math.max(0, Math.ceil(filtered.length / rowsPerPage) - 1);
  const currentPage = Math.min(page, lastPage);
  const pageRows = filtered.slice(currentPage * rowsPerPage, currentPage * rowsPerPage + rowsPerPage);

  const toggleSelect = useCallback((key) => {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }, []);

  // 全选只作用于当前页,与分页展示保持一致。
  const toggleAll = useCallback(
    (select) => {
      setSelectedKeys((prev) => {
        const next = new Set(prev);
        pageRows.forEach((row) => {
          if (select) next.add(row.key);
          else next.delete(row.key);
        });
        return next;
      });
    },
    [pageRows]
  );

  // 隐藏开关只作用于目录行;选中的无目录行模型不参与批量隐藏。
  const selectedCatalogIds = useMemo(
    () => rows.filter((row) => row.catalogId > 0 && selectedKeys.has(row.key)).map((row) => row.catalogId),
    [rows, selectedKeys]
  );

  const hide = useCallback(
    async (ids, hidden) => {
      if (ids.length === 0) return;
      setHiding(true);
      try {
        const res = await API.post('/api/model_info/hide', { ids, hidden });
        const { success, message } = res.data;
        if (success) {
          toast.success(t('common.operationSuccess'));
          setSelectedKeys(new Set());
          handleRefresh();
        } else {
          showError(message);
        }
      } catch (error) {
        showError(error);
      } finally {
        setHiding(false);
      }
    },
    [t]
  );

  const toggleHidden = useCallback((item, next) => hide([item.catalogId], next), [hide]);

  // 新建与行编辑共用一个抽屉(资料 / 定价 / 渠道与分组),行内入口决定默认 Tab。
  const openCreate = () => {
    setEditRow(null);
    setSheetTab('profile');
    setSheetOpen(true);
  };
  const openSheet = useCallback((row, tab = 'profile') => {
    setEditRow(row);
    setSheetTab(tab);
    setSheetOpen(true);
  }, []);
  const onSaved = () => {
    setSheetOpen(false);
    setEditRow(null);
    handleRefresh();
  };

  // 批量定价:选中模型统一设价,已有价格的模型走更新,其余新增。
  const openBatchPrice = () => {
    const selected = rows.filter((row) => selectedKeys.has(row.key));
    setPriceSheet({
      open: true,
      item: { models: selected.map((row) => row.model), original_models: selected.filter((row) => row.price).map((row) => row.model) }
    });
  };
  const closePriceSheet = () => setPriceSheet(CLOSED_PRICE_SHEET);
  const onPriceSaved = () => {
    closePriceSheet();
    setSelectedKeys(new Set());
    handleRefresh();
  };

  const noPriceModels = useMemo(() => rows.filter((row) => !row.price).map((row) => row.model), [rows]);

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const res = await API.delete('/api/model_info/' + deleteTarget.catalogId);
      if (res.data?.success) {
        toast.success(t('common.operationSuccess'));
        handleRefresh();
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const confirmDeletePrice = async () => {
    if (!deletePriceTarget) return;
    setDeleting(true);
    try {
      const res = await API.delete('/api/prices/single/' + encodeURIComponent(deletePriceTarget.model));
      if (res.data?.success) {
        toast.success(t('pricing_edit.saveOk'));
        handleRefresh();
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setDeleting(false);
      setDeletePriceTarget(null);
    }
  };

  const selectedRows = useMemo(() => rows.filter((row) => selectedKeys.has(row.key)), [rows, selectedKeys]);
  const selectedPricedModels = selectedRows.filter((row) => row.price).map((row) => row.model);

  // 批量删除价格走 /api/prices/multiple/delete;目录无批量接口,逐条删除并汇总失败数。
  const confirmBatchDelete = async () => {
    if (!batchDelete) return;
    setDeleting(true);
    try {
      if (batchDelete === 'price') {
        const res = await API.put('/api/prices/multiple/delete', { models: selectedPricedModels });
        if (!res.data?.success) return showError(res.data?.message);
      } else {
        const results = await Promise.allSettled(selectedCatalogIds.map((id) => API.delete('/api/model_info/' + id)));
        const failed = results.filter((r) => r.status !== 'fulfilled' || !r.value.data?.success).length;
        if (failed > 0) showError(t('modelsPage.batchDeleteFailed', { count: failed }));
      }
      toast.success(t('common.operationSuccess'));
      setSelectedKeys(new Set());
    } catch (error) {
      showError(error);
    } finally {
      setDeleting(false);
      setBatchDelete(null);
      handleRefresh();
    }
  };

  const handleExport = () => {
    const date = new Date().toISOString().slice(0, 10);
    downloadTextAsFile(JSON.stringify(buildCatalogExport(rows), null, 2), `models-${date}.json`);
  };

  const handleSyncCatalog = async () => {
    try {
      const res = await API.post('/api/prices/sync_catalog');
      const { success, message, data } = res.data;
      if (success) {
        toast.success(t('pricingPage.syncCatalogDone', { count: data?.synced ?? 0 }));
        handleRefresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const statChips = [
    { id: 'all', label: t('modelsPage.stats.all'), count: stats.all, active: !params.status, onClick: () => applyStatusChip(null) },
    ...STATUS_FILTERS.map((s) => ({
      id: s,
      label: t(`modelsPage.stats.${s}`),
      count: stats[s],
      tone: STAT_TONES[s],
      active: params.status === s,
      onClick: () => applyStatusChip(s)
    }))
  ];

  let body;
  if (!loading && rows.length === 0) {
    body = (
      <ListEmptyState
        title={t('modelsPage.emptyTitle')}
        description={t('modelsPage.emptyDescription')}
        action={
          <Button size="sm" onClick={openCreate}>
            <Plus className="size-4" /> {t('modelInfoPage.create')}
          </Button>
        }
      />
    );
  } else if (!loading && filtered.length === 0) {
    body = <ListEmptyState variant="noMatch" onClearFilters={handleClearFilters} />;
  } else {
    body = (
      <ModelTable
        items={pageRows}
        vendors={vendors}
        unit={unit}
        onUnitChange={handleUnitChange}
        columnVisibility={columnVisibility}
        selectedKeys={selectedKeys}
        onToggleSelect={toggleSelect}
        onToggleAll={toggleAll}
        onToggleHidden={toggleHidden}
        onOpen={openSheet}
        onDeleteCatalog={setDeleteTarget}
        onDeletePrice={setDeletePriceTarget}
      />
    );
  }

  return (
    <>
      {/* 页头主操作:「同步」下拉 + ⋯ 菜单 + 新增模型(primary)。 */}
      <PageActions>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" aria-label={t('modelsPage.sync')}>
              <RefreshCw className="size-4" />
              <span>{t('modelsPage.sync')}</span>
              <ChevronDown className="size-3.5 text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-60">
            <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">{t('modelsPage.syncGroup')}</DropdownMenuLabel>
            <DropdownMenuItem onClick={handleSyncCatalog}>
              <RefreshCw className="size-4" />
              {t('pricingPage.syncCatalogButton')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setCheckOpen(true)}>
              <DownloadCloud className="size-4" />
              {t('pricingPage.updatePricesButton')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => setImportOpen(true)}>
              <Upload className="size-4" />
              {t('modelInfoPage.importTitle')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={handleExport} disabled={rows.length === 0}>
              <Download className="size-4" />
              {t('modelsPage.export')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="px-2" aria-label={t('modelsPage.moreActions')}>
              <MoreHorizontal className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem onClick={handleRefresh}>
              <RotateCcw className="size-4" />
              {t('modelInfoPage.refresh')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setBillingInfoOpen(true)}>
              <Info className="size-4" />
              {t('pricingPage.billingInfoTitle')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button size="sm" onClick={openCreate} aria-label={t('modelInfoPage.create')}>
          <Plus className="size-4" />
          <span>{t('modelInfoPage.create')}</span>
        </Button>
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
            className="max-sm:[&>div:first-child]:w-auto max-sm:[&>div:first-child]:min-w-0 max-sm:[&>div:first-child]:flex-1 max-sm:[&>div:last-child]:w-full max-sm:[&>div:last-child]:flex-wrap max-sm:[&>div:last-child]:justify-end"
            search={{ value: keyword, onChange: handleKeywordChange, placeholder: t('modelsPage.searchPlaceholder') }}
            filters={{ fields: filterFields, state: filterState, onChange: handleFilterChange, onClearAll: handleClearFilters }}
            viewControls={
              <>
                <label className="flex items-center gap-2 whitespace-nowrap text-sm max-sm:basis-full">
                  <Switch
                    checked={onlyUsable}
                    onCheckedChange={(v) => {
                      setPage(0);
                      setOnlyUsable(v);
                    }}
                    aria-label={t('modelsPage.onlyUsable')}
                  />
                  {t('modelsPage.onlyUsable')}
                </label>
                <ColumnMenu visibility={columnVisibility} onChange={handleColumnVisibilityChange} />
              </>
            }
          />
        }
        selectedCount={selectedKeys.size}
        batchBar={
          <BatchBar
            count={selectedKeys.size}
            onClear={() => setSelectedKeys(new Set())}
            actions={
              <>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={hiding || selectedCatalogIds.length === 0}
                  onClick={() => hide(selectedCatalogIds, false)}
                >
                  <Eye className="size-4" />
                  {t('modelsPage.batchShow')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={hiding || selectedCatalogIds.length === 0}
                  onClick={() => hide(selectedCatalogIds, true)}
                >
                  <EyeOff className="size-4" />
                  {t('modelsPage.batchHide')}
                </Button>
                <Button size="sm" variant="outline" onClick={openBatchPrice}>
                  <DollarSign className="size-4" />
                  {t('modelInfoPage.setPrice')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="text-destructive"
                  disabled={selectedPricedModels.length === 0}
                  onClick={() => setBatchDelete('price')}
                >
                  <Trash2 className="size-4" />
                  {t('modelInfoPage.deletePrice')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="text-destructive"
                  disabled={selectedCatalogIds.length === 0}
                  onClick={() => setBatchDelete('catalog')}
                >
                  <Trash2 className="size-4" />
                  {t('modelInfoPage.deleteCatalog')}
                </Button>
              </>
            }
          />
        }
        footer={
          <ListFooter
            total={filtered.length}
            page={currentPage}
            pageSize={rowsPerPage}
            selectedCount={selectedKeys.size}
            onPageChange={setPage}
            onPageSizeChange={(n) => {
              setRowsPerPage(n);
              setPage(0);
              savePageSize('model_info', n);
            }}
          />
        }
      >
        {body}
      </ListPage>

      {unlabeledCount > 0 && params.labels !== 'unlabeled' && (
        <p className="mt-2 text-xs text-muted-foreground">
          <button
            type="button"
            className="underline-offset-2 hover:underline"
            onClick={() => handleFilterChange((prev) => ({ ...prev, labels: enumFilterEntry('labels', 'unlabeled') }))}
          >
            {t('modelInfoPage.unlabeledCount', { count: unlabeledCount })}
          </button>
        </p>
      )}

      <div>
        <ModelInfoSheet
          open={sheetOpen}
          onOpenChange={setSheetOpen}
          row={editRow}
          initialTab={sheetTab}
          existingModels={existingModels}
          prices={prices}
          ownedby={ownedby}
          unit={unit}
          onSaved={onSaved}
        />

        <ImportDialog
          open={importOpen}
          onOpenChange={setImportOpen}
          existingModels={existingModels}
          onImported={() => {
            setImportOpen(false);
            handleRefresh();
          }}
        />

        <PricingSheet
          open={priceSheet.open}
          priceItem={priceSheet.item}
          ownedby={ownedby}
          modelOptions={[...new Set([...noPriceModels, ...(priceSheet.item?.models || [])])]}
          unit={unit}
          onClose={closePriceSheet}
          onSaved={onPriceSaved}
        />

        <CheckUpdatesDialog
          open={checkOpen}
          row={prices}
          onClose={() => setCheckOpen(false)}
          onOk={() => {
            setCheckOpen(false);
            handleRefresh();
          }}
        />

        <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('modelInfoPage.deleteCatalog')}</DialogTitle>
              <DialogDescription>{t('common.deleteConfirm', { title: deleteTarget?.name || deleteTarget?.model })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeleteTarget(null)}>
                {t('common.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmDelete} disabled={deleting}>
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={!!deletePriceTarget} onOpenChange={(o) => !o && setDeletePriceTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('pricing_edit.delTip')}</DialogTitle>
              <DialogDescription>{t('pricing_edit.delInfoTip', { name: deletePriceTarget?.model })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeletePriceTarget(null)}>
                {t('common.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmDeletePrice} disabled={deleting}>
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* 计费说明(「更多」入口的弹层)。 */}
        <Dialog open={billingInfoOpen} onOpenChange={setBillingInfoOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('pricingPage.billingInfoTitle')}</DialogTitle>
            </DialogHeader>
            <div className="space-y-1 text-sm">
              <p>
                <b>{t('pricingPage.currencyInfo1')}</b>
                {t('pricingPage.currencyInfo2')}
              </p>
              <p>
                <b>{t('pricingPage.currencyInfo3')}</b>
                {t('pricingPage.currencyInfo4')}
              </p>
              <p>
                <b>{t('pricingPage.currencyInfo5')}</b>
                {t('pricingPage.currencyInfo6')}
              </p>
              <p>{t('pricingPage.currencyInfo7')}</p>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setBillingInfoOpen(false)}>
                {t('common.close')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={!!batchDelete} onOpenChange={(o) => !o && setBatchDelete(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{batchDelete === 'price' ? t('modelInfoPage.deletePrice') : t('modelInfoPage.deleteCatalog')}</DialogTitle>
              <DialogDescription>
                {t('modelsPage.batchDeleteConfirm', {
                  count: batchDelete === 'price' ? selectedPricedModels.length : selectedCatalogIds.length
                })}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setBatchDelete(null)}>
                {t('common.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmBatchDelete} disabled={deleting}>
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </>
  );
}
