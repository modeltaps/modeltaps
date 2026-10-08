import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ArrowDown, Copy, Flame } from 'lucide-react';

import { cn } from '@/lib/utils';
import BrandIcon from '@/components/brand/BrandIcon';
import { Card } from '@/components/ui/card';
import { toast } from '@/components/ui/sonner';
import { MODALITY_OPTIONS } from 'constants/Modality';
import { CAPABILITY_OPTIONS } from 'constants/Capability';

const copyText = (text, label) => {
  try {
    navigator.clipboard.writeText(text);
    toast.success(`${label} ✓`);
  } catch (e) {
    toast.error(`${label}: ${text}`);
  }
};

const TONES = {
  primary: 'bg-muted text-foreground',
  success: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  warning: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  info: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  error: 'bg-red-500/10 text-red-600 dark:text-red-400',
  secondary: 'bg-slate-500/10 text-slate-600 dark:text-slate-400',
  default: 'bg-muted text-muted-foreground'
};

const OUTLINE_TONES = {
  success: 'border-emerald-500/40 text-emerald-600 dark:text-emerald-400',
  warning: 'border-amber-500/40 text-amber-600 dark:text-amber-400'
};

function Pill({ tone = 'default', outline = false, className, title, children }) {
  return (
    <span
      title={title}
      className={cn(
        'inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[0.7rem] font-medium',
        outline ? cn('border bg-transparent', OUTLINE_TONES[tone] || OUTLINE_TONES.success) : TONES[tone] || TONES.default,
        className
      )}
    >
      {children}
    </span>
  );
}

Pill.propTypes = {
  tone: PropTypes.string,
  outline: PropTypes.bool,
  className: PropTypes.string,
  title: PropTypes.string,
  children: PropTypes.node
};

const parseJson = (str) => {
  try {
    return JSON.parse(str || '[]');
  } catch (e) {
    return [];
  }
};

// 上下文长度紧凑格式：128000 → 128K，1000000 → 1M
const formatContext = (ctx) => {
  if (!ctx || ctx <= 0) return null;
  if (ctx >= 1000000) return `${Number((ctx / 1000000).toFixed(1))}M`;
  if (ctx >= 1000) return `${Math.round(ctx / 1000)}K`;
  return `${ctx}`;
};

