import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { ArrowUpRight, Loader2, X } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError, timestamp2string, trims } from 'utils/common';
import { cn } from '@/lib/utils';
import { statusLabel } from '../Channel/channelApi';
import { PriceFields, buildPricePayload, priceFormValues, usePriceUnits } from '../Pricing/PricingSheet';
import { enabledChannels, modelGroups, modelStatus, splitChannelGroups } from '../Pricing/modelCatalog';
import { ChannelGlyph, channelEditPath } from './ModelTable';
import {
  ModelStatusBadge,
  MODALITY_VALUES,
  MODALITY_COLORS,
  MODE_VALUES,
  MODE_LABEL_KEYS,
  CAPABILITY_VALUES,
  CAPABILITY_LABEL_KEYS,
  ENDPOINT_VALUES,
  ENDPOINT_LABEL_KEYS,
  modalityText,
  safeJsonArray
} from './modelInfoHelpers';

// 厂商下拉的「未知」项:Select 不接受空串值,用哨兵值表示 vendor_id = 0。
const VENDOR_NONE = 'none';

// 渠道未带状态(后端摘要)时视为启用;只有启用渠道才算可路由。
const isChannelEnabled = (channel) => (channel.status ?? 1) === 1;

export const SHEET_TABS = ['profile', 'pricing', 'channels'];

const ORIGIN = {
  model: '',
  name: '',
  description: '',
  context_length: 128000,
  max_tokens: 4096,
  input_modalities: '["text"]',
  output_modalities: '["text"]',
  tags: '[]',
  mode: '',
  capabilities: '',
  vendor_id: 0,
  hidden: false,
  endpoints: '',
  alias_of: '',
  locked: false
};

const SOURCE_LABEL_KEYS = {
  manual: 'modelInfoPage.sourceManual',
  'models.dev': 'modelInfoPage.sourceModelsDev',
  openrouter: 'modelInfoPage.sourceOpenRouter'
};

// 目录表单 → 接口载荷。
function buildCatalogPayload(values, originalCapabilities) {
  const payload = trims({ ...values });
  payload.context_length = parseInt(payload.context_length, 10);
  payload.max_tokens = parseInt(payload.max_tokens, 10);
  payload.locked = !!values.locked;
  payload.hidden = !!values.hidden;
  payload.vendor_id = parseInt(values.vendor_id, 10) || 0;
  // 接口能力空数组即「未设置」,与后端 endpoints 空串语义一致。
  const endpoints = safeJsonArray(values.endpoints);
  payload.endpoints = endpoints.length === 0 ? '' : JSON.stringify(endpoints);
  // 空串=未知:三项都关且原值也是空串时保持空串,不把「未知」写成「明确都不支持」。
  const capabilities = safeJsonArray(values.capabilities);
  payload.capabilities = capabilities.length === 0 && !originalCapabilities ? '' : JSON.stringify(capabilities);
  // 来源由后端强制为 manual,前端不回传。
  delete payload.source;
  return payload;
}

// 请求结果统一为 { ok, message },便于分别提示目录 / 价格哪一部分没保存。
async function callApi(request) {
  try {
    const res = await request();
    return { ok: !!res.data?.success, message: res.data?.message || '' };
  } catch (error) {
    return { ok: false, message: error?.message || String(error) };
  }
}

function Section({ title, children }) {
  return (
    <section className="space-y-4">
      <h3 className="border-b border-border pb-1.5 text-sm font-semibold">{title}</h3>
      {children}
    </section>
  );
}

Section.propTypes = { title: PropTypes.node, children: PropTypes.node };

