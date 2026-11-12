import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Search, X, LayoutGrid, List, Users, Check, Minus, Columns3, Copy, SlidersHorizontal, ChevronDown } from 'lucide-react';
import FilterDropdown from './FilterDropdown';

import { cn } from '@/lib/utils';
import { FloatingPanel } from '@/components/ui/floating';
import BrandIcon from '@/components/brand/BrandIcon';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { toast } from '@/components/ui/sonner';
import { FilterBar, isEntryActive, OP_NOT_IN } from '@/components/filter-bar';
import { API } from 'utils/api';
import { showError, ValueFormatter } from 'utils/common';
import { MODALITY_OPTIONS } from 'constants/Modality';
import ModelCard from './ModelCard';
import ModelDetailModal from './ModelDetailModal';
import { buildModelFilterFields, buildModalityOptions, buildCapabilityOptions, CONTEXT_BUCKETS, PRICE_BUCKETS } from './modelFilterFields';
import { Switch } from '@/components/ui/switch';
import { ListEmptyState, ListFooter, useListLayout } from '@/components/list-page';
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';

// 模态 Tab 的动词文案兜底值（i18n 缺 key 时使用），与 modelpricePage.modalityTab 对应。
const MODALITY_TAB_FALLBACK = {
  text: 'Chat',
  image: 'Image Generation',
  audio: 'Audio Output',
  video: 'Video Generation',
  music: 'Music Generation',
  file: 'File',
  speech: 'Text to Speech',
  transcription: 'Speech to Text'
};

const copyText = (text, label) => {
  try {
    navigator.clipboard.writeText(text);
    toast.success(`${label} \u2713`);
  } catch (e) {
    toast.error(`${label}: ${text}`);
  }
};

function ModalityTab({ active, onClick, count, children }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'inline-flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium transition-colors',
        active ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'
      )}
    >
      {children}
      <span
        className={cn(
          'rounded-full px-1.5 py-0.5 text-[0.6875rem] font-medium tabular-nums',
          active ? 'bg-muted text-foreground' : 'bg-muted text-muted-foreground'
        )}
      >
        {count}
      </span>
    </button>
  );
}

ModalityTab.propTypes = {
  active: PropTypes.bool,
  onClick: PropTypes.func,
  count: PropTypes.number,
  children: PropTypes.node
};