export default function ModelCard({ model, provider, modelInfo, price, group, ownedbyIcon, type, formatPrice, onViewDetail }) {
  const { t } = useTranslation();
  const inputModalities = modelInfo ? parseJson(modelInfo.input_modalities) : [];
  const outputModalities = modelInfo ? parseJson(modelInfo.output_modalities) : [];
  const inputOnlyModalities = inputModalities.filter((m) => m !== 'text' && !outputModalities.includes(m));
  // capabilities 为空串 / 缺失时解析为空数组，此时不渲染能力徽标。
  const capabilities = modelInfo ? parseJson(modelInfo.capabilities).filter((c) => CAPABILITY_OPTIONS[c]) : [];
  const tags = modelInfo ? parseJson(modelInfo.tags) : [];
  const hot = tags.some((tag) => tag.toLowerCase() === 'hot');
  const otherTags = tags.filter((tag) => tag.toLowerCase() !== 'hot');
  const isPriceAvailable = typeof price.input === 'number' && typeof price.output === 'number';
  const ctx = formatContext(modelInfo?.context_length);

  return (
    <Card onClick={onViewDetail} className="cursor-pointer p-4 transition-all hover:border-foreground/30 hover:shadow-md">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <BrandIcon icon={ownedbyIcon} ownedBy={provider} model={model} fallbackText={provider} className="size-5" />
            <span className="text-sm font-semibold" title={model}>
              {model}
            </span>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                copyText(model, t('modelpricePage.modelId'));
              }}
              className="text-muted-foreground opacity-60 hover:opacity-100"
              title={t('modelpricePage.copyModelId')}
            >
              <Copy className="size-4" />
            </button>
            {hot && (
              <Pill tone="error" className="font-semibold">
                <Flame className="size-3" /> HOT
              </Pill>
            )}
            {modelInfo && outputModalities.length === 0 ? (
              <Pill tone="default" className="border border-border/60" title={t('modelpricePage.outputModality')}>
                {t('modelpricePage.unlabeledModality')}
              </Pill>
            ) : (
              outputModalities.map((m, i) => (
                <Pill key={`out-${i}`} tone={MODALITY_OPTIONS[m]?.color || 'primary'}>
                  {t(`modelpricePage.modality.${m}`, { defaultValue: MODALITY_OPTIONS[m]?.text || m })}
                </Pill>
              ))
            )}
            {inputOnlyModalities.map((m, i) => (
              <Pill key={`in-${i}`} tone="default" className="border border-border/60" title={t('modelpricePage.inputModality')}>
                <ArrowDown className="size-3" />
                {t(`modelpricePage.modality.${m}`, { defaultValue: MODALITY_OPTIONS[m]?.text || m })}
              </Pill>
            ))}
            {capabilities.map((c) => {
              const Icon = CAPABILITY_OPTIONS[c].icon;
              return (
                <Pill key={`cap-${c}`} tone={CAPABILITY_OPTIONS[c].color} title={t('modelpricePage.capabilities')}>
                  <Icon className="size-3" />
                  {t(`modelpricePage.capability.${c}`, { defaultValue: CAPABILITY_OPTIONS[c].text })}
                </Pill>
              );
            })}
            {otherTags.map((tag) => (
              <Pill key={tag} tone="default">
                {tag}
              </Pill>
            ))}
          </div>

          {modelInfo?.description && (
            <p className="mt-1.5 line-clamp-2 text-[0.8125rem] leading-relaxed text-muted-foreground">{modelInfo.description}</p>
          )}

          <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
            <span>
              {t('modelpricePage.by')} {provider}
            </span>
            {ctx && (
              <>
                <span className="text-border">·</span>
                <span>
                  {ctx} {t('modelpricePage.contextLabel')}
                </span>
              </>
            )}
            {price.unconfigured ? (
              <>
                <span className="text-border">·</span>
                <span className="text-amber-600 dark:text-amber-400">
                  {t('modelpricePage.priceUnconfigured')}
                </span>
              </>
            ) : isPriceAvailable ? (
              <>
                <span className="text-border">·</span>
                <span className="text-emerald-600 dark:text-emerald-400">
                  {t('modelpricePage.input')} {formatPrice(price.input, type)}
                </span>
                <span className="text-border">·</span>
                <span className="text-amber-600 dark:text-amber-400">
                  {t('modelpricePage.output')} {formatPrice(price.output, type)}
                </span>
              </>
            ) : (
              typeof price.input === 'string' && (
                <>
                  <span className="text-border">·</span>
                  <span>{price.input}</span>
                </>
              )
            )}
          </div>
        </div>

        {group && (
          <div className="flex shrink-0 items-center gap-1">
            {group.ratio > 0 ? (
              <Pill tone={group.ratio > 1 ? 'warning' : 'info'} className="font-semibold">
                x{group.ratio}
              </Pill>
            ) : (
              <Pill tone="success" className="font-semibold">
                {t('modelpricePage.free')}
              </Pill>
            )}
          </div>
        )}
      </div>
    </Card>
  );
}

ModelCard.propTypes = {
  model: PropTypes.string.isRequired,
  provider: PropTypes.string.isRequired,
  modelInfo: PropTypes.object,
  price: PropTypes.object.isRequired,
  group: PropTypes.object,
  ownedbyIcon: PropTypes.string,
  unit: PropTypes.string,
  type: PropTypes.string,
  formatPrice: PropTypes.func,
  onViewDetail: PropTypes.func
};
