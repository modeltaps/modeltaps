import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  Copy,
  X,
  Info,
  Package,
  FileText,
  Maximize2,
  ArrowDown,
  ArrowUp,
  Tags,
  DollarSign,
  BarChart3,
  Flame,
  Sparkles
} from 'lucide-react';

import { cn } from '@/lib/utils';
import BrandIcon from '@/components/brand/BrandIcon';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { toast } from '@/components/ui/sonner';
import { MODALITY_OPTIONS } from 'constants/Modality';
import { CAPABILITY_OPTIONS } from 'constants/Capability';

const copyText = (text, label) => {
  try {
    navigator.clipboard.writeText(text);
    toast.success(`${label} \u2713`);
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

function Pill({ tone = 'default', outline = false, className, children }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium',
        outline ? cn('border bg-transparent', OUTLINE_TONES[tone] || OUTLINE_TONES.success) : TONES[tone] || TONES.default,
        className
      )}
    >
      {children}
    </span>
  );
}
Pill.propTypes = { tone: PropTypes.string, outline: PropTypes.bool, className: PropTypes.string, children: PropTypes.node };

function InfoRow({ icon, label, children }) {
  return (
    <div className="flex items-center gap-3">
      {icon}
      <span className="min-w-20 text-sm text-muted-foreground">{label}:</span>
      <div className="flex flex-wrap items-center gap-1">{children}</div>
    </div>
  );
}
InfoRow.propTypes = { icon: PropTypes.node, label: PropTypes.string, children: PropTypes.node };

const parseJson = (str) => {
  try {
    return JSON.parse(str || '[]');
  } catch (e) {
    return [];
  }
};

