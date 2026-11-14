import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import {
  ChevronDown,
  DollarSign,
  Download,
  DownloadCloud,
  Eye,
  EyeOff,
  Info,
  Library,
  MoreHorizontal,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  Trash2,
  Upload,
  Boxes
} from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
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
import { FilterSheet, describeEntry, filterStateToParams, isEntryActive, paramsToFilterState } from '@/components/filter-bar';
import {
  BatchBar,
  ColumnPicker,
  ConditionBar,
  ListEmptyState,
  PageEmptyState,
  ListFooter,
  ListSkeleton,
  PhoneListToolbar,
  PresetChips,
  StatusSegments,
  useColumnVisibility,
  useFilterPresets,
  useListLayout
} from '@/components/list-page';
import PageActions from '@/components/chrome/PageActions';
import { API } from 'utils/api';
import { downloadTextAsFile, showError } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import PricingSheet from '../Pricing/PricingSheet';
import SupplyBatchMenu from './supply/SupplyBatchMenu';
import CheckUpdatesDialog from '../Pricing/CheckUpdatesDialog';
import {
  COLUMN_STORAGE_KEY,
  LEGACY_COLUMN_STORAGE_KEYS,
  MODEL_COLUMNS,
  STATUS_FILTERS,
  attachChannelDetails,
  billingType,
  buildCatalogExport,
  catalogStats,
  indexChannelsByModel,
  isVisibleToUsers,
  loadPriceUnit,
  matchesStatusFilter,
  mergeCatalogRows,
  channelModelNames,
  suppliedUpstreamKeys,
  savePriceUnit
} from '../Pricing/modelCatalog';
import ModelTable, { ModelList, ReadinessLegend } from './ModelTable';
import ModelInfoSheet from './ModelInfoSheet';
import ImportDialog from './ImportDialog';
import LibrarySheet from './LibrarySheet';
import { ENDPOINT_LABEL_KEYS, ENDPOINT_VALUES, isUnlabeled, safeJsonArray } from './modelInfoHelpers';
import { listSupplies } from './supply/supplyApi';
import { useSupplySwitches, SupplySwitchNote } from './supply/supplyParts';
import { MODEL_STATUSES, countSuppliesByModel, isPublicModelRow, normalizeModelStatus } from './supply/supplyHelpers';

// Conditions in the "Show models that" bar: vendor, public status, visibility, billing, endpoint
// and "unlabeled". All filter locally; each single-choice field's URL key is its field key, so a
// reload restores them. The catalog state (usable / no channel / unpriced / hidden) is the
// segment row above, not a condition.
const singleEnum = (key, labelKey, options) => ({
  key,
  labelKey,
  type: 'enum',
  single: true,
  supportsExclude: false,
  paramInclude: key,
  options
});

