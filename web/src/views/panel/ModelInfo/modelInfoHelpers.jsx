import { useTranslation } from 'react-i18next';

import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import BrandIcon from '@/components/brand/BrandIcon';
import ModelIcon from '@/components/brand/ModelIcon';
import { resolveBrandFromText } from '@/components/brand/brandIcons';
import { cn } from '@/lib/utils';
import { MODALITY_OPTIONS } from 'constants/Modality';
import { CAPABILITY_OPTIONS } from 'constants/Capability';

export const MODALITY_COLORS = {
  text: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
  image: 'bg-sky-500/15 text-sky-600 dark:text-sky-400',
  audio: 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
  music: 'bg-rose-500/15 text-rose-600 dark:text-rose-400',
  video: 'bg-slate-500/15 text-slate-600 dark:text-slate-400',
  file: 'bg-violet-500/15 text-violet-600 dark:text-violet-400',
  speech: 'bg-orange-500/15 text-orange-600 dark:text-orange-400',
  transcription: 'bg-teal-500/15 text-teal-600 dark:text-teal-400'
};

export const MODALITY_VALUES = Object.values(MODALITY_OPTIONS).map((o) => o.value);

// 任务类型枚举:空值表示未设置,沿用系统内置名单。
export const MODE_VALUES = ['', 'chat', 'image', 'chat_image'];

export const MODE_LABEL_KEYS = {
  '': 'modelInfoPage.modeOptions.unset',
  chat: 'modelInfoPage.modeOptions.chat',
  image: 'modelInfoPage.modeOptions.image',
  chat_image: 'modelInfoPage.modeOptions.chat_image'
};

// 能力词表:顺序即展示与保存顺序;空串表示「未知」,不展示任何徽标。
export const CAPABILITY_VALUES = Object.values(CAPABILITY_OPTIONS).map((o) => o.value);

export const CAPABILITY_LABEL_KEYS = {
  tool_call: 'modelInfoPage.capabilityOptions.tool_call',
  reasoning: 'modelInfoPage.capabilityOptions.reasoning',
  structured_output: 'modelInfoPage.capabilityOptions.structured_output'
};

// 接口能力词表：取值与后端 model_info.endpoints 一致；空串表示未设置。
export const ENDPOINT_VALUES = ['chat', 'responses', 'images', 'audio.speech', 'audio.transcription', 'embeddings', 'rerank'];

export const ENDPOINT_LABEL_KEYS = {
  chat: 'modelInfoPage.endpointOptions.chat',
  responses: 'modelInfoPage.endpointOptions.responses',
  images: 'modelInfoPage.endpointOptions.images',
  'audio.speech': 'modelInfoPage.endpointOptions.audioSpeech',
  'audio.transcription': 'modelInfoPage.endpointOptions.audioTranscription',
  embeddings: 'modelInfoPage.endpointOptions.embeddings',
  rerank: 'modelInfoPage.endpointOptions.rerank'
};

export const modalityText = (value) => MODALITY_OPTIONS[value]?.text || value;

export function safeJsonArray(jsonString) {
  try {
    const parsed = JSON.parse(jsonString);
    return Array.isArray(parsed) ? parsed : [];
  } catch (e) {
    return [];
  }
}

// 输入或输出模态解析后为空数组即视为「未标注」(容错空串 / 非法 JSON)。
export function isUnlabeled(info) {
  if (!info) return false;
  return safeJsonArray(info.input_modalities).length === 0 || safeJsonArray(info.output_modalities).length === 0;
}

// 徽章单行不折行:最多显示 max 个,其余收进 `+N`,悬停看全部。
// items: [{ key, label, className?, icon? }]
export function BadgeOverflow({ items, max = 2, variant = 'secondary' }) {
  if (!items.length) return <span className="text-muted-foreground">-</span>;
  const shown = items.slice(0, max);
  const rest = items.length - shown.length;
  const renderBadge = (item) => {
    const Icon = item.icon;
    return (
      <Badge
        key={item.key}
        variant={variant}
        className={`shrink-0 gap-1 whitespace-nowrap px-1.5 py-0 font-normal ${item.className || ''}`}
      >
        {Icon && <Icon className="size-3" />}
        {item.label}
      </Badge>
    );
  };
  return (
    <div className="flex flex-nowrap items-center gap-1">
      {shown.map(renderBadge)}
      {rest > 0 && (
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <Badge variant="outline" className="shrink-0 cursor-default px-1.5 py-0 font-normal tabular-nums text-muted-foreground">
                +{rest}
              </Badge>
            </TooltipTrigger>
            <TooltipContent>
              <div className="flex max-w-xs flex-wrap gap-1">{items.map(renderBadge)}</div>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      )}
    </div>
  );
}