export default function ModelDetailModal({ open, onClose, model, provider, modelInfo, priceData, ownedbyIcon, formatPrice }) {
  const { t } = useTranslation();
  if (!model) return null;

  const inputModalities = modelInfo ? parseJson(modelInfo.input_modalities) : [];
  const outputModalities = modelInfo ? parseJson(modelInfo.output_modalities) : [];
  const tags = modelInfo ? parseJson(modelInfo.tags) : [];
  // capabilities 为空串 / 缺失时解析为空数组，此时不渲染能力行。
  const capabilities = modelInfo ? parseJson(modelInfo.capabilities).filter((c) => CAPABILITY_OPTIONS[c]) : [];
  const iconCls = 'size-[18px] shrink-0 text-muted-foreground';

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-3xl">
        <div className="flex items-start justify-between gap-3 border-b border-border p-6">
          <div className="flex items-center gap-3">
            <BrandIcon icon={ownedbyIcon} ownedBy={provider} model={model} fallbackText={provider} className="size-12" />
            <div>
              <h2 className="mb-0.5 text-xl font-semibold">{modelInfo?.name || model}</h2>
              <div className="flex items-center gap-1.5">
                <span className="text-sm text-muted-foreground">{model}</span>
                <button
                  type="button"
                  onClick={() => copyText(model, t('modelpricePage.modelId'))}
                  className="text-muted-foreground hover:text-foreground"
                >
                  <Copy className="size-4" />
                </button>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {tags.includes('Hot') && (
              <Pill tone="error" className="font-semibold">
                <Flame className="size-4" /> Hot
              </Pill>
            )}
            <button type="button" onClick={onClose} className="text-muted-foreground hover:text-foreground">
              <X className="size-5" />
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          {modelInfo?.description && <p className="mb-5 leading-relaxed text-muted-foreground">{modelInfo.description}</p>}

          <div className="mb-5">
            <div className="mb-3 flex items-center gap-2">
              <Info className="size-5" />
              <h3 className="font-semibold">{t('modelpricePage.modelInfo')}</h3>
            </div>
            <div className="space-y-3">
              <InfoRow icon={<Package className={iconCls} />} label={t('modelpricePage.type')}>
                <Pill tone="primary">{priceData?.price?.type === 'tokens' ? t('modelpricePage.tokens') : t('modelpricePage.times')}</Pill>
              </InfoRow>
              {modelInfo?.context_length > 0 && (
                <InfoRow icon={<FileText className={iconCls} />} label={t('modelpricePage.contextLength')}>
                  <span className="text-sm font-medium">{modelInfo.context_length.toLocaleString()}</span>
                </InfoRow>
              )}
              {modelInfo?.max_tokens > 0 && (
                <InfoRow icon={<Maximize2 className={iconCls} />} label={t('modelpricePage.maxTokens')}>
                  <span className="text-sm font-medium">{modelInfo.max_tokens.toLocaleString()}</span>
                </InfoRow>
              )}
              {modelInfo && (
                <InfoRow icon={<ArrowDown className={iconCls} />} label={t('modelpricePage.inputModality')}>
                  {inputModalities.length > 0 ? (
                    inputModalities.map((m, i) => (
                      <Pill key={i} tone={MODALITY_OPTIONS[m]?.color || 'primary'}>
                        {t(`modelpricePage.modality.${m}`, { defaultValue: MODALITY_OPTIONS[m]?.text || m })}
                      </Pill>
                    ))
                  ) : (
                    <span className="text-sm text-muted-foreground">{t('modelpricePage.unlabeledModality')}</span>
                  )}
                </InfoRow>
              )}
              {modelInfo && (
                <InfoRow icon={<ArrowUp className={iconCls} />} label={t('modelpricePage.outputModality')}>
                  {outputModalities.length > 0 ? (
                    outputModalities.map((m, i) => (
                      <Pill key={i} tone={MODALITY_OPTIONS[m]?.color || 'secondary'}>
                        {t(`modelpricePage.modality.${m}`, { defaultValue: MODALITY_OPTIONS[m]?.text || m })}
                      </Pill>
                    ))
                  ) : (
                    <span className="text-sm text-muted-foreground">{t('modelpricePage.unlabeledModality')}</span>
                  )}
                </InfoRow>
              )}
              {capabilities.length > 0 && (
                <InfoRow icon={<Sparkles className={iconCls} />} label={t('modelpricePage.capabilities')}>
                  {capabilities.map((c) => {
                    const Icon = CAPABILITY_OPTIONS[c].icon;
                    return (
                      <Pill key={c} tone={CAPABILITY_OPTIONS[c].color}>
                        <Icon className="size-3.5" />
                        {t(`modelpricePage.capability.${c}`, { defaultValue: CAPABILITY_OPTIONS[c].text })}
                      </Pill>
                    );
                  })}
                </InfoRow>
              )}
              {tags.length > 0 && (
                <InfoRow icon={<Tags className={iconCls} />} label={t('modelpricePage.tags')}>
                  {tags.map((tag, i) => (
                    <Pill key={i} tone="default">
                      {tag}
                    </Pill>
                  ))}
                </InfoRow>
              )}
            </div>
          </div>

          <div className="mb-2">
            <div className="mb-3 flex items-center gap-2">
              <DollarSign className="size-5" />
              <h3 className="font-semibold">{t('modelpricePage.priceDetails')}</h3>
            </div>
            <p className="mb-3 text-sm leading-relaxed text-muted-foreground">{t('modelpricePage.priceNote')}</p>
            <div className="overflow-hidden rounded-lg border border-border">
              <Table>
                <TableHeader>
                  <TableRow className="bg-muted/40 hover:bg-muted/40">
                    <TableHead className="normal-case">{t('modelpricePage.group')}</TableHead>
                    <TableHead className="normal-case">{t('modelpricePage.input')}</TableHead>
                    <TableHead className="normal-case">{t('modelpricePage.output')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {priceData?.allGroupPrices?.map((gp, i) => (
                    <TableRow key={i}>
                      <TableCell>
                        <Pill tone="primary">{gp.groupName}</Pill>
                      </TableCell>
                      {gp.unconfigured ? (
                        <TableCell colSpan={2}>
                          <Pill tone="warning" outline>
                            {t('modelpricePage.priceUnconfigured')}
                          </Pill>
                        </TableCell>
                      ) : (
                        <>
                          <TableCell>
                            <Pill tone="success" outline>
                              {formatPrice(gp.input, gp?.type)}
                            </Pill>
                          </TableCell>
                          <TableCell>
                            <Pill tone="warning" outline>
                              {formatPrice(gp.output, gp?.type)}
                            </Pill>
                          </TableCell>
                        </>
                      )}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>

          {priceData?.price?.extra_ratios && Object.keys(priceData.price.extra_ratios).length > 0 && (
            <div className="mt-5">
              <div className="mb-3 flex items-center gap-2">
                <BarChart3 className="size-5" />
                <h3 className="font-semibold">{t('modelpricePage.otherInfo')}</h3>
              </div>
              <div className="space-y-1.5">
                {Object.entries(priceData.price.extra_ratios).map(([key, value]) => (
                  <div key={key} className="flex items-center gap-3">
                    <span className="min-w-20 text-sm text-muted-foreground">{t(`modelpricePage.${key}`)}:</span>
                    <span className="text-sm font-medium">{value}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

ModelDetailModal.propTypes = {
  open: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  model: PropTypes.string,
  provider: PropTypes.string,
  modelInfo: PropTypes.object,
  priceData: PropTypes.object,
  ownedbyIcon: PropTypes.string,
  userGroupMap: PropTypes.object,
  formatPrice: PropTypes.func,
  unit: PropTypes.string
};
