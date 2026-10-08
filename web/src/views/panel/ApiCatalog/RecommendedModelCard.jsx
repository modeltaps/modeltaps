import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { ArrowRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ValueFormatter } from 'utils/common';

// ==============================|| API CATALOG — RECOMMENDED MODEL ||============================== //
// 页面推荐模型卡：名称 / 上下文 / 输入输出价 / 能力标签。
// 价格按基准分组（倍率 x1）折算为每 1M tokens 的美元参考价，精确到分组的价格见模型广场。

const formatContext = (length) => {
  if (!length || length <= 0) return null;
  if (length >= 1000000) return `${Math.round(length / 100000) / 10}M`;
  if (length >= 1000) return `${Math.round(length / 1000)}K`;
  return String(length);
};

// 按次计费给出「单次价格」，按量计费给出每 1M tokens 的参考价：价格数字不带单位
// 会让按次与按量看起来是同一口径。0 一律显示为本地化的「免费」。
const formatPrice = (value, type, t) => {
  if (typeof value !== 'number') return null;
  if (value === 0) return t('modelpricePage.free');
  return type === 'times' ? ValueFormatter(value, true, false) : `${ValueFormatter(value, true, true)} / 1M tokens`;
};

function Field({ label, value }) {
  if (!value) return null;
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-sm font-medium tabular-nums">{value}</span>
    </div>
  );
}

Field.propTypes = {
  label: PropTypes.node,
  value: PropTypes.node
};

// 模型名 + 厂商 + 上下文 / 输入输出价：推荐模型卡与概览页的模态卡共用同一份格式化。
export function ModelFacts({ model }) {
  const { t } = useTranslation();

  if (!model) return null;

  const priceType = model.price?.type;

  return (
    <>
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-sm font-semibold">{model.id}</span>
        {model.ownedBy && <span className="rounded bg-muted px-1.5 py-0.5 text-[0.6875rem] text-muted-foreground">{model.ownedBy}</span>}
      </div>

      <div className="flex flex-wrap gap-x-8 gap-y-2">
        <Field label={t('apiCatalogPage.contextLength')} value={formatContext(model.info?.contextLength)} />
        {/* 按次计费只有一个价格，拆成「输入价 / 输出价」两列会让人以为要付两次。 */}
        {priceType === 'times' ? (
          <Field label={t('modelpricePage.timesPrice')} value={formatPrice(model.price?.input, priceType, t)} />
        ) : (
          <>
            <Field label={t('apiCatalogPage.inputPrice')} value={formatPrice(model.price?.input, priceType, t)} />
            <Field label={t('apiCatalogPage.outputPrice')} value={formatPrice(model.price?.output, priceType, t)} />
          </>
        )}
      </div>
    </>
  );
}

ModelFacts.propTypes = {
  model: PropTypes.object
};

export default function RecommendedModelCard({ model, onOpenPlayground }) {
  const { t } = useTranslation();

  if (!model) return null;

  const capabilities = model.info?.capabilities || [];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('apiCatalogPage.recommended')}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <ModelFacts model={model} />

        {capabilities.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {capabilities.map((cap) => (
              <span key={cap} className="rounded bg-muted px-1.5 py-0.5 text-[0.6875rem] font-medium text-muted-foreground">
                {t(`modelpricePage.capability.${cap}`, { defaultValue: cap })}
              </span>
            ))}
          </div>
        )}

        <p className="text-xs text-muted-foreground">{t('apiCatalogPage.priceNote')}</p>

        <div className="flex flex-wrap items-center gap-2">
          <Button asChild variant="outline" size="sm">
            <Link to="/panel/model_price">
              {t('apiCatalogPage.viewModel')}
              <ArrowRight className="size-4" />
            </Link>
          </Button>
          {onOpenPlayground && (
            <Button variant="outline" size="sm" onClick={onOpenPlayground}>
              {t('apiCatalogPage.openPlayground')}
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

RecommendedModelCard.propTypes = {
  model: PropTypes.object,
  onOpenPlayground: PropTypes.func
};