// 「显示」合并 popover：把「怎么看」类控件（排序 / 视图 / 单位）收进一个触发按钮。
// 逻辑与状态完全复用上层，仅做 UI 重组。选择排序后关闭；视图/单位切换保持打开便于连续调整。
function DisplayMenu({ sortBy, onSortChange, viewMode, onViewChange, unit, onUnitChange, onlyAvailable, onOnlyAvailableChange, t }) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef(null);
  const panelRef = useRef(null);

  useEffect(() => {
    if (!open) return;
    const onDocClick = (e) => {
      if (!rootRef.current?.contains(e.target) && !panelRef.current?.contains(e.target)) setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const sortOptions = [
    { value: 'provider', label: t('modelpricePage.sortProvider') },
    { value: 'name', label: t('modelpricePage.sortName') },
    { value: 'price-asc', label: t('modelpricePage.sortPriceAsc') },
    { value: 'price-desc', label: t('modelpricePage.sortPriceDesc') }
  ];

  const sectionLabel = 'px-1 pb-1 text-xs font-medium text-muted-foreground';

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-label={t('modelpricePage.display')}
        aria-expanded={open}
        className={cn(
          'inline-flex h-9 items-center gap-1.5 rounded-md border px-2.5 text-[0.8125rem] font-medium transition-colors',
          open
            ? 'border-foreground/40 bg-muted text-foreground shadow-sm'
            : 'border-input bg-field text-foreground hover:border-foreground/30'
        )}
      >
        <SlidersHorizontal className="size-4 shrink-0" />
        {/* Icon only on phones, where the toolbar is one row. */}
        <span className="hidden sm:inline">{t('modelpricePage.display')}</span>
        <ChevronDown className="hidden size-4 shrink-0 opacity-50 sm:block" />
      </button>
      <FloatingPanel
        ref={panelRef}
        open={open}
        anchorRef={rootRef}
        align="end"
        className="w-56 space-y-3 rounded-md border border-border bg-card p-2 text-card-foreground shadow-md"
      >
        {/* Only the models the selected group can call (on by default). */}
        <label className="flex cursor-pointer items-center justify-between gap-2 px-1 text-sm">
          {t('modelpricePage.onlyAvailable')}
          <Switch checked={onlyAvailable} onCheckedChange={onOnlyAvailableChange} />
        </label>
        {/* 排序：单选列表，当前项打勾。 */}
        <div>
          <div className={sectionLabel}>{t('modelpricePage.sortBy')}</div>
          <div className="space-y-0.5">
            {sortOptions.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => {
                  onSortChange(opt.value);
                  setOpen(false);
                }}
                className={cn(
                  'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-muted',
                  sortBy === opt.value && 'bg-muted font-medium'
                )}
              >
                <Check className={cn('size-4 shrink-0', sortBy === opt.value ? 'opacity-100' : 'opacity-0')} />
                <span className="truncate">{opt.label}</span>
              </button>
            ))}
          </div>
        </div>
        {/* 视图：卡片 / 表格 两段式切换（不关闭 popover）。 */}
        <div>
          <div className={sectionLabel}>{t('modelpricePage.viewMode')}</div>
          <div className="inline-flex w-full overflow-hidden rounded-md border border-border">
            <button
              type="button"
              onClick={() => onViewChange('card')}
              className={cn(
                'flex flex-1 items-center justify-center gap-1.5 py-1.5 text-sm font-medium',
                viewMode === 'card' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted'
              )}
            >
              <LayoutGrid className="size-4" />
              {t('modelpricePage.viewCard')}
            </button>
            <button
              type="button"
              onClick={() => onViewChange('list')}
              className={cn(
                'flex flex-1 items-center justify-center gap-1.5 border-l border-border py-1.5 text-sm font-medium',
                viewMode === 'list' ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted'
              )}
            >
              <List className="size-4" />
              {t('modelpricePage.viewList')}
            </button>
          </div>
        </div>
        {/* 单位：K / M 两段式切换（不关闭 popover）。 */}
        <div>
          <div className={sectionLabel}>{t('modelpricePage.unit')}</div>
          <div className="inline-flex w-full overflow-hidden rounded-md border border-border">
            {['K', 'M'].map((u) => (
              <button
                key={u}
                type="button"
                onClick={() => onUnitChange(u)}
                className={cn(
                  'flex-1 py-1.5 text-sm font-medium',
                  unit === u ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted',
                  u === 'M' && 'border-l border-border'
                )}
              >
                {u}
              </button>
            ))}
          </div>
        </div>
      </FloatingPanel>
    </div>
  );
}

DisplayMenu.propTypes = {
  sortBy: PropTypes.string,
  onSortChange: PropTypes.func,
  viewMode: PropTypes.string,
  onViewChange: PropTypes.func,
  unit: PropTypes.string,
  onUnitChange: PropTypes.func,
  onlyAvailable: PropTypes.bool,
  onOnlyAvailableChange: PropTypes.func,
  t: PropTypes.func
};

// 表格视图列注册表（对齐 Log 页的「列」勾选菜单）。id 用于列显隐判断。
// One price column for the selected group ("input / output"); the billing type only shows when a
// model is not billed per token, and the whole row opens the details.
const MODEL_PRICE_COLUMNS = [
  { id: 'model', labelKey: 'modelpricePage.modelName' },
  { id: 'provider', labelKey: 'modelpricePage.provider' },
  { id: 'price', labelKey: 'modelpricePage.price' }
];

const COLUMN_VISIBILITY_STORAGE_KEY = 'modelprice-column-visibility-v2';

const buildDefaultColumnVisibility = () => Object.fromEntries(MODEL_PRICE_COLUMNS.map((c) => [c.id, true]));

// 未知/缺失的键回落为「显示」；storage 损坏或不可用时全部默认显示。
const loadColumnVisibility = () => {
  const defaults = buildDefaultColumnVisibility();
  try {
    const saved = JSON.parse(localStorage.getItem(COLUMN_VISIBILITY_STORAGE_KEY));
    if (!saved || typeof saved !== 'object') return defaults;
    for (const id of Object.keys(defaults)) {
      if (typeof saved[id] === 'boolean') defaults[id] = saved[id];
    }
  } catch (e) {
    /* corrupted storage -> defaults */
  }
  return defaults;
};

