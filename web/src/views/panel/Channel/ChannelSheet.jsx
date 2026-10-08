import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useForm, Controller } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Loader2, CloudDownload, Eraser, AlertTriangle, KeyRound } from 'lucide-react';

import { API } from 'utils/api';
import { trims } from 'utils/common';
import { toast } from '@/components/ui/sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { Checkbox } from '@/components/ui/checkbox';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Combobox } from '@/components/ui/combobox';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { CHANNEL_OPTIONS_ORDERED } from 'constants/ChannelConstants';
import { configText } from 'i18n/configText';
import { defaultConfig, typeConfig } from './typeConfig';
import { syncChannelPricing, fetchUpstreamModelList } from './channelApi';
import ChannelOAuth from './ChannelOAuth';
import ModelHeadersInput from './ModelHeadersInput';
import ModelListInput from './ModelListInput';
import ModelMappingInput from './ModelMappingInput';
import ModelSelectorModal from './ModelSelectorModal';
import PluginForm from './PluginForm';
import pluginConfig from './pluginConfig';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { BrandIconRefreshButton } from '@/components/brand/BrandIconPicker';
import { iconCacheHost } from '@/components/brand/brandIconManifest';

const EMPTY = {
  name: '',
  type: 1,
  // 携带 status 往返，避免编辑保存时被后端 Select("*") 覆写成 0(未知)；新建默认启用。
  status: 1,
  key: '',
  base_url: '',
  other: '',
  proxy: '',
  test_model: '',
  models: '',
  groups: ['default'],
  tag: '',
  model_mapping: '',
  model_headers: [],
  header_override: [],
  custom_parameter: '',
  plugin: {},
  only_chat: false,
  priority: 0,
  weight: 1,
  pre_cost: 1,
  disabled_stream: '',
  compatible_response: false,
  allow_extra_body: false,
  pass_through_body: false
};

// Same values as v1 PreCostType (type/other.js): 1=normal, 2=skip images, 3=skip all.
const PRE_COST_OPTIONS = [
  { value: 1, labelKey: 'channel_edit.preCostNormal' },
  { value: 2, labelKey: 'channel_edit.preCostNoImage' },
  { value: 3, labelKey: 'channel_edit.preCostNone' }
];