const buildFilterFields = (vendors) => [
  singleEnum('vendor_id', 'modelInfoPage.vendor', [
    ...Object.values(vendors).map((v) => ({ value: String(v.id), label: v.name })),
    { value: '0', labelKey: 'modelInfoPage.vendorUnknown' }
  ]),
  singleEnum(
    'publish',
    'modelSupply.status.label',
    MODEL_STATUSES.map((s) => ({ value: s, labelKey: `modelSupply.status.${s}` }))
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

const MODELINFO_FILTER_PARAM_KEYS = ['vendor_id', 'publish', 'visible', 'billing', 'endpoint', 'labels'];
// Parameter mapping depends only on key / type / single / paramInclude, not on the vendor
// options, so URL parsing can use the static description.
const MODELINFO_FILTER_FIELDS = buildFilterFields({});

// A single-choice condition entry (the shape the condition bar writes).
const enumFilterEntry = (key, value) => paramsToFilterState(MODELINFO_FILTER_FIELDS, new URLSearchParams({ [key]: value }))[key];

// Segments (?status=): all, then the catalog states. They overlap (an unpriced model can also
// have no channel), as the counts do.
const STATUS_SEGMENTS = ['all', ...STATUS_FILTERS];
const readStatus = (searchParams) => {
  if (searchParams.get('usable') === '1') return 'usable';
  const s = searchParams.get('status');
  return STATUS_SEGMENTS.includes(s) ? s : 'all';
};

// Built-in presets.
const BUILTIN_PRESETS = [
  { id: 'unlabeled', labelKey: 'modelsPage.presetUnlabeled', status: 'all', conditions: { labels: { op: 'in', values: ['unlabeled'] } } },
  { id: 'percall', labelKey: 'modelsPage.presetPerCall', status: 'all', conditions: { billing: { op: 'in', values: ['times'] } } }
];
const PRESET_STORAGE_KEY = 'model-filter-presets-v1';

export function filterCatalogRows(rows, { keyword = '', params = {}, aliasesByTarget = {} }) {
  const k = keyword.trim().toLowerCase();
  return rows.filter((row) => {
    if (params.status && params.status !== 'all' && !matchesStatusFilter(row, params.status)) return false;
    if (params.vendor_id && String(row.vendor_id || 0) !== params.vendor_id) return false;
    if (params.publish && !(isPublicModelRow(row) && normalizeModelStatus(row.status) === params.publish)) return false;
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

export default function ModelInfo() {
  const { t } = useTranslation();
  const [modelInfos, setModelInfos] = useState([]);
  const [prices, setPrices] = useState([]);
  const [modelList, setModelList] = useState([]);
  const [channelsByModel, setChannelsByModel] = useState({});
  const [channelList, setChannelList] = useState([]);
  const [supplyRouting, setSupplyRouting] = useState(false);
  const [supplyCounts, setSupplyCounts] = useState({});
  // (channel, upstream name) pairs supplies send: those channel-listed names are not models of
  // their own while supplies route requests.
  const [suppliedUpstreams, setSuppliedUpstreams] = useState(null);
  const supplySwitches = useSupplySwitches();
  const [vendors, setVendors] = useState({});
  const [ownedby, setOwnedby] = useState([]);
  const [refreshFlag, setRefreshFlag] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(MODELINFO_FILTER_FIELDS, searchParams));
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('model_info'));
  const [keyword, setKeyword] = useState(() => searchParams.get('keyword') || '');
  const [status, setStatus] = useState(() => readStatus(searchParams));
  const [selectedKeys, setSelectedKeys] = useState(() => new Set());
  const [selectMode, setSelectMode] = useState(false);
  const [filterSheetOpen, setFilterSheetOpen] = useState(false);
  const [hiding, setHiding] = useState(false);
  const [unit, setUnit] = useState(() => loadPriceUnit());
  const columnState = useColumnVisibility({
    storageKey: COLUMN_STORAGE_KEY,
    legacyKeys: LEGACY_COLUMN_STORAGE_KEYS,
    columns: MODEL_COLUMNS
  });
  const columnVisibility = columnState.visibility;
  // Full table while it fits (the sidebar collapses first), phone list otherwise.
  const { layout, narrow } = useListLayout();

  const [sheetOpen, setSheetOpen] = useState(false);
  const [sheetTab, setSheetTab] = useState('profile');
  const [editRow, setEditRow] = useState(null);
  const [batchDelete, setBatchDelete] = useState(null);
  const [importOpen, setImportOpen] = useState(false);
  const [libraryOpen, setLibraryOpen] = useState(false);
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
          const [channels, supplies] = await Promise.all([fetchAllChannels(), listSupplies().catch(() => null)]);
          return { cres, catalog, priceRows, models, channels, supplies };
        },
        {
          onStart: () => setLoading(true),
          onResult: ({ cres, catalog, priceRows, models, channels, supplies }) => {
            setSupplyCounts(supplies?.success ? countSuppliesByModel(supplies.data || []) : {});
            setSuppliedUpstreams(supplies?.success ? suppliedUpstreamKeys(supplies.data || []) : null);
            if (!cres.data.success) showError(cres.data.message);
            setModelInfos(catalog);
            setPrices(priceRows);
            setModelList(models);
            setChannelList(channels);
            // While supplies route requests, a model without a catalog row has no supply and so
            // no channel, whatever the channels' model lists say.
            const bySupplies = cres.data.success && cres.data.supply_routing === true;
            setSupplyRouting(bySupplies);
            setChannelsByModel(bySupplies ? {} : indexChannelsByModel(channels));
          },
          onError: (error) => console.error(error),
          onFinally: () => {
            setLoading(false);
            setLoaded(true);
          }
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
        [...MODELINFO_FILTER_PARAM_KEYS, 'status', 'keyword', 'usable'].forEach((k) => params.delete(k));
        const q = filterStateToParams(MODELINFO_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        if (keyword) params.set('keyword', keyword);
        if (status !== 'all') params.set('status', status);
        return params;
      },
      { replace: true }
    );
  }, [filterState, keyword, status, setSearchParams]);

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
  };
  const clearEverything = () => {
    handleClearFilters();
    setKeyword('');
    setStatus('all');
  };
  const changeStatus = (next) => {
    setPage(0);
    setStatus(next);
  };

  const handleUnitChange = useCallback((next) => {
    setUnit(next);
    savePriceUnit(next);
  }, []);

  // Vendor options follow /api/model_ownedby/.
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
    () =>
      attachChannelDetails(
        mergeCatalogRows({ catalog: modelInfos, prices, modelList, channelsByModel, listedModels: channelModelNames(channelList, supplyRouting ? suppliedUpstreams : null) }),
        channelList,
        {
          supplyRouting
        }
      )
        .map((row) => (supplyRouting ? { ...row, supplyRouting } : row))
        .map((row) => (isPublicModelRow(row) ? { ...row, supplyCounts: supplyCounts[row.model] || { total: 0, usable: 0 } } : row)),
    [modelInfos, prices, modelList, channelsByModel, channelList, supplyCounts, supplyRouting, suppliedUpstreams]
  );

  // Local filtering: search box, segment and conditions.
  const params = useMemo(() => ({ ...filterStateToParams(MODELINFO_FILTER_FIELDS, filterState), status }), [filterState, status]);

  const stats = useMemo(() => catalogStats(rows), [rows]);
  const unlabeledCount = useMemo(() => modelInfos.filter(isUnlabeled).length, [modelInfos]);

  const filtered = useMemo(() => filterCatalogRows(rows, { keyword, params, aliasesByTarget }), [rows, keyword, params, aliasesByTarget]);

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

  // 新建与行编辑共用一个抽屉(信息 / 定价 / 渠道与分组),行内入口决定默认 Tab。
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

  // Deep link from elsewhere (channel panel, paused supplies): ?model=<name>&panel=<tab> opens
  // that model's panel once the rows are loaded, then the two keys are dropped from the URL.
  const linkedModel = searchParams.get('model');
  const linkedPanel = searchParams.get('panel');
  useEffect(() => {
    if (!linkedModel || loading || rows.length === 0) return;
    const row = rows.find((r) => r.model === linkedModel);
    if (row) openSheet(row, linkedPanel || 'profile');
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.delete('model');
        next.delete('panel');
        return next;
      },
      { replace: true }
    );
  }, [linkedModel, linkedPanel, loading, rows, openSheet, setSearchParams]);

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

  const presets = useFilterPresets({
    storageKey: PRESET_STORAGE_KEY,
    builtin: BUILTIN_PRESETS,
    fields: filterFields,
    state: filterState,
    status,
    statusLabel: (value) => t(value === 'all' ? 'modelsPage.stats.all' : `modelsPage.stats.${value}`),
    apply: (preset) => {
      setPage(0);
      setStatus(preset.status || 'all');
      setFilterState({ ...preset.conditions });
    }
  });
  const activeConditionFields = filterFields.filter((f) => isEntryActive(f, filterState[f.key]));
  const removeCondition = (key) =>
    handleFilterChange((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });

  const segmentOptions = STATUS_SEGMENTS.map((value) => ({ value, label: t(`modelsPage.stats.${value}`) }));
  const exitSelectMode = () => {
    setSelectMode(false);
    setSelectedKeys(new Set());
  };

  const batchActions = (
    <>
      <Button
        size="sm"
        variant="outline"
        className="h-7"
        disabled={hiding || selectedCatalogIds.length === 0}
        onClick={() => hide(selectedCatalogIds, false)}
      >
        <Eye className="size-3.5" />
        {t('modelsPage.batchShow')}
      </Button>
      <Button
        size="sm"
        variant="outline"
        className="h-7"
        disabled={hiding || selectedCatalogIds.length === 0}
        onClick={() => hide(selectedCatalogIds, true)}
      >
        <EyeOff className="size-3.5" />
        {t('modelsPage.batchHide')}
      </Button>
      <Button size="sm" variant="outline" className="h-7" disabled={selectedKeys.size === 0} onClick={openBatchPrice}>
        <DollarSign className="size-3.5" />
        {t('modelInfoPage.setPrice')}
      </Button>
      <SupplyBatchMenu
        rows={selectedRows}
        onDone={() => {
          setSelectedKeys(new Set());
          handleRefresh();
        }}
      />
      <Button
        size="sm"
        variant="outline"
        className="h-7 text-destructive"
        disabled={selectedPricedModels.length === 0}
        onClick={() => setBatchDelete('price')}
      >
        <Trash2 className="size-3.5" />
        {t('modelInfoPage.deletePrice')}
      </Button>
      <Button
        size="sm"
        variant="outline"
        className="h-7 text-destructive"
        disabled={selectedCatalogIds.length === 0}
        onClick={() => setBatchDelete('catalog')}
      >
        <Trash2 className="size-3.5" />
        {t('modelInfoPage.deleteCatalog')}
      </Button>
    </>
  );

  // The floating bar on phones has room for one action; the rest sit under ⋯.
  const phoneBatchActions = (
    <>
      <Button size="sm" variant="outline" className="h-8" disabled={selectedKeys.size === 0} onClick={openBatchPrice}>
        <DollarSign className="size-3.5" />
        {t('modelInfoPage.setPrice')}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            size="icon"
            variant="outline"
            className="size-8"
            disabled={selectedKeys.size === 0}
            aria-label={t('modelsPage.moreActions')}
          >
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem disabled={hiding || selectedCatalogIds.length === 0} onClick={() => hide(selectedCatalogIds, false)}>
            <Eye className="size-4" />
            {t('modelsPage.batchShow')}
          </DropdownMenuItem>
          <DropdownMenuItem disabled={hiding || selectedCatalogIds.length === 0} onClick={() => hide(selectedCatalogIds, true)}>
            <EyeOff className="size-4" />
            {t('modelsPage.batchHide')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            className="text-destructive focus:text-destructive"
            disabled={selectedPricedModels.length === 0}
            onClick={() => setBatchDelete('price')}
          >
            <Trash2 className="size-4" />
            {t('modelInfoPage.deletePrice')}
          </DropdownMenuItem>
          <DropdownMenuItem
            className="text-destructive focus:text-destructive"
            disabled={selectedCatalogIds.length === 0}
            onClick={() => setBatchDelete('catalog')}
          >
            <Trash2 className="size-4" />
            {t('modelInfoPage.deleteCatalog')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );

  // Nothing in the catalog yet: the whole page becomes the first-run empty state.
  const blank = loaded && rows.length === 0;
  let body;
  if (!loaded) {
    body = <ListSkeleton layout={layout} cells={[120, 80, 96, 64, 32]} />;
  } else if (filtered.length === 0) {
    body = <ListEmptyState variant="noMatch" onClearFilters={clearEverything} />;
  } else if (narrow) {
    body = (
      <ModelList
        items={pageRows}
        vendors={vendors}
        unit={unit}
        selectMode={selectMode}
        selectedKeys={selectedKeys}
        onToggleSelect={toggleSelect}
        onToggleHidden={toggleHidden}
        onOpen={openSheet}
        onDeleteCatalog={setDeleteTarget}
        onDeletePrice={setDeletePriceTarget}
      />
    );
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
        <Button variant="outline" size="sm" onClick={() => setLibraryOpen(true)} aria-label={t('modelLibrary.open')}>
          <Library className="size-4" />
          <span>{t('modelLibrary.open')}</span>
        </Button>
        <Button size="sm" onClick={openCreate} aria-label={t('modelInfoPage.create')}>
          <Plus className="size-4" />
          <span>{t('modelInfoPage.create')}</span>
        </Button>
      </PageActions>

      {blank ? (
        <PageEmptyState
          icon={Boxes}
          title={t('pageEmpty.models.title')}
          description={t('pageEmpty.models.description')}
          action={
            <>
              <Button variant="outline" onClick={openCreate}>
                {t('modelInfoPage.create')}
              </Button>
              <Button onClick={() => setLibraryOpen(true)}>{t('modelLibrary.open')}</Button>
            </>
          }
        />
      ) : (
        <>
          <div className={cn('space-y-3', narrow && selectMode && 'pb-20')}>
            <div className="space-y-2">
              {narrow ? (
                <>
                  <StatusSegments chips value={status} onChange={changeStatus} options={segmentOptions} counts={loaded ? stats : null} />
                  <PhoneListToolbar
                    search={{ value: keyword, onChange: handleKeywordChange, placeholder: t('modelsPage.searchShort') }}
                    filter={{ count: activeConditionFields.length, onOpen: () => setFilterSheetOpen(true) }}
                    select={{ active: selectMode, onToggle: () => (selectMode ? exitSelectMode() : setSelectMode(true)) }}
                    chips={activeConditionFields.map((f) => ({
                      key: f.key,
                      label: describeEntry(t, f, filterState[f.key]),
                      onRemove: () => removeCondition(f.key)
                    }))}
                  />
                </>
              ) : (
                <>
                  <div className="flex flex-wrap items-center gap-2">
                    <StatusSegments value={status} onChange={changeStatus} options={segmentOptions} counts={loaded ? stats : null} />
                    <div className="relative min-w-[12rem] flex-1">
                      <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        value={keyword}
                        onChange={(e) => handleKeywordChange(e.target.value)}
                        placeholder={t('modelsPage.searchPlaceholder')}
                        aria-label={t('modelsPage.searchShort')}
                        className="h-8 pl-8"
                      />
                    </div>
                    <ColumnPicker columns={columnState} />
                  </div>
                  {selectedKeys.size > 0 ? (
                    <BatchBar
                      count={selectedKeys.size}
                      onClear={() => setSelectedKeys(new Set())}
                      className="min-h-10 rounded-lg border border-primary/30 bg-primary/[0.04] px-3 py-1.5"
                      actions={batchActions}
                    />
                  ) : (
                    <ConditionBar
                      fields={filterFields}
                      state={filterState}
                      onChange={handleFilterChange}
                      onClearAll={handleClearFilters}
                      leadingLabel={t('modelsPage.showModelsThat')}
                      presets={presets}
                    />
                  )}
                </>
              )}
            </div>

            <Card>
              {body}
              <div className="border-t border-border">
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
              </div>
            </Card>
            {narrow && loaded && filtered.length > 0 && <ReadinessLegend />}
          </div>
        </>
      )}
      {narrow && selectMode && <BatchBar count={selectedKeys.size} onClear={exitSelectMode} floating actions={phoneBatchActions} />}
      {narrow && (
        <FilterSheet
          open={filterSheetOpen}
          onOpenChange={setFilterSheetOpen}
          title={t('listPage.filter')}
          fields={filterFields}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          t={t}
          top={<PresetChips presets={presets} onPick={() => setFilterSheetOpen(false)} />}
        />
      )}

      {params.publish && <SupplySwitchNote switches={supplySwitches} className="mt-2" />}

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
          liveRow={editRow ? rows.find((r) => r.model === editRow.model) || editRow : null}
          initialTab={sheetTab}
          existingModels={existingModels}
          prices={prices}
          ownedby={ownedby}
          unit={unit}
          channels={channelList}
          supplyCount={editRow ? supplyCounts[editRow.model]?.total || 0 : undefined}
          switches={supplySwitches}
          supplyRouting={supplyRouting}
          onSaved={onSaved}
          onSupplyChanged={handleRefresh}
        />

        <LibrarySheet open={libraryOpen} onOpenChange={setLibraryOpen} vendors={vendors} onAdded={handleRefresh} />
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