// 勾选行：DropdownMenuItem 点击即关闭菜单会打断连续勾选，故用普通 button；
// 勾选块用 span（避免 button 嵌 button）。
function ColumnToggleItem({ checked, indeterminate = false, label, onToggle }) {
  return (
    <button
      type="button"
      role="menuitemcheckbox"
      aria-checked={indeterminate ? 'mixed' : checked}
      onClick={onToggle}
      className="flex w-full cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-muted focus:bg-muted"
    >
      <span
        className={cn(
          'flex size-4 shrink-0 items-center justify-center rounded-sm border border-input',
          (checked || indeterminate) && 'border-primary bg-primary text-primary-foreground'
        )}
      >
        {indeterminate ? <Minus className="size-3.5" /> : checked ? <Check className="size-3.5" /> : null}
      </span>
      <span className="truncate">{label}</span>
    </button>
  );
}

ColumnToggleItem.propTypes = {
  checked: PropTypes.bool,
  indeterminate: PropTypes.bool,
  label: PropTypes.node,
  onToggle: PropTypes.func.isRequired
};

const parseJson = (str) => {
  try {
    return JSON.parse(str || '[]');
  } catch (e) {
    return [];
  }
};

// FilterBar 客户端过滤：对单个模型求值一个 enum 条目。多选 = OR 并集；排除 = NOT IN。
// test(value) 返回该模型是否命中某候选值；条目未启用（无值）时返回 true（不约束）。
const matchEnumEntry = (entry, test) => {
  if (!entry || !Array.isArray(entry.values) || entry.values.length === 0) return true;
  const hit = entry.values.some((v) => test(v));
  return entry.op === OP_NOT_IN ? !hit : hit;
};