// 「渠道与分组」Tab:提供此模型的渠道(状态 / 优先级·权重 / 分组,点行进渠道编辑)、可用分组、别名指向;
// 没有渠道时引导去渠道页绑定。渠道关联只读,在渠道页维护。
function ChannelsPanel({ row }) {
  const { t } = useTranslation();
  const channels = row.bound_channels || [];
  const groups = modelGroups(row);
  return (
    <div className="space-y-6">
      <section className="space-y-2">
        <h3 className="text-sm font-semibold">{t('modelsPage.sheet.providingChannels', { count: channels.length })}</h3>
        {enabledChannels(row).length === 0 && (
          <div className="space-y-2 rounded-lg border border-warning/40 bg-warning/10 p-3 text-sm">
            <p className="text-warning">{t('modelInfoPage.noChannel')}</p>
            <p className="text-muted-foreground">{t('modelsPage.sheet.noChannelHint')}</p>
            <Button asChild size="sm" variant="outline">
              <Link to="/panel/channel">{t('modelsPage.sheet.goChannels')}</Link>
            </Button>
          </div>
        )}
        {channels.length > 0 && (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="py-1.5 pr-2 font-medium">{t('modelInfoPage.channels')}</th>
                  <th className="py-1.5 pr-2 font-medium">{t('modelsPage.sheet.status')}</th>
                  <th className="py-1.5 pr-2 font-medium">{t('modelsPage.sheet.priorityWeight')}</th>
                  <th className="py-1.5 pr-2 font-medium">{t('modelsPage.sheet.groups')}</th>
                  <th className="w-6" />
                </tr>
              </thead>
              <tbody>
                {channels.map((channel) => {
                  const enabled = isChannelEnabled(channel);
                  return (
                    <tr key={channel.id} data-testid="bound-channel" className="border-b border-border last:border-0">
                      <td className="py-2 pr-2">
                        <span className={cn('flex items-center gap-2', !enabled && 'text-muted-foreground')}>
                          <ChannelGlyph channel={channel} />
                          <span className="truncate">{channel.name}</span>
                        </span>
                      </td>
                      <td className="py-2 pr-2">
                        <Badge
                          variant="outline"
                          data-testid={enabled ? undefined : 'channel-disabled-badge'}
                          className={cn(
                            'whitespace-nowrap border-transparent px-1.5 py-0 font-normal',
                            enabled ? 'bg-success/15 text-success' : 'bg-destructive/10 text-destructive'
                          )}
                        >
                          {statusLabel(t, channel.status ?? 1)}
                        </Badge>
                      </td>
                      <td className="whitespace-nowrap py-2 pr-2 tabular-nums">
                        {channel.priority ?? '—'} / {channel.weight ?? '—'}
                      </td>
                      <td className="py-2 pr-2 text-muted-foreground">{splitChannelGroups(channel.group).join(', ') || '—'}</td>
                      <td className="py-2 text-right">
                        <Link
                          to={channelEditPath(channel)}
                          aria-label={t('modelsPage.sheet.openChannel', { name: channel.name })}
                          className="inline-flex text-muted-foreground hover:text-foreground"
                        >
                          <ArrowUpRight className="size-4" />
                        </Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
        {channels.length > 0 && <p className="text-xs text-muted-foreground">{t('modelsPage.sheet.routingHint')}</p>}
      </section>

      <section className="space-y-2">
        <h3 className="text-sm font-semibold">{t('modelsPage.sheet.availableGroups')}</h3>
        {groups.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {groups.map((group) => (
              <Badge key={group} variant="secondary" className="font-normal">
                {group}
              </Badge>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">—</p>
        )}
      </section>

      {row.alias_of && (
        <section className="space-y-2">
          <h3 className="text-sm font-semibold">{t('modelInfoPage.aliasOf')}</h3>
          <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{row.alias_of}</code>
        </section>
      )}
    </div>
  );
}

ChannelsPanel.propTypes = { row: PropTypes.object.isRequired };

function ModalityPicker({ value, onChange }) {
  const { t } = useTranslation();
  const selected = safeJsonArray(value);
  const toggle = (m) => {
    const next = selected.includes(m) ? selected.filter((x) => x !== m) : [...selected, m];
    onChange(JSON.stringify(next));
  };
  return (
    <div className="flex flex-wrap gap-2">
      {MODALITY_VALUES.map((m) => {
        const active = selected.includes(m);
        return (
          <button key={m} type="button" onClick={() => toggle(m)}>
            <Badge variant="outline" className={active ? `border-transparent ${MODALITY_COLORS[m]}` : 'text-muted-foreground'}>
              {t(`modelpricePage.modality.${m}`, { defaultValue: modalityText(m) })}
            </Badge>
          </button>
        );
      })}
    </div>
  );
}

function TagInput({ value, onChange }) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState('');
  const tags = safeJsonArray(value);
  const add = () => {
    const v = draft.trim();
    if (v && !tags.includes(v)) onChange(JSON.stringify([...tags, v]));
    setDraft('');
  };
  return (
    <div className="space-y-2">
      <Input
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            add();
          }
        }}
        onBlur={add}
        placeholder={t('modelInfoPage.tagsPlaceholder')}
      />
      {tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {tags.map((tag, i) => (
            <Badge key={`${tag}-${i}`} variant="secondary" className="gap-1">
              {tag}
              <button type="button" onClick={() => onChange(JSON.stringify(tags.filter((x) => x !== tag)))}>
                <X className="size-3" />
              </button>
            </Badge>
          ))}
        </div>
      )}
    </div>
  );
}

// 模型抽屉:资料 / 定价 / 渠道与分组三个 Tab,一处编辑目录信息与价格。row 为模型表的合并行(null = 工具栏新建);
// 目录无行的模型按新建处理,无价格的模型价格节为空表单。保存时分别调用目录与价格接口,只写有改动的部分。
// 各 Tab 面板常驻 DOM(非当前 Tab 隐藏),切换 Tab 不丢未保存的输入。
export default function ModelInfoSheet({
  open,
  onOpenChange,
  row = null,
  initialTab = 'profile',
  existingModels = [],
  prices = [],
  ownedby = [],
  unit = 'M',
  onSaved
}) {
  const { t } = useTranslation();
  const editId = row?.catalogId || 0;
  const hasPrice = !!row?.price;
  const [tab, setTab] = useState(initialTab);

  useEffect(() => {
    if (open) setTab(row && SHEET_TABS.includes(initialTab) ? initialTab : 'profile');
  }, [open, row, initialTab]);
  const [values, setValues] = useState(ORIGIN);
  const [baseline, setBaseline] = useState(ORIGIN);
  const [originalModel, setOriginalModel] = useState('');
  const [originalCapabilities, setOriginalCapabilities] = useState('');
  const [modelOptions, setModelOptions] = useState([]);
  const [vendorOptions, setVendorOptions] = useState([]);
  const [errors, setErrors] = useState({});
  const [saving, setSaving] = useState(false);

  const setField = (key, val) => setValues((prev) => ({ ...prev, [key]: val }));

  const {
    control,
    reset: resetPrice,
    watch,
    getValues,
    setValue,
    formState: { isDirty: priceDirty }
  } = useForm({ defaultValues: priceFormValues(null) });
  const priceType = watch('type');
  const units = usePriceUnits({ open, isEdit: hasPrice, unit, getValues, setValue });

  useEffect(() => {
    if (!open) return;
    resetPrice(priceFormValues(row?.price || null));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, row]);

  useEffect(() => {
    if (!open) return;
    API.get('/api/prices/model_list')
      .then((res) => {
        const { success, data } = res.data;
        if (success) setModelOptions(data || []);
      })
      .catch(() => {});
    API.get('/api/model_ownedby/')
      .then((res) => {
        const { success, data } = res.data;
        if (success) setVendorOptions(data || []);
      })
      .catch(() => {});
  }, [open]);

  useEffect(() => {
    if (!open) return;
    setErrors({});
    if (!editId) {
      const initial = row?.model ? { ...ORIGIN, model: row.model, name: row.model } : ORIGIN;
      setValues(initial);
      setBaseline(initial);
      setOriginalModel('');
      setOriginalCapabilities('');
      return;
    }
    API.get(`/api/model_info/${editId}`)
      .then((res) => {
        const { success, message, data } = res.data;
        if (!success) return showError(message);
        if (!data.input_modalities) data.input_modalities = '[]';
        if (!data.output_modalities) data.output_modalities = '[]';
        if (!data.tags) data.tags = '[]';
        if (!data.mode) data.mode = '';
        if (!data.capabilities) data.capabilities = '';
        if (!data.endpoints) data.endpoints = '';
        if (!data.alias_of) data.alias_of = '';
        // 决策 4:编辑已有记录时锁定默认打开,防止改完被下次同步冲掉。
        const loaded = { ...ORIGIN, ...data, locked: true };
        setValues(loaded);
        setBaseline(loaded);
        setOriginalModel(data.model);
        setOriginalCapabilities(data.capabilities);
      })
      .catch(() => {});
  }, [open, editId, row?.model]);

  const validate = () => {
    const e = {};
    if (!values.model?.trim()) e.model = t('modelInfoPage.modelRequired');
    if (!values.name?.trim()) e.name = t('modelInfoPage.nameRequired');
    if (values.context_length === '' || isNaN(Number(values.context_length))) e.context_length = t('modelInfoPage.contextLengthRequired');
    if (values.max_tokens === '' || isNaN(Number(values.max_tokens))) e.max_tokens = t('modelInfoPage.maxTokensRequired');
    if (values.alias_of?.trim() && values.alias_of.trim() === values.model?.trim()) e.alias_of = t('modelInfoPage.aliasSelfError');
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const submit = async () => {
    const model = (values.model || '').trim();
    const catalogDirty = JSON.stringify(values) !== JSON.stringify(baseline);
    // 改了模型名时价格随之改名(original_models 为旧名,models 为新名)。
    const savePrice = hasPrice ? priceDirty || model !== row.model : priceDirty;
    // 工具栏新建必写目录;目录无行的模型在目录有改动或整张表都没改时按新建写入。
    const saveCatalog = !row || catalogDirty || (!editId && !savePrice);
    if (!saveCatalog && !savePrice) {
      onOpenChange(false);
      return;
    }

    if (saveCatalog) {
      if (!validate()) return;
      if (existingModels.includes(model) && model !== originalModel) {
        setErrors((prev) => ({ ...prev, model: t('modelInfoPage.modelExists') }));
        return;
      }
    }
    let price = null;
    if (savePrice) {
      const built = buildPricePayload(getValues(), units.unitType, units.localUnit);
      if (built.error) return toast.error(t(built.error));
      if (model !== row?.price?.model && prices.some((p) => p.model === model)) return toast.error(t('pricing_edit.modelNameRe'));
      price = built.price;
    }

    setSaving(true);
    const failures = [];
    let saved = false;
    let catalogOk = true;
    if (saveCatalog) {
      const payload = buildCatalogPayload(values, originalCapabilities);
      const result = await callApi(() =>
        editId ? API.put('/api/model_info/', { ...payload, id: parseInt(editId, 10) }) : API.post('/api/model_info/', payload)
      );
      catalogOk = result.ok;
      saved ||= result.ok;
      if (!result.ok) failures.push(t('modelInfoPage.catalogNotSaved', { message: result.message }));
    }
    if (savePrice) {
      // 目录改名失败时价格留在原名下。有价格的传原模型名、无价格传 [](全部新增)。
      const priceModel = catalogOk ? model : row?.model || model;
      const result = await callApi(() =>
        API.post('/api/prices/multiple', {
          original_models: hasPrice ? [row.model] : [],
          models: [priceModel],
          price: { model: 'batch', ...price }
        })
      );
      saved ||= result.ok;
      if (!result.ok) failures.push(t('modelInfoPage.priceNotSaved', { message: result.message }));
    }
    setSaving(false);

    if (failures.length === 0) toast.success(t('common.saveSuccess'));
    failures.forEach((message) => toast.error(message));
    if (saved) onSaved?.();
  };

  const availableModelOptions = modelOptions.filter((o) => !existingModels.includes(o) || o === values.model);

  const selectedCapabilities = safeJsonArray(values.capabilities);
  const toggleCapability = (cap, on) => {
    const next = CAPABILITY_VALUES.filter((v) => (v === cap ? on : selectedCapabilities.includes(v)));
    setField('capabilities', JSON.stringify(next));
  };

  const selectedEndpoints = safeJsonArray(values.endpoints);
  const toggleEndpoint = (endpoint) => {
    const on = !selectedEndpoints.includes(endpoint);
    const next = ENDPOINT_VALUES.filter((v) => (v === endpoint ? on : selectedEndpoints.includes(v)));
    setField('endpoints', next.length === 0 ? '' : JSON.stringify(next));
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{editId ? t('modelInfoPage.editTitle') : t('modelInfoPage.createTitle')}</SheetTitle>
          {row ? (
            <SheetDescription className="flex flex-wrap items-center gap-2">
              <span className="font-mono text-foreground">{row.model}</span>
              <ModelStatusBadge status={modelStatus(row)} />
              <span>
                {t('modelsPage.channelCount', { count: enabledChannels(row).length })} ·{' '}
                {t('modelsPage.sheet.groupCount', { count: modelGroups(row).length })}
              </span>
            </SheetDescription>
          ) : (
            <SheetDescription>{t('modelInfoPage.subtitle')}</SheetDescription>
          )}
          <Tabs value={tab} onValueChange={setTab} className="pt-2">
            <TabsList>
              <TabsTrigger value="profile">{t('modelsPage.sheet.tabProfile')}</TabsTrigger>
              <TabsTrigger value="pricing">{t('modelsPage.sheet.tabPricing')}</TabsTrigger>
              {row && (
                <TabsTrigger value="channels">
                  {t('modelsPage.channelsTab')} ({(row.bound_channels || []).length})
                </TabsTrigger>
              )}
            </TabsList>
          </Tabs>
        </SheetHeader>
        <SheetBody>
          <div role="tabpanel" hidden={tab !== 'profile'} className="space-y-8">
            <Section title={t('modelInfoPage.sectionBasic')}>
              <FormField id="modelinfo-model" label={t('modelInfoPage.model')} required error={errors.model}>
                <Input
                  id="modelinfo-model"
                  list="model-info-options"
                  value={values.model}
                  onChange={(e) => {
                    setField('model', e.target.value);
                    if (e.target.value && !values.name) setField('name', e.target.value);
                  }}
                />
                <datalist id="model-info-options">
                  {availableModelOptions.map((o) => (
                    <option key={o} value={o} />
                  ))}
                </datalist>
              </FormField>

              <FormField id="modelinfo-name" label={t('modelInfoPage.name')} required error={errors.name}>
                <Input id="modelinfo-name" value={values.name} onChange={(e) => setField('name', e.target.value)} />
              </FormField>

              <FormField id="modelinfo-description" label={t('modelInfoPage.description')}>
                <Textarea
                  id="modelinfo-description"
                  rows={3}
                  value={values.description || ''}
                  onChange={(e) => setField('description', e.target.value)}
                />
              </FormField>

              <FormField label={t('modelInfoPage.vendor')} help={t('modelInfoPage.vendorTip')}>
                <Select
                  value={values.vendor_id ? String(values.vendor_id) : VENDOR_NONE}
                  onValueChange={(v) => setField('vendor_id', v === VENDOR_NONE ? 0 : parseInt(v, 10))}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t('modelInfoPage.vendorUnknown')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={VENDOR_NONE}>{t('modelInfoPage.vendorUnknown')}</SelectItem>
                    {vendorOptions.map((v) => (
                      <SelectItem key={v.id} value={String(v.id)}>
                        {v.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>

              <FormField label={t('modelInfoPage.tags')}>
                <TagInput value={values.tags} onChange={(v) => setField('tags', v)} />
              </FormField>

              <FormField
                id="modelinfo-alias-of"
                label={t('modelInfoPage.aliasOf')}
                help={t('modelInfoPage.aliasOfTip')}
                error={errors.alias_of}
              >
                <Input
                  id="modelinfo-alias-of"
                  list="model-info-options"
                  value={values.alias_of || ''}
                  onChange={(e) => setField('alias_of', e.target.value)}
                />
              </FormField>

              <FormField row label={t('modelInfoPage.hidden')} help={t('modelInfoPage.hiddenTip')} controlClassName="w-auto">
                <Switch checked={!!values.hidden} onCheckedChange={(v) => setField('hidden', v)} />
              </FormField>

              <FormField row label={t('modelInfoPage.locked')} help={t('modelInfoPage.lockedTip')} controlClassName="w-auto">
                <Switch checked={!!values.locked} onCheckedChange={(v) => setField('locked', v)} />
              </FormField>
            </Section>

            <Section title={t('modelInfoPage.sectionEndpoints')}>
              <FormField label={t('modelInfoPage.endpoints')} help={t('modelInfoPage.endpointsTip')}>
                <div className="flex flex-wrap gap-2">
                  {ENDPOINT_VALUES.map((endpoint) => {
                    const active = selectedEndpoints.includes(endpoint);
                    return (
                      <button key={endpoint} type="button" onClick={() => toggleEndpoint(endpoint)}>
                        <Badge variant={active ? 'secondary' : 'outline'} className={active ? '' : 'text-muted-foreground'}>
                          {t(ENDPOINT_LABEL_KEYS[endpoint])}
                        </Badge>
                      </button>
                    );
                  })}
                </div>
              </FormField>

              <FormField label={t('modelInfoPage.mode')} help={t('modelInfoPage.modeTip')}>
                <Select value={values.mode || ''} onValueChange={(v) => setField('mode', v)}>
                  <SelectTrigger>
                    <SelectValue placeholder={t('modelInfoPage.modeOptions.unset')} />
                  </SelectTrigger>
                  <SelectContent>
                    {MODE_VALUES.map((m) => (
                      <SelectItem key={m || 'unset'} value={m}>
                        {t(MODE_LABEL_KEYS[m])}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>

              <FormField label={t('modelInfoPage.capabilities')} help={t('modelInfoPage.capabilitiesTip')}>
                <div className="space-y-3">
                  {CAPABILITY_VALUES.map((cap) => (
                    <div key={cap} className="flex items-center justify-between gap-4">
                      <span className="text-sm">{t(CAPABILITY_LABEL_KEYS[cap])}</span>
                      <Switch checked={selectedCapabilities.includes(cap)} onCheckedChange={(v) => toggleCapability(cap, v)} />
                    </div>
                  ))}
                </div>
              </FormField>
            </Section>

            <Section title={t('modelInfoPage.sectionLimits')}>
              <FormField label={t('modelInfoPage.inputModalities')}>
                <ModalityPicker value={values.input_modalities} onChange={(v) => setField('input_modalities', v)} />
              </FormField>

              <FormField label={t('modelInfoPage.outputModalities')}>
                <ModalityPicker value={values.output_modalities} onChange={(v) => setField('output_modalities', v)} />
              </FormField>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField id="modelinfo-context-length" label={t('modelInfoPage.contextLength')} required error={errors.context_length}>
                  <Input
                    id="modelinfo-context-length"
                    type="number"
                    value={values.context_length}
                    onChange={(e) => setField('context_length', e.target.value)}
                  />
                </FormField>
                <FormField id="modelinfo-max-tokens" label={t('modelInfoPage.maxTokens')} required error={errors.max_tokens}>
                  <Input
                    id="modelinfo-max-tokens"
                    type="number"
                    value={values.max_tokens}
                    onChange={(e) => setField('max_tokens', e.target.value)}
                  />
                </FormField>
              </div>
            </Section>

            {editId > 0 && (
              <Section title={t('modelInfoPage.sectionSource')}>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <FormField label={t('modelInfoPage.source')}>
                    <p className="text-sm text-muted-foreground">{t(SOURCE_LABEL_KEYS[values.source] || 'modelInfoPage.sourceUnknown')}</p>
                  </FormField>
                  <FormField label={t('modelInfoPage.syncedAt')}>
                    <p className="text-sm text-muted-foreground">{values.synced_at ? timestamp2string(values.synced_at) : '—'}</p>
                  </FormField>
                </div>
              </Section>
            )}
          </div>

          <div role="tabpanel" hidden={tab !== 'pricing'} className="space-y-8">
            <Section title={t('modelInfoPage.sectionPrice')}>
              {!hasPrice && <p className="text-sm text-muted-foreground">{t('modelInfoPage.priceEmptyTip')}</p>}
              <PriceFields control={control} type={priceType} ownedby={ownedby} units={units} />
            </Section>
          </div>

          {row && (
            <div role="tabpanel" hidden={tab !== 'channels'}>
              <ChannelsPanel row={row} />
            </div>
          )}
        </SheetBody>
        <SheetFooter>
          {tab === 'channels' && <span className="mr-auto text-xs text-muted-foreground">{t('modelsPage.sheet.channelsReadonly')}</span>}
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

ModelInfoSheet.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  row: PropTypes.object,
  initialTab: PropTypes.oneOf(SHEET_TABS),
  existingModels: PropTypes.array,
  prices: PropTypes.array,
  ownedby: PropTypes.array,
  unit: PropTypes.string,
  onSaved: PropTypes.func
};
