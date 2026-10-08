import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import CodeBlock from '@/components/ui/code-block';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from '@/components/ui/sonner';
import { capabilityEmptyKey, pickPreferredModel } from './availability';

// ==============================|| API CATALOG — CAPABILITY SECTION ||============================== //
// 单个能力的小节：说明 → 端点 → 模型选择器（只列真实可用的模型）→ 多语言代码 Tab。
// 没有可用模型时整节置灰并标「暂无可用模型」，示例仍按占位模型名渲染，便于对照文档。

// 代码 Tab 的语言顺序与 highlight.js 语言名（code-block 已注册 bash/python/javascript）。
// 概览页的快速开始示例共用同一份，保证两处的 Tab 顺序与标签一致。
export const LANGUAGES = [
  { id: 'curl', hljs: 'bash', label: 'cURL' },
  { id: 'python', hljs: 'python', label: 'Python' },
  { id: 'javascript', hljs: 'javascript', label: 'JavaScript' }
];

// 请求示例里的密钥 / 模型占位符：不读取用户真实 Key，取不到推荐模型时也给可读的占位模型名。
// 概览页与模态页共用这两个常量，避免同一串字面量在多处各写一遍。
export const API_KEY_PLACEHOLDER = 'sk-YOUR_TOKEN';
export const MODEL_PLACEHOLDER = 'your-model';

// 示例 Tab 行：概览页快速开始、能力小节、旧版渲染分支共用一份样式与交互。
export function ExampleTabs({ items, activeId, onSelect, className }) {
  return (
    <div className={cn('flex flex-wrap items-center gap-1', className)}>
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          onClick={() => onSelect(item.id)}
          className={cn(
            'rounded-md px-2.5 py-1 text-sm font-medium transition-colors',
            activeId === item.id ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'
          )}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}

ExampleTabs.propTypes = {
  items: PropTypes.array.isRequired,
  activeId: PropTypes.string,
  onSelect: PropTypes.func.isRequired,
  className: PropTypes.string
};

export default function CapabilitySection({ capability, models, baseUrl, apiKey, loading, error }) {
  const { t } = useTranslation();
  const [language, setLanguage] = useState(LANGUAGES[0].id);
  const [model, setModel] = useState('');

  const available = models.length > 0;
  // 没有可用模型时的原因文案与 chip、模态卡同源：planned 恒为「即将上线」，
  // 取数未完成是「加载中」，取数失败是「可用性未知」——不能把还没到 / 取不到的模型列表说成不支持。
  const emptyLabel = t(capabilityEmptyKey(capability, { loading, error }));
  // 推荐模型：catalog 指定的 preferredModels 优先，否则取可用列表首项。
  const preferred = pickPreferredModel(models, capability.preferredModels, capability.availability?.vendors)?.id || '';

  // 选中模型直接派生：用户选过且仍在可用列表里就用它，否则（首次取数完成 / 切站点）落到推荐模型。
  const selected = models.find((m) => m.id === model)?.id || preferred;

  const code = useMemo(() => {
    const build = capability.examples?.[language];
    if (typeof build !== 'function') return '';
    return build({ baseUrl, apiKey, model: selected || MODEL_PLACEHOLDER });
  }, [capability.examples, language, baseUrl, apiKey, selected]);

  const hljsLang = LANGUAGES.find((l) => l.id === language)?.hljs || 'bash';

  const labelId = `catalog-model-label-${capability.id}`;

  return (
    // 不可用的小节用虚线描边 + 弱化底色区分，不整块降透明度：降透明度会把正文与代码
    // 一起压到对比度不合格。
    <Card className={cn(!available && 'border-dashed bg-muted/30')}>
      <CardHeader className="gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle className="text-base">{t(capability.titleKey)}</CardTitle>
          {!available && (
            <span className="rounded bg-muted px-1.5 py-0.5 text-[0.6875rem] font-medium text-muted-foreground">{emptyLabel}</span>
          )}
        </div>
        <p className="text-sm text-muted-foreground">{t(capability.descKey)}</p>
        {/* 非 OpenAI 标准的写法(网关 / OpenRouter 扩展)在这里挑明,避免用户照 OpenAI 文档的直觉取字段。 */}
        {capability.noteKey && <p className="text-xs text-muted-foreground">{t(capability.noteKey)}</p>}
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[0.6875rem] font-semibold uppercase text-foreground">
            {capability.endpoint.method}
          </span>
          <code className="font-mono text-sm">{capability.endpoint.path}</code>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <span id={labelId} className="text-xs font-medium text-muted-foreground">
            {t('apiCatalogPage.model')}
          </span>
          {/* 没有可用模型时不渲染一个空的禁用下拉，直接说明原因。 */}
          {available ? (
            <div className="w-full max-w-xs">
              <Select value={selected} onValueChange={setModel}>
                <SelectTrigger aria-labelledby={labelId}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {models.map((m) => (
                    <SelectItem key={m.id} value={m.id}>
                      {m.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ) : (
            <span className="text-sm text-muted-foreground">{emptyLabel}</span>
          )}
          {/* 取数失败时模型数恒为 0，报出来等于把故障说成「本站没有」。 */}
          {!loading && !error && (
            <span className="text-xs text-muted-foreground">{t('apiCatalogPage.modelCount', { count: models.length })}</span>
          )}
        </div>

        <ExampleTabs items={LANGUAGES} activeId={language} onSelect={setLanguage} />

        <CodeBlock
          code={code}
          language={hljsLang}
          wrap
          maxHeightClass="max-h-96"
          copyLabel={t('common.copy')}
          copiedLabel={t('apiCatalogPage.copied')}
          onCopy={() => toast.success(t('apiCatalogPage.copied'))}
          onCopyError={() => toast.error(t('apiCatalogPage.copyFailed'))}
        />
      </CardContent>
    </Card>
  );
}

CapabilitySection.propTypes = {
  capability: PropTypes.object.isRequired,
  models: PropTypes.array.isRequired,
  baseUrl: PropTypes.string,
  apiKey: PropTypes.string,
  loading: PropTypes.bool,
  error: PropTypes.bool
};