// initialModality:外部预选模态（text|image|audio|video），仅决定模态 Tab 的初始值，
// 用户仍可自由切换；未传时保持原来的「全部」。
// 注意：模态 Tab 的语义是「输出模态」（模型能产出什么），与漏斗里的输入/输出模态筛选相互独立。
export default function ModelPrice({ embedded = false, initialModality }) {
  const { t } = useTranslation();
  const { narrow } = useListLayout();
  const ownedby = useSelector((state) => state.siteInfo?.ownedby);

  const [availableModels, setAvailableModels] = useState({});
  const [modelInfoMap, setModelInfoMap] = useState({});
  const [userGroupMap, setUserGroupMap] = useState({});
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedGroup, setSelectedGroup] = useState('');
  const [unit, setUnit] = useState('M');
  const [onlyShowAvailable, setOnlyShowAvailable] = useState(true);
  const [selectedModality, setSelectedModality] = useState(initialModality && MODALITY_OPTIONS[initialModality] ? initialModality : 'all');
  // 漏斗筛选受控状态（owned_by/tags/context/price）：{ [key]: { op, values } }，仅客户端过滤用。
  const [filterState, setFilterState] = useState({});
  const [sortBy, setSortBy] = useState('provider');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  // 默认表格（重数据定价页更利于扫读/比价）；记住用户的切换选择。
  const [viewMode, setViewMode] = useState(() => {
    try {
      return localStorage.getItem('modelprice_view') || 'list';
    } catch (e) {
      return 'list';
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem('modelprice_view', viewMode);
    } catch (e) {
      // ignore storage errors (e.g. storage disabled / private mode)
    }
  }, [viewMode]);
  const [detailModalOpen, setDetailModalOpen] = useState(false);
  const [selectedModelDetail, setSelectedModelDetail] = useState(null);
  // 表格视图列显隐（localStorage 持久化）。
  const [columnVisibility, setColumnVisibility] = useState(loadColumnVisibility);
  useEffect(() => {
    try {
      localStorage.setItem(COLUMN_VISIBILITY_STORAGE_KEY, JSON.stringify(columnVisibility));
    } catch (e) {
      /* storage unavailable -> keep in-memory only */
    }
  }, [columnVisibility]);
  const allColumnsVisible = MODEL_PRICE_COLUMNS.every((c) => columnVisibility[c.id]);
  const someColumnsVisible = MODEL_PRICE_COLUMNS.some((c) => columnVisibility[c.id]);
  const toggleColumn = (id) => setColumnVisibility((v) => ({ ...v, [id]: !v[id] }));
  const toggleAllColumns = () =>
    setColumnVisibility((v) => {
      const next = { ...v };
      MODEL_PRICE_COLUMNS.forEach((c) => {
        next[c.id] = !allColumnsVisible;
      });
      return next;
    });

  const pageSizeOptions = [20, 30, 60, 100];

  const fetchAvailableModels = useCallback(async () => {
    try {
      const res = await API.get('/api/available_model');
      const { success, message, data } = res.data;
      if (success) setAvailableModels(data);
      else showError(message);
    } catch (error) {
      console.error(error);
    }
  }, []);

  const fetchModelInfo = useCallback(async () => {
    try {
      const res = await API.get('/api/model_info/');
      const { success, message, data } = res.data;
      if (success) {
        const infoMap = {};
        data.forEach((info) => {
          infoMap[info.model] = info;
        });
        setModelInfoMap(infoMap);
      } else showError(message);
    } catch (error) {
      console.error(error);
    }
  }, []);

  const fetchUserGroupMap = useCallback(async () => {
    try {
      const res = await API.get('/api/user_group_map');
      const { success, message, data } = res.data;
      if (success) {
        setUserGroupMap(data);
        const firstKey = Object.entries(data)
          .filter(([, g]) => !g.inaccessible)
          .sort(([, a], [, b]) => (a.ratio ?? 0) - (b.ratio ?? 0))[0]?.[0];
        setSelectedGroup(firstKey);
      } else showError(message);
    } catch (error) {
      console.error(error);
    }
  }, []);

  const sortedUserGroupEntries = useMemo(
    () =>
      Object.entries(userGroupMap)
        .filter(([, group]) => !group.inaccessible)
        .sort(([, a], [, b]) => (a.ratio ?? 0) - (b.ratio ?? 0)),
    [userGroupMap]
  );

  useEffect(() => {
    fetchAvailableModels();
    fetchModelInfo();
    fetchUserGroupMap();
  }, [fetchAvailableModels, fetchModelInfo, fetchUserGroupMap]);

  const allTags = [...new Set(Object.values(modelInfoMap).flatMap((info) => parseJson(info.tags)))];
  // 模态候选项取自真实数据（无数据的模态不出现在漏斗里）。
  const presentInputModalities = [...new Set(Object.values(modelInfoMap).flatMap((info) => parseJson(info.input_modalities)))];
  const presentOutputModalities = [...new Set(Object.values(modelInfoMap).flatMap((info) => parseJson(info.output_modalities)))];
  // 能力候选项同理：capabilities 为空串 / 缺失时解析为空数组，全无数据时该维度隐藏。
  const presentCapabilities = [...new Set(Object.values(modelInfoMap).flatMap((info) => parseJson(info.capabilities)))];

  // 模态 Tab 计数口径：全量（对照 OpenRouter，不随其他筛选变化）。
  // all = 全部模型数；各模态 = 输出模态命中该模态的模型数（Tab 只按输出模态归类）。
  const modalityCounts = useMemo(() => {
    const counts = { all: 0 };
    Object.keys(MODALITY_OPTIONS).forEach((k) => {
      counts[k] = 0;
    });
    Object.keys(availableModels).forEach((modelName) => {
      counts.all += 1;
      const info = modelInfoMap[modelName];
      if (!info) return;
      const mods = new Set(parseJson(info.output_modalities));
      mods.forEach((m) => {
        if (counts[m] !== undefined) counts[m] += 1;
      });
    });
    return counts;
  }, [availableModels, modelInfoMap]);

  const formatPrice = (value, type) => {
    if (typeof value === 'number') {
      let nowUnit = '';
      let isM = unit === 'M';
      if (type === 'times') isM = false;
      if (type === 'tokens') nowUnit = `/ 1${unit}`;
      return ValueFormatter(value, true, isM) + nowUnit;
    }
    return value;
  };

  const filteredModels = useMemo(() => {
    return Object.entries(availableModels)
      .filter(([modelName, model]) => {
        const modelInfo = modelInfoMap[modelName];
        // 供应商（漏斗，多选 + 排除）。
        if (!matchEnumEntry(filterState.owned_by, (v) => model.owned_by === v)) return false;
        // 只显示可用（一等公民，常驻工具栏）。
        if (onlyShowAvailable && !model.groups.includes(selectedGroup)) return false;
        if (searchQuery) {
          const query = searchQuery.toLowerCase();
          const matchModel = modelName.toLowerCase().includes(query);
          const matchDescription = modelInfo?.description?.toLowerCase().includes(query);
          if (!matchModel && !matchDescription) return false;
        }
        // 模态 Tab = 范围导航（非漏斗），只按输出模态归类。
        if (selectedModality !== 'all') {
          if (!modelInfo) return false;
          if (!parseJson(modelInfo.output_modalities).includes(selectedModality)) return false;
        }
        // 标签（漏斗，多选 + 排除）：命中 = 模型含该标签。
        if (!matchEnumEntry(filterState.tags, (v) => parseJson(modelInfo?.tags).includes(v))) return false;
        // 输入/输出模态（漏斗，多选 + 排除）：命中 = 模型该方向模态列表含该值。
        if (!matchEnumEntry(filterState.input_modalities, (v) => parseJson(modelInfo?.input_modalities).includes(v))) return false;
        if (!matchEnumEntry(filterState.output_modalities, (v) => parseJson(modelInfo?.output_modalities).includes(v))) return false;
        // 能力（漏斗，多选 + 排除）：命中 = 模型能力列表含该值。
        if (!matchEnumEntry(filterState.capabilities, (v) => parseJson(modelInfo?.capabilities).includes(v))) return false;
        // 上下文长度（漏斗，多选 + 排除）：命中 = 上下文落在该分桶。
        if (
          !matchEnumEntry(filterState.context, (v) => {
            const ctx = modelInfo?.context_length;
            if (!ctx || ctx <= 0) return false;
            const bucket = CONTEXT_BUCKETS.find((b) => b.key === v);
            return bucket ? bucket.match(ctx) : false;
          })
        )
          return false;
        // 价格分桶（漏斗，多选 + 排除）：按所选分组 ratio 折算输入价，仅 tokens 类型可命中。
        if (
          !matchEnumEntry(filterState.price, (v) => {
            const grp = userGroupMap[selectedGroup];
            const hasAccess = model.groups.includes(selectedGroup);
            if (!hasAccess || !grp || model.price.type !== 'tokens') return false;
            const usdPerM = grp.ratio * model.price.input * 2;
            const bucket = PRICE_BUCKETS.find((b) => b.key === v);
            return bucket ? bucket.match(usdPerM) : false;
          })
        )
          return false;
        return true;
      })
      .map(([modelName, model]) => {
        const group = userGroupMap[selectedGroup];
        const hasAccess = model.groups.includes(selectedGroup);
        const price = hasAccess
          ? { input: group.ratio * model.price.input, output: group.ratio * model.price.output, unconfigured: model.price.unconfigured }
          : { input: t('modelpricePage.noneGroup'), output: t('modelpricePage.noneGroup') };
        const allGroupPrices = sortedUserGroupEntries
          .filter(([key]) => model.groups.includes(key))
          .map(([key, grp]) => ({
            groupName: grp.name,
            groupKey: key,
            input: grp.ratio * model.price.input,
            output: grp.ratio * model.price.output,
            type: model.price.type,
            ratio: grp.ratio,
            unconfigured: model.price.unconfigured,
            extraRatios: model.price.extra_ratios
              ? Object.fromEntries(Object.entries(model.price.extra_ratios).map(([k, v]) => [k, (grp.ratio * v).toFixed(6)]))
              : null
          }));
        return {
          model: modelName,
          provider: model.owned_by,
          modelInfo: modelInfoMap[modelName],
          price,
          group: hasAccess ? group : null,
          type: model.price.type,
          routing: model.routing || null,
          priceData: { price: model.price, allGroupPrices }
        };
      })
      .sort((a, b) => {
        if (sortBy === 'name') return a.model.localeCompare(b.model);
        if (sortBy === 'price-asc' || sortBy === 'price-desc') {
          // Non-numeric prices (no group access / unconfigured) sink to the bottom.
          const pa = typeof a.price.input === 'number' ? a.price.input : Infinity;
          const pb = typeof b.price.input === 'number' ? b.price.input : Infinity;
          if (pa !== pb) return sortBy === 'price-asc' ? pa - pb : pb - pa;
          return a.model.localeCompare(b.model);
        }
        // 'provider' (default): group by owner id, then model name within a provider.
        const ownerA = ownedby?.find((item) => item.name === a.provider);
        const ownerB = ownedby?.find((item) => item.name === b.provider);
        const d = (ownerA?.id || 0) - (ownerB?.id || 0);
        return d !== 0 ? d : a.model.localeCompare(b.model);
      });
  }, [
    availableModels,
    filterState,
    onlyShowAvailable,
    selectedGroup,
    searchQuery,
    modelInfoMap,
    selectedModality,
    sortBy,
    userGroupMap,
    sortedUserGroupEntries,
    ownedby,
    t
  ]);

  const paginatedModels = useMemo(() => {
    const startIndex = (page - 1) * pageSize;
    return filteredModels.slice(startIndex, startIndex + pageSize);
  }, [filteredModels, page, pageSize]);

  useEffect(() => {
    setPage(1);
  }, [filterState, selectedGroup, searchQuery, selectedModality, sortBy, onlyShowAvailable, pageSize]);

  const uniqueOwnedBy = [
    'all',
    ...[...new Set(Object.values(availableModels).map((model) => model.owned_by))].sort((a, b) => {
      const ownerA = ownedby?.find((item) => item.name === a);
      const ownerB = ownedby?.find((item) => item.name === b);
      return (ownerA?.id || 0) - (ownerB?.id || 0);
    })
  ];

  const getIconByName = (name) => {
    if (name === 'all') return null;
    const owner = ownedby?.find((item) => item.name === name);
    return owner?.icon;
  };

  const renderProviderIcon = (name) => <BrandIcon icon={getIconByName(name)} ownedBy={name} fallbackText={name} className="size-4" />;

  // 漏斗筛选候选项（不含「全部」；多选语义下空 = 不约束）。供应商带厂商 icon。
  const providerOptions = uniqueOwnedBy.filter((ob) => ob !== 'all').map((ob) => ({ value: ob, label: ob, icon: renderProviderIcon(ob) }));
  const tagOptions = allTags.map((tag) => ({ value: tag, label: tag }));

  // FilterBar 字段定义（无标签时自动隐藏 tags 维度）与当前活跃的漏斗字段（驱动 chips 行的条件渲染）。
  const filterFields = buildModelFilterFields({
    t,
    providerOptions,
    tagOptions,
    inputModalityOptions: buildModalityOptions(t, presentInputModalities),
    outputModalityOptions: buildModalityOptions(t, presentOutputModalities),
    capabilityOptions: buildCapabilityOptions(t, presentCapabilities)
  });
  const activeChipFields = filterFields.filter((f) => isEntryActive(f, filterState[f.key]));

  const groupOptions = sortedUserGroupEntries.map(([key, group]) => ({
    value: key,
    label: group.name,
    badge: (
      <span className="ml-auto rounded-sm bg-muted px-1 py-0.5 text-[0.6875rem] font-medium tabular-nums text-muted-foreground">
        {group.ratio > 0 ? `x${group.ratio}` : t('modelpricePage.free')}
      </span>
    )
  }));

  const handleViewDetail = (modelData) => {
    setSelectedModelDetail(modelData);
    setDetailModalOpen(true);
  };

  const handleCloseDetail = () => {
    setDetailModalOpen(false);
    setSelectedModelDetail(null);
  };

  // 漏斗筛选受控回调：setFilterState 兼容对象或函数式更新（FilterBar 内部走函数式）。
  const handleFilterChange = (next) => setFilterState(next);
  // 「清除全部」只清漏斗 4 维 + 搜索词；不动模态 Tab / 分组 / 排序 / 单位 / 视图。
  const handleClearFilters = () => {
    setSearchQuery('');
    setFilterState({});
  };

  const footer = (
    <ListFooter
      total={filteredModels.length}
      page={page - 1}
      pageSize={pageSize}
      pageSizeOptions={pageSizeOptions}
      onPageChange={(p) => setPage(p + 1)}
      onPageSizeChange={(size) => {
        setPageSize(size);
        setPage(1);
      }}
    />
  );

  // The selected group's price as "input / output"; per-call models say so after the figure.
  const priceText = (row) => {
    if (row.routing) return t('modelpricePage.routing.billedByTarget', { model: row.routing.fallback_model });
    const { input, output } = row.price;
    if (typeof input !== 'number') return input;
    if (row.type !== 'tokens')
      return `${input > 0 ? ValueFormatter(input, true, false) : t('modelpricePage.free')} · ${t('modelpricePage.times')}`;
    if (input <= 0 && output <= 0) return t('modelpricePage.free');
    return `${ValueFormatter(input, true, unit === 'M')} / ${ValueFormatter(output, true, unit === 'M')}`;
  };
  const groupRatio = userGroupMap[selectedGroup]?.ratio;
  const priceHeader = `${t('modelpricePage.price')} / 1${unit}`;
  const copyButton = (name) => (
    <RowControl>
      <button
        type="button"
        onClick={() => copyText(name, t('modelpricePage.modelName'))}
        aria-label={t('modelpricePage.copyModelId')}
        className="text-muted-foreground opacity-0 transition-opacity hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100"
      >
        <Copy className="size-3.5" />
      </button>
    </RowControl>
  );

  return (
    <div className={cn('space-y-4', !embedded && 'p-6')}>
      <div className="space-y-3">
        {/* One toolbar row: search, the group (its price multiplier in the menu), filters, the display
            menu (availability, sort, view, unit) and, for the table, the column picker. On phones the
            buttons drop their labels so the row still fits. */}
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          {!embedded && <h1 className="min-w-0 truncate text-2xl font-semibold tracking-tight">{t('modelpricePage.availableModels')}</h1>}
          <div className={cn('flex items-center gap-2', embedded ? 'w-full' : 'lg:ml-auto lg:justify-end')}>
            <div className={cn('relative min-w-0 flex-1', !embedded && 'sm:w-60 sm:flex-none')}>
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-9 pr-9"
                placeholder={t('modelpricePage.search')}
                aria-label={t('modelpricePage.search')}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => setSearchQuery('')}
                  aria-label={t('modelpricePage.clearFilters')}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  <X className="size-4" />
                </button>
              )}
            </div>
            <div className="shrink-0" title={t('priceList.groupHint')}>
              <FilterDropdown
                icon={Users}
                label={t('modelpricePage.group')}
                value={selectedGroup}
                options={groupOptions}
                onSelect={setSelectedGroup}
                active
              />
            </div>
            <div className="shrink-0">
              <FilterBar
                fields={filterFields}
                state={filterState}
                onChange={handleFilterChange}
                onClearAll={handleClearFilters}
                hideChips
                t={t}
              />
            </div>
            <div className="shrink-0">
              <DisplayMenu
                sortBy={sortBy}
                onSortChange={setSortBy}
                viewMode={viewMode}
                onViewChange={setViewMode}
                unit={unit}
                onUnitChange={setUnit}
                onlyAvailable={onlyShowAvailable}
                onOnlyAvailableChange={setOnlyShowAvailable}
                t={t}
              />
            </div>
            {/* 列显示：仅表格视图有意义，卡片视图与手机行下不渲染。选择持久化到 localStorage。 */}
            {viewMode === 'list' && !narrow && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="outline"
                    className="h-9 shrink-0 gap-1.5 px-2.5 text-[0.8125rem] font-medium"
                    aria-label={t('modelpricePage.selectColumns')}
                  >
                    <Columns3 className="size-4 shrink-0" />
                    <span>{t('modelpricePage.columns')}</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="max-h-[70vh] w-52 overflow-y-auto">
                  <DropdownMenuLabel>{t('modelpricePage.selectColumns')}</DropdownMenuLabel>
                  <ColumnToggleItem
                    checked={allColumnsVisible}
                    indeterminate={!allColumnsVisible && someColumnsVisible}
                    label={t('modelpricePage.columnSelectAll')}
                    onToggle={toggleAllColumns}
                  />
                  <DropdownMenuSeparator />
                  {MODEL_PRICE_COLUMNS.map((c) => (
                    <ColumnToggleItem
                      key={c.id}
                      checked={!!columnVisibility[c.id]}
                      label={t(c.labelKey)}
                      onToggle={() => toggleColumn(c.id)}
                    />
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </div>
        </div>

        {/* 模态 Tabs：范围导航（非筛选），窄屏可横滑；右侧不放控件。 */}
        <div className="border-b border-border">
          <div className="no-scrollbar flex items-center gap-1 overflow-x-auto">
            <ModalityTab active={selectedModality === 'all'} onClick={() => setSelectedModality('all')} count={modalityCounts.all}>
              {t('modelpricePage.allModality')}
            </ModalityTab>
            {/* 计数为 0 的模态默认隐藏；但当前选中的模态始终渲染，避免 initialModality 预选后看不到过滤态。 */}
            {Object.entries(MODALITY_OPTIONS).map(
              ([key, option]) =>
                (modalityCounts[key] > 0 || selectedModality === key) && (
                  <ModalityTab
                    key={key}
                    active={selectedModality === key}
                    onClick={() => setSelectedModality(key)}
                    count={modalityCounts[key]}
                  >
                    {t(`modelpricePage.modalityTab.${key}`, { defaultValue: MODALITY_TAB_FALLBACK[key] || option.text })}
                  </ModalityTab>
                )
            )}
          </div>
        </div>

        {/* chips 行：仅漏斗筛选活跃时出现，独立成行（chips + 「+」追加 + 右侧「清除全部」）。 */}
        {activeChipFields.length > 0 && (
          <FilterBar
            fields={filterFields}
            state={filterState}
            onChange={handleFilterChange}
            onClearAll={handleClearFilters}
            triggerVariant="plus"
            clearAlignEnd
            t={t}
            className="w-full rounded-lg border border-input bg-field px-3 py-2"
          />
        )}
      </div>

      {filteredModels.length > 0 ? (
        viewMode === 'card' ? (
          <div className="space-y-3">
            <div className="flex flex-col gap-3">
              {paginatedModels.map((model) => (
                <ModelCard
                  key={model.model}
                  model={model.model}
                  provider={model.provider}
                  modelInfo={model.modelInfo}
                  price={model.price}
                  group={model.group}
                  ownedbyIcon={getIconByName(model.provider)}
                  unit={unit}
                  type={model.type}
                  formatPrice={formatPrice}
                  onViewDetail={() => handleViewDetail(model)}
                  routing={model.routing}
                />
              ))}
            </div>
            <Card className="overflow-hidden">{footer}</Card>
          </div>
        ) : (
          <Card className="overflow-hidden">
            {narrow ? (
              // Phone rows: the name and price on top, the vendor below; the row opens the details.
              <ul className="divide-y divide-border">
                {paginatedModels.map((model) => (
                  <li
                    key={model.model}
                    className={cn('flex h-14 flex-col justify-center px-4', ROW_OPENS)}
                    {...rowOpenProps(() => handleViewDetail(model))}
                  >
                    <div className="flex items-baseline gap-3 text-sm">
                      <span className="min-w-0 truncate font-medium">{model.model}</span>
                      {/* A routed model shows its fallback in short; the full note is the tooltip. */}
                      <span
                        className={cn('ml-auto shrink-0 tabular-nums', model.routing && 'text-muted-foreground')}
                        title={model.routing ? priceText(model) : undefined}
                      >
                        {model.routing ? `→ ${model.routing.fallback_model}` : priceText(model)}
                      </span>
                    </div>
                    <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                      {renderProviderIcon(model.provider)}
                      <span className="truncate">{model.provider}</span>
                    </div>
                  </li>
                ))}
              </ul>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    {columnVisibility.model && <TableHead>{t('modelpricePage.modelName')}</TableHead>}
                    {columnVisibility.provider && <TableHead>{t('modelpricePage.provider')}</TableHead>}
                    {columnVisibility.price && (
                      <TableHead className="text-right" title={groupRatio != null ? `x${groupRatio}` : undefined}>
                        {priceHeader}
                      </TableHead>
                    )}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {paginatedModels.map((model) => (
                    <TableRow key={model.model} className={cn('group', ROW_OPENS)} {...rowOpenProps(() => handleViewDetail(model))}>
                      {columnVisibility.model && (
                        <TableCell>
                          <div className="flex items-center gap-1.5">
                            <span className="truncate font-medium">{model.model}</span>
                            {copyButton(model.model)}
                          </div>
                        </TableCell>
                      )}
                      {columnVisibility.provider && (
                        <TableCell>
                          <div className="flex items-center gap-1.5">
                            {renderProviderIcon(model.provider)}
                            <span>{model.provider}</span>
                          </div>
                        </TableCell>
                      )}
                      {columnVisibility.price && (
                        <TableCell
                          className={cn(
                            'whitespace-nowrap text-right tabular-nums',
                            (model.routing || typeof model.price.input !== 'number') && 'text-muted-foreground'
                          )}
                        >
                          {priceText(model)}
                        </TableCell>
                      )}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            <div className="border-t border-border">{footer}</div>
          </Card>
        )
      ) : (
        <Card>
          <ListEmptyState
            variant="noMatch"
            title={t('modelpricePage.noModelsFound')}
            description={t('modelpricePage.noModelsFoundTip')}
            onClearFilters={handleClearFilters}
          />
        </Card>
      )}

      <ModelDetailModal
        open={detailModalOpen}
        onClose={handleCloseDetail}
        model={selectedModelDetail?.model}
        provider={selectedModelDetail?.provider}
        modelInfo={selectedModelDetail?.modelInfo}
        priceData={selectedModelDetail?.priceData}
        routing={selectedModelDetail?.routing}
        ownedbyIcon={selectedModelDetail ? getIconByName(selectedModelDetail.provider) : null}
        userGroupMap={userGroupMap}
        formatPrice={formatPrice}
        unit={unit}
      />
    </div>
  );
}

ModelPrice.propTypes = {
  embedded: PropTypes.bool,
  initialModality: PropTypes.oneOf(Object.keys(MODALITY_OPTIONS))
};