export default function ChannelSheet({ open, channelId, initialTab = 'basic', onClose, onSaved, groupOptions }) {
  const { t } = useTranslation();
  const [submitting, setSubmitting] = useState(false);
  const [hasTag, setHasTag] = useState(false);
  const [modelSelectorOpen, setModelSelectorOpen] = useState(false);
  const [modelSelectorValues, setModelSelectorValues] = useState(null);
  const [activeTab, setActiveTab] = useState('basic');
  // Safe projection of the stored key from the backend: { configured, count, masked[] }.
  const [keyStatus, setKeyStatus] = useState(null);
  // displayed model name -> original model name (v1 modelOriginalMapping)
  const [modelOriginalMapping, setModelOriginalMapping] = useState({});
  const { register, handleSubmit, control, reset, setValue, getValues, watch, formState: { errors } } = useForm({ defaultValues: EMPTY });
  const typeValue = watch('type');
  const baseUrlValue = watch('base_url');
  const [iconRefreshKey, setIconRefreshKey] = useState(0);
  const [syncingPricing, setSyncingPricing] = useState(false);
  const [fetchSyncing, setFetchSyncing] = useState(false);
  const [pruning, setPruning] = useState(false);

  // OpenRouter(type=20) 自带价格：从上游 /v1/models 拉真实价并 upsert 进价格表。
  // 一键：拉取上游全量模型 → 追加进 models(去重,不删) → 同步价格+元信息。
  const handleFetchAndSync = async () => {
    setFetchSyncing(true);
    try {
      const v = trims(getValues());
      const id = channelId ? parseInt(channelId, 10) : 0;
      const fetched = await fetchUpstreamModelList({ id, type: v.type, key: v.key || '', base_url: v.base_url, other: v.other });
      const merged = Array.from(new Set([...splitModels(getValues('models')), ...fetched]));
      setValue('models', merged.join(','));
      const { synced } = await syncChannelPricing({ id, name: v.name, type: v.type, key: v.key || '' });
      toast.success(
        t('channel_edit.fetchSyncDone', {
          models: fetched.length,
          synced
        })
      );
    } catch (e) {
      toast.error(e.message);
    } finally {
      setFetchSyncing(false);
    }
  };

  const handleSyncPricing = async () => {
    setSyncingPricing(true);
    try {
      // 只发同步所需的最小字段：编辑态靠 id 后端取存储 key；新建态靠表单 key。
      // 不要整个 getValues()——其含 model_headers 数组等会触发后端绑定失败。
      const v = trims(getValues());
      const { synced, unmatched } = await syncChannelPricing({
        id: channelId ? parseInt(channelId, 10) : 0,
        name: v.name,
        type: v.type,
        key: v.key || ''
      });
      toast.success(t('channel_edit.syncPricingDone', { count: synced }));
      if (unmatched.length) {
        // 渠道里这些模型没匹配到上游价格（多为 id 带版本/别名），提示用户核对模型名。
        toast.warning(
          t('channel_edit.syncPricingUnmatched', {
            count: unmatched.length,
            models: unmatched.join(', ')
          })
        );
      }
    } catch (e) {
      toast.error(e.message);
    } finally {
      setSyncingPricing(false);
    }
  };

  // 清理「僵尸模型」：移除本渠道 models 中上游目录已不存在的条目。
  // Fetch & Sync 只增不删，上游改名/下架后旧 id 会永久残留、显示「价格未配置」。
  // 别名（model_mapping 的 key）是故意配置的，即使不在上游目录也保留。
  // 仅改动表单里的 models 值，用户仍需保存才生效（可先核对再保存）。
  const handlePruneStaleModels = async () => {
    setPruning(true);
    try {
      const v = trims(getValues());
      const id = channelId ? parseInt(channelId, 10) : 0;
      const upstream = await fetchUpstreamModelList({ id, type: v.type, key: v.key || '', base_url: v.base_url, other: v.other });
      const upstreamSet = new Set(upstream);
      const current = splitModels(getValues('models'));
      const mapping = parseModelMapping(getValues('model_mapping')) || {};
      const aliasKeys = new Set(Object.keys(mapping).map((k) => k.trim()));
      const stale = current.filter((m) => !upstreamSet.has(m) && !aliasKeys.has(m));
      if (stale.length === 0) {
        toast.success(t('channel_edit.pruneNone'));
        return;
      }
      const kept = current.filter((m) => !stale.includes(m));
      setValue('models', kept.join(','));
      const preview = stale.slice(0, 8).join(', ') + (stale.length > 8 ? ` …(+${stale.length - 8})` : '');
      toast.success(
        t('channel_edit.pruneDone', {
          count: stale.length,
          models: preview
        })
      );
    } catch (e) {
      toast.error(e.message);
    } finally {
      setPruning(false);
    }
  };

  const applyTypeDefaults = (nextType) => {
    const cfg = typeConfig[nextType]?.input;
    if (cfg?.models) setValue('models', cfg.models.join(','));
    if (cfg?.test_model !== undefined) setValue('test_model', cfg.test_model);
  };

  useEffect(() => {
    if (!open) return;
    setHasTag(false);
    setModelSelectorOpen(false);
    setModelOriginalMapping({});
    setActiveTab(initialTab);
    setKeyStatus(null);
    if (channelId) {
      loadChannel(channelId);
    } else {
      reset(EMPTY);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, channelId]);

  // Snapshot the form values the selector needs at open time. When editing,
  // `key` is usually blank, so include `id` for the backend stored-key
  // fallback (see controller GetModelList).
  const openModelSelector = () => {
    const v = getValues();
    setModelSelectorValues({
      id: channelId ? parseInt(channelId, 10) : 0,
      type: v.type,
      key: v.key,
      base_url: v.base_url,
      other: v.other,
      models: v.models
    });
    setModelSelectorOpen(true);
  };

  // Write the picked models back into the comma-separated Textarea value.
  // Overwrite replaces the list; merge appends only the new ids (same
  // semantics as v1 handleModelSelectorConfirm).
  const handleModelSelectorConfirm = (selectedIds, overwrite) => {
    if (overwrite) {
      setValue('models', selectedIds.join(','));
      return;
    }
    const existing = (getValues('models') || '')
      .split(/[,\n]/)
      .map((m) => m.trim())
      .filter(Boolean);
    setValue('models', [...new Set([...existing, ...selectedIds])].join(','));
  };

  const splitModels = (str) =>
    (str || '')
      .split(/[,\n]/)
      .map((m) => m.trim())
      .filter(Boolean);

  // Parse the model_mapping JSON string into a plain {redirect: original}
  // object; returns null when empty/invalid (v1 parseModelMapping semantics,
  // tolerant of malformed JSON while typing).
  const parseModelMapping = (mappingStr) => {
    if (!mappingStr || !mappingStr.trim()) return null;
    try {
      const parsed = JSON.parse(mappingStr);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null;
      const mapping = {};
      Object.entries(parsed).forEach(([key, value]) => {
        if (key && typeof value === 'string' && value) mapping[key] = value;
      });
      return Object.keys(mapping).length ? mapping : null;
    } catch (e) {
      return null;
    }
  };

  // Unified model list update (v1 updateModelsList) for the comma-separated value.
  const updateModelsList = (newModels, newMapping) => {
    setValue('models', Array.from(new Set(newModels.filter(Boolean))).join(','));
    setModelOriginalMapping(newMapping);
  };

  // Restore displayed names back to original names when mapping is cleared
  // (v1 restoreModelsToOriginalNames).
  const restoreModelsToOriginalNames = (currentModels) => {
    const restoredModels = currentModels.map((m) => modelOriginalMapping[m] || m);
    const hasChanges = currentModels.some((m, index) => m !== restoredModels[index]);
    if (hasChanges) updateModelsList(restoredModels, {});
  };

  // Core mapping application logic (v1 applyModelMapping): rename models that
  // match a mapping value to the mapping key, tracking originals; models no
  // longer covered by the mapping are restored to their original names.
  const applyModelMapping = (mapping, currentModels, currentMapping) => {
    let updatedModels = [...currentModels];
    const newMapping = { ...currentMapping };
    let hasChanges = false;

    Object.entries(mapping).forEach(([key, mappedValue]) => {
      if (typeof key !== 'string' || typeof mappedValue !== 'string') return;
      const keyTrimmed = key.trim();
      const valueTrimmed = mappedValue.trim();
      if (!keyTrimmed || !valueTrimmed) return;

      const valueIndex = updatedModels.findIndex((m) => m === valueTrimmed || newMapping[m] === valueTrimmed);
      if (valueIndex !== -1) {
        const currentDisplayName = updatedModels[valueIndex];
        if (currentDisplayName !== keyTrimmed) {
          if (!newMapping[keyTrimmed]) {
            newMapping[keyTrimmed] = newMapping[currentDisplayName] || currentDisplayName;
          }
          if (newMapping[currentDisplayName]) {
            delete newMapping[currentDisplayName];
          }
          updatedModels[valueIndex] = keyTrimmed;
          hasChanges = true;
        }
      }
    });

    const mappingKeys = new Set(Object.keys(mapping).map((key) => key.trim()));
    updatedModels = updatedModels.map((m) => {
      if (!mappingKeys.has(m) && newMapping[m]) {
        const originalName = newMapping[m];
        delete newMapping[m];
        hasChanges = true;
        return originalName;
      }
      return m;
    });

    return { updatedModels, newMapping, hasChanges };
  };

  // Live sync from the model_mapping field to the models list
  // (v1 syncModelMappingToModels).
  const syncModelMappingToModels = (mappingStr) => {
    const currentModels = splitModels(getValues('models'));
    const mapping = parseModelMapping(mappingStr);
    if (!mapping) {
      restoreModelsToOriginalNames(currentModels);
      return;
    }
    const { updatedModels, newMapping, hasChanges } = applyModelMapping(mapping, currentModels, modelOriginalMapping);
    if (hasChanges) updateModelsList(updatedModels, newMapping);
  };

  const loadChannel = async (id) => {
    try {
      const res = await API.get(`/api/channel/${id}`);
      const { success, message, data } = res.data;
      if (!success) {
        toast.error(message);
        return;
      }
      setKeyStatus(data.key_status ?? null);
      let mapping = '';
      if (data.model_mapping && data.model_mapping !== '') {
        try {
          mapping = JSON.stringify(JSON.parse(data.model_mapping), null, 2);
        } catch (e) {
          mapping = data.model_mapping;
        }
      }
      let custom = '';
      if (data.custom_parameter && data.custom_parameter !== '') {
        try {
          custom = JSON.stringify(JSON.parse(data.custom_parameter), null, 2);
        } catch (e) {
          custom = data.custom_parameter;
        }
      }
      let headers = [];
      if (data.model_headers && data.model_headers !== '') {
        try {
          headers = Object.entries(JSON.parse(data.model_headers)).map(([key, value]) => ({ key, value }));
        } catch (e) {
          headers = [];
        }
      }
      let overrideHeaders = [];
      if (data.header_override && data.header_override !== '') {
        try {
          overrideHeaders = Object.entries(JSON.parse(data.header_override)).map(([key, value]) => ({ key, value }));
        } catch (e) {
          overrideHeaders = [];
        }
      }
      reset({
        ...EMPTY,
        name: data.name ?? '',
        type: data.type ?? 1,
        status: data.status ?? 1,
        key: '',
        base_url: data.base_url ?? '',
        other: data.other ?? '',
        proxy: data.proxy ?? '',
        test_model: data.test_model ?? '',
        models: data.models ?? '',
        groups: data.group ? data.group.split(',') : [],
        tag: data.tag ?? '',
        model_mapping: mapping,
        model_headers: headers,
        header_override: overrideHeaders,
        custom_parameter: custom,
        plugin: data.plugin && typeof data.plugin === 'object' ? data.plugin : {},
        only_chat: !!data.only_chat,
        priority: data.priority ?? 0,
        weight: data.weight ?? 1,
        pre_cost: data.pre_cost ?? 1,
        disabled_stream: Array.isArray(data.disabled_stream) ? data.disabled_stream.join(',') : '',
        compatible_response: !!data.compatible_response,
        allow_extra_body: !!data.allow_extra_body,
        pass_through_body: !!data.pass_through_body
      });
      setHasTag(!!data.tag);
      // Seed the original-name mapping from the stored model_mapping so later
      // edits can restore correctly (v1 EditModal load-time init).
      const parsedMapping = parseModelMapping(mapping);
      if (parsedMapping) {
        const loadedModels = splitModels(data.models ?? '');
        const initialMapping = {};
        Object.entries(parsedMapping).forEach(([key, value]) => {
          if (loadedModels.includes(key)) initialMapping[key] = value;
        });
        setModelOriginalMapping(initialMapping);
      } else {
        setModelOriginalMapping({});
      }
    } catch (error) {
      toast.error(error.message);
    }
  };

  const onSubmit = async (raw) => {
    const values = trims({ ...raw });
    // All required fields live in the "basic" tab; jump there on any miss.
    const fail = (key) => {
      setActiveTab('basic');
      toast.error(t(`channel_edit.${key}`));
      return true;
    };
    if (!values.name && fail('requiredName')) return;
    if (!values.models && fail('requiredModels')) return;
    if ((!values.groups || values.groups.length === 0) && fail('requiredGroup')) return;
    if (!channelId && !values.key && fail('requiredKey')) return;
    if ([3, 8].includes(values.type) && !values.base_url && fail('requiredBaseUrl')) return;

    if (values.base_url && values.base_url.endsWith('/')) values.base_url = values.base_url.slice(0, -1);
    if (values.type === 3 && values.other === '') values.other = '2024-05-01-preview';
    if (values.type === 18 && values.other === '') values.other = 'v2.1';

    if (values.model_mapping) {
      try {
        values.model_mapping = JSON.stringify(JSON.parse(values.model_mapping));
      } catch (e) {
        return toast.error('Error parsing model mapping: ' + e.message);
      }
    }
    if (Array.isArray(values.model_headers)) {
      const headerObj = {};
      values.model_headers.forEach((item) => {
        if (item && item.key && item.value && !(item.key in headerObj)) {
          headerObj[item.key] = item.value;
        }
      });
      values.model_headers = Object.keys(headerObj).length ? JSON.stringify(headerObj) : '';
    }
    if (Array.isArray(values.header_override)) {
      const overrideObj = {};
      values.header_override.forEach((item) => {
        if (item && item.key && item.value && !(item.key in overrideObj)) {
          overrideObj[item.key] = item.value;
        }
      });
      values.header_override = Object.keys(overrideObj).length ? JSON.stringify(overrideObj) : '';
    }
    if (values.custom_parameter) {
      try {
        JSON.parse(values.custom_parameter);
      } catch (e) {
        return toast.error('Error parsing custom parameter: ' + e.message);
      }
    }
    const typePlugins = pluginConfig[String(values.type)];
    if (typePlugins) {
      const normalized = {};
      for (const pluginId of Object.keys(typePlugins)) {
        normalized[pluginId] = {};
        for (const paramId of Object.keys(typePlugins[pluginId].params)) {
          const param = typePlugins[pluginId].params[paramId];
          const v = values.plugin?.[pluginId]?.[paramId];
          // 未触碰的字段写入声明的默认值而非空串，与表单展示值一致（后端对空串同样归一到默认值）。
          normalized[pluginId][paramId] = v ?? (param.type === 'bool' ? false : param.default ?? '');
        }
      }
      values.plugin = normalized;
    } else {
      values.plugin = values.plugin && typeof values.plugin === 'object' ? values.plugin : {};
    }

    // Same as v1 removeDuplicates (EditModal.jsx L819-820): payload is a deduped string[].
    values.disabled_stream = Array.from(
      new Set(
        (values.disabled_stream || '')
          .split(/[,\n]/)
          .map((m) => m.trim())
          .filter(Boolean)
      )
    );

    values.priority = Number(values.priority) || 0;
    values.weight = Number(values.weight) || 1;

    const modelsStr = Array.from(
      new Set(
        values.models
          .split(/[,\n]/)
          .map((m) => m.trim())
          .filter(Boolean)
      )
    ).join(',');
    const payload = { ...values, models: modelsStr, group: values.groups.join(',') };

    setSubmitting(true);
    try {
      let res;
      if (channelId) {
        res = await API.put('/api/channel/', { ...payload, id: parseInt(channelId, 10) });
      } else {
        res = await API.post('/api/channel/', { ...payload });
      }
      const { success, message, data } = res.data;
      if (success) {
        if (data?.key_status) setKeyStatus(data.key_status);
        toast.success(channelId ? t('channel_edit.editSuccess') : t('channel_edit.addSuccess'));
        onSaved();
      } else {
        toast.error(message);
      }
    } catch (error) {
      toast.error(error.message);
    }
    setSubmitting(false);
  };

  return (
    <>
      <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
        <SheetContent onClose={onClose} className="max-w-3xl">
          <SheetHeader>
            <SheetTitle>{channelId ? t('common.edit') : t('common.create')}</SheetTitle>
          </SheetHeader>
          <form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
            <SheetBody className="space-y-4">{renderFields()}</SheetBody>
            <SheetFooter>
              <Button type="button" variant="outline" onClick={onClose}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting && <Loader2 className="size-4 animate-spin" />}
                {t('common.submit')}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
      <ModelSelectorModal
        open={modelSelectorOpen}
        onClose={() => setModelSelectorOpen(false)}
        onConfirm={handleModelSelectorConfirm}
        channelValues={modelSelectorValues}
      />
    </>
  );

  function renderFields() {
    const hasPlugin = Boolean(pluginConfig[String(typeValue)]);
    return (
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="basic">{t('channel_edit.tabBasic')}</TabsTrigger>
          <TabsTrigger value="model">{t('channel_edit.tabModel')}</TabsTrigger>
          <TabsTrigger value="request">{t('channel_edit.tabRequest')}</TabsTrigger>
          <TabsTrigger value="advanced">{t('channel_edit.tabAdvanced')}</TabsTrigger>
          {hasPlugin && <TabsTrigger value="plugin">{t('channel_edit.pluginConfig')}</TabsTrigger>}
        </TabsList>

        {/* 基础：必填集中在这里，填完即可创建。 */}
        <TabsContent value="basic" className="space-y-5">
          <FormField id="channel-name" label={t('channel_index.name')} required error={errors.name}>
            <Input id="channel-name" {...register('name')} />
          </FormField>

          <FormField id="channel-type" label={t('channel_index.type')} required>
            <Controller
              control={control}
              name="type"
              render={({ field }) => (
                <Combobox
                  id="channel-type"
                  value={String(field.value)}
                  onValueChange={(v) => {
                    field.onChange(Number(v));
                    applyTypeDefaults(Number(v));
                  }}
                  options={CHANNEL_OPTIONS_ORDERED.map((o) => ({ value: String(o.value), label: o.text }))}
                  searchPlaceholder={t('channel_index.typeSearchPlaceholder')}
                  emptyText={t('channel_index.typeSearchEmpty')}
                />
              )}
            />
          </FormField>

          <FormField
            id="channel-key"
            label={configText(t, defaultConfig.inputLabel.key)}
            required={!channelId}
            hint={channelId ? t('channel_edit.keyEditTip', { defaultValue: 'Leave blank to keep the existing key' }) : null}
          >
            {channelId && keyStatus ? (
              keyStatus.configured ? (
                <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <KeyRound className="size-3.5" />
                  {keyStatus.count > 1
                    ? t('channel_edit.keyConfiguredMulti', {
                        count: keyStatus.count,
                        masked: keyStatus.masked.join(' ')
                      })
                    : t('channel_edit.keyConfigured', {
                        masked: keyStatus.masked[0]
                      })}
                </p>
              ) : (
                <p className="flex items-center gap-1.5 text-xs font-medium text-amber-600 dark:text-amber-400">
                  <AlertTriangle className="size-3.5" />
                  {t('channel_edit.keyNotConfigured')}
                </p>
              )
            ) : null}
            <Textarea id="channel-key" rows={2} {...register('key')} placeholder={channelId ? '••••••••' : ''} />
            {[57, 58, 59, 60].includes(typeValue) && (
              <ChannelOAuth
                key={typeValue}
                type={typeValue}
                channelId={channelId}
                getValues={getValues}
                onCredentials={(credentials) => setValue('key', credentials)}
              />
            )}
          </FormField>

          <FormField id="channel-base-url" label={t('channel_index.channelApiAddress')} required={[3, 8].includes(typeValue)}>
            <Input id="channel-base-url" {...register('base_url')} />
          </FormField>

          <FormField label={t('channel_edit.icon')} hint={t('channel_edit.iconTip')}>
            <div className="flex items-center gap-3">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-md border border-border bg-background">
                <ProviderIcon key={iconRefreshKey} type={typeValue} name={watch('name')} baseUrl={baseUrlValue} className="size-6" />
              </span>
              <BrandIconRefreshButton
                domain={iconCacheHost(baseUrlValue) || undefined}
                onRefreshed={() => setIconRefreshKey((k) => k + 1)}
              />
            </div>
          </FormField>

          <FormField label={t('channel_index.group')} required>
            <div className="flex flex-wrap gap-3">
              <Controller
                control={control}
                name="groups"
                render={({ field }) => (
                  <>
                    {(groupOptions.length ? groupOptions : ['default']).map((g) => {
                      const checked = field.value?.includes(g);
                      return (
                        <label key={g} className="flex items-center gap-2 text-sm">
                          <Checkbox
                            checked={checked}
                            onCheckedChange={(c) =>
                              field.onChange(c ? [...(field.value || []), g] : field.value.filter((x) => x !== g))
                            }
                          />
                          {g}
                        </label>
                      );
                    })}
                  </>
                )}
              />
            </div>
          </FormField>

          <FormField id="channel-models" label={configText(t, defaultConfig.inputLabel.models)} required help={configText(t, defaultConfig.prompt.models)}>
            <Controller
              control={control}
              name="models"
              render={({ field }) => (
                <ModelListInput
                  value={field.value}
                  onChange={field.onChange}
                  headerActions={
                    typeConfig[typeValue]?.inputLabel?.provider_models_list ? (
                      <>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="h-7 gap-1 px-2 text-xs"
                          disabled={hasTag}
                          title={configText(t, defaultConfig.prompt.provider_models_list)}
                          onClick={openModelSelector}
                        >
                          <CloudDownload className="size-3.5" />
                          {configText(t, typeConfig[typeValue].inputLabel.provider_models_list)}
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="h-7 gap-1 px-2 text-xs"
                          disabled={pruning || hasTag}
                          title={t('channel_edit.pruneStaleTip')}
                          onClick={handlePruneStaleModels}
                        >
                          {pruning ? <Loader2 className="size-3.5 animate-spin" /> : <Eraser className="size-3.5" />}
                          {t('channel_edit.pruneStale')}
                        </Button>
                      </>
                    ) : null
                  }
                />
              )}
            />
            {typeValue === 20 && (
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={syncingPricing}
                  title={t('channel_edit.syncPricingTip')}
                  onClick={handleSyncPricing}
                >
                  {syncingPricing ? <Loader2 className="size-4 animate-spin" /> : <CloudDownload className="size-4" />}
                  {t('channel_edit.syncPricing')}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={fetchSyncing || hasTag}
                  title={t('channel_edit.fetchAndSyncTip')}
                  onClick={handleFetchAndSync}
                >
                  {fetchSyncing ? <Loader2 className="size-4 animate-spin" /> : <CloudDownload className="size-4" />}
                  {t('channel_edit.fetchAndSync')}
                </Button>
              </div>
            )}
          </FormField>
        </TabsContent>

        {/* 模型 */}
        <TabsContent value="model" className="space-y-5">
          <FormField label={configText(t, defaultConfig.inputLabel.model_mapping)} help={configText(t, defaultConfig.prompt.model_mapping)}>
            <Controller
              control={control}
              name="model_mapping"
              render={({ field }) => (
                <ModelMappingInput
                  value={field.value}
                  models={splitModels(watch('models'))}
                  onChange={(v) => {
                    field.onChange(v);
                    syncModelMappingToModels(v);
                  }}
                />
              )}
            />
          </FormField>

          <FormField id="channel-disabled-stream" label={configText(t, defaultConfig.inputLabel.disabled_stream)} hint={configText(t, defaultConfig.prompt.disabled_stream)}>
            <Textarea id="channel-disabled-stream" rows={2} disabled={hasTag} placeholder="gpt-4o, o1-mini" {...register('disabled_stream')} />
          </FormField>

          <FormField id="channel-test-model" label={t('channel_index.testModel')} hint={configText(t, defaultConfig.prompt.test_model)}>
            <Input id="channel-test-model" {...register('test_model')} />
          </FormField>
        </TabsContent>

        {/* 请求 */}
        <TabsContent value="request" className="space-y-5">
          <FormField id="channel-custom-parameter" label={configText(t, defaultConfig.inputLabel.custom_parameter)} help={configText(t, defaultConfig.prompt.custom_parameter)}>
            <Textarea id="channel-custom-parameter" rows={3} className="font-mono text-xs" {...register('custom_parameter')} />
          </FormField>

          <FormField label={configText(t, defaultConfig.inputLabel.model_headers)} hint={configText(t, defaultConfig.prompt.model_headers)}>
            <Controller
              control={control}
              name="model_headers"
              render={({ field }) => <ModelHeadersInput value={field.value} onChange={field.onChange} />}
            />
          </FormField>

          <FormField label={configText(t, defaultConfig.inputLabel.header_override)} hint={configText(t, defaultConfig.prompt.header_override)}>
            <Controller
              control={control}
              name="header_override"
              render={({ field }) => <ModelHeadersInput value={field.value} onChange={field.onChange} />}
            />
          </FormField>

          <div className="space-y-3">
            <ToggleRow control={control} name="only_chat" label={configText(t, defaultConfig.inputLabel.only_chat)} />
            <ToggleRow control={control} name="compatible_response" label={configText(t, defaultConfig.inputLabel.compatible_response)} />
            <ToggleRow control={control} name="allow_extra_body" label={configText(t, defaultConfig.inputLabel.allow_extra_body)} />
            <ToggleRow control={control} name="pass_through_body" label={configText(t, defaultConfig.inputLabel.pass_through_body)} />
          </div>
        </TabsContent>

        {/* 高级 */}
        <TabsContent value="advanced" className="space-y-5">
          <FormField id="channel-pre-cost" row label={configText(t, defaultConfig.inputLabel.pre_cost)} help={configText(t, defaultConfig.prompt.pre_cost)}>
            <Controller
              control={control}
              name="pre_cost"
              render={({ field }) => (
                <Select value={String(field.value)} onValueChange={(v) => field.onChange(Number(v))}>
                  <SelectTrigger id="channel-pre-cost" disabled={hasTag}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PRE_COST_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={String(o.value)}>
                        {t(o.labelKey)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
          </FormField>

          <FormField
            row
            id="channel-priority"
            label={t('channel_index.priority')}
            controlClassName="w-32"
            help={
              <>
                {t('channel_index.priorityWeightExplanation')}
                <br />
                {t('channel_index.description1')}
                <br />
                {t('channel_index.description2')}
                <br />
                {t('channel_index.description3')}
                <br />
                {t('channel_index.description4')}
              </>
            }
          >
            <Input id="channel-priority" type="number" {...register('priority')} />
          </FormField>

          <FormField row id="channel-weight" label={t('channel_index.weight')} controlClassName="w-32">
            <Input id="channel-weight" type="number" {...register('weight')} />
          </FormField>

          <FormField id="channel-proxy" label={configText(t, defaultConfig.inputLabel.proxy)} help={configText(t, defaultConfig.prompt.proxy)}>
            <Input id="channel-proxy" {...register('proxy')} />
          </FormField>

          <FormField id="channel-other" label={configText(t, defaultConfig.inputLabel.other)}>
            <Input id="channel-other" {...register('other')} />
          </FormField>

          <FormField id="channel-tag" label={configText(t, defaultConfig.inputLabel.tag)} help={configText(t, defaultConfig.prompt.tag)}>
            <Input id="channel-tag" {...register('tag')} />
          </FormField>
        </TabsContent>

        {hasPlugin && (
          <TabsContent value="plugin" className="space-y-5">
            <Controller
              control={control}
              name="plugin"
              render={({ field }) => <PluginForm type={typeValue} value={field.value} onChange={field.onChange} />}
            />
          </TabsContent>
        )}
      </Tabs>
    );
  }
}

function ToggleRow({ control, name, label }) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field }) => (
        <div className="flex items-center justify-between">
          <span className="text-sm">{label}</span>
          <Switch checked={!!field.value} onCheckedChange={field.onChange} />
        </div>
      )}
    />
  );
}

ChannelSheet.propTypes = {
  open: PropTypes.bool,
  channelId: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  initialTab: PropTypes.string,
  onClose: PropTypes.func,
  onSaved: PropTypes.func,
  groupOptions: PropTypes.array
};