const modalityItems = (list, t) =>
  list.map((m, i) => ({
    key: `${m}-${i}`,
    label: t(`modelpricePage.modality.${m}`, { defaultValue: modalityText(m) }),
    className: `border-transparent ${MODALITY_COLORS[m] || 'bg-muted'}`
  }));

// 输入 → 输出模态合并在一格;任一侧为空显示「未标注」。
export function ModalityBadges({ input, output, max = 3 }) {
  const { t } = useTranslation();
  const inList = safeJsonArray(input);
  const outList = safeJsonArray(output);
  if (inList.length === 0 || outList.length === 0) {
    return (
      <Badge variant="outline" className="whitespace-nowrap border-transparent bg-destructive/15 px-1.5 py-0 text-destructive">
        {t('modelInfoPage.unlabeled')}
      </Badge>
    );
  }
  return (
    <div className="flex flex-nowrap items-center gap-1">
      <BadgeOverflow items={modalityItems(inList, t)} max={max} variant="outline" />
      <span className="text-muted-foreground">→</span>
      <BadgeOverflow items={modalityItems(outList, t)} max={max} variant="outline" />
    </div>
  );
}

export function CapabilityBadges({ json, max = 2 }) {
  const { t } = useTranslation();
  const items = safeJsonArray(json).map((cap, i) => ({
    key: `${cap}-${i}`,
    label: CAPABILITY_LABEL_KEYS[cap] ? t(CAPABILITY_LABEL_KEYS[cap]) : cap,
    icon: CAPABILITY_OPTIONS[cap]?.icon
  }));
  return <BadgeOverflow items={items} max={max} />;
}

export function EndpointBadges({ json, max = 2 }) {
  const { t } = useTranslation();
  const items = safeJsonArray(json).map((endpoint, i) => ({
    key: `${endpoint}-${i}`,
    label: ENDPOINT_LABEL_KEYS[endpoint] ? t(ENDPOINT_LABEL_KEYS[endpoint]) : endpoint
  }));
  return <BadgeOverflow items={items} max={max} />;
}

// 列表状态徽章三态(可用 / 无渠道 / 未定价),取值见 modelCatalog.modelStatus。
export const MODEL_STATUS_LABEL_KEYS = {
  ok: 'modelsPage.status.ok',
  nochannel: 'modelsPage.status.nochannel',
  unpriced: 'modelsPage.status.unpriced'
};

const MODEL_STATUS_COLORS = {
  ok: 'bg-success/15 text-success',
  nochannel: 'bg-warning/15 text-warning',
  unpriced: 'bg-warning/15 text-warning'
};

const MODEL_STATUS_DOTS = { ok: 'bg-success', nochannel: 'bg-warning', unpriced: 'bg-warning' };

export function ModelStatusBadge({ status }) {
  const { t } = useTranslation();
  return (
    <Badge
      variant="outline"
      className={cn('gap-1.5 whitespace-nowrap border-transparent px-1.5 py-0 font-normal', MODEL_STATUS_COLORS[status])}
    >
      <span className={cn('size-1.5 rounded-full', MODEL_STATUS_DOTS[status])} />
      {t(MODEL_STATUS_LABEL_KEYS[status] || MODEL_STATUS_LABEL_KEYS.ok)}
    </Badge>
  );
}

// 厂商图标:厂商配了图标用图标,否则按 slug / 名称匹配品牌标;无厂商时按模型名推断。
export function VendorIcon({ vendor, model, className = 'size-4' }) {
  if (vendor)
    return (
      <BrandIcon
        icon={vendor.icon}
        brandKey={resolveBrandFromText(vendor.slug || vendor.name)}
        ownedBy={vendor.slug || vendor.name}
        fallbackText={vendor.name}
        className={className}
      />
    );
  return <ModelIcon model={model} className={className} />;
}

export function TagBadges({ json, max = 2 }) {
  const items = safeJsonArray(json).map((tag, i) => ({ key: `${tag}-${i}`, label: tag }));
  return <BadgeOverflow items={items} max={max} />;
}
