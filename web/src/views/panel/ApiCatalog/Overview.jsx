import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Link } from 'react-router';
import { ArrowRight, ExternalLink } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import CodeBlock from '@/components/ui/code-block';
import { toast } from '@/components/ui/sonner';
import { resolveServerAddress } from 'utils/serverAddress';
import { useOrg } from 'contexts/OrgContext';
import CapabilityChips from './CapabilityChips';
import { API_KEY_PLACEHOLDER, ExampleTabs, LANGUAGES, MODEL_PLACEHOLDER } from './CapabilitySection';
import ModelsError from './ModelsError';
import { ModelFacts } from './RecommendedModelCard';
import useCatalogModels from './useCatalogModels';
import { buildCapabilitySections, capabilityEmptyKey, pickSectionRecommended } from './availability';
import { CATALOG } from './catalog';

// ==============================|| PANEL — API OVERVIEW ||============================== //
// /panel/api 的落地页:一段快速开始示例(对话模态的主能力 + 推荐模型)+ 四张模态卡
// (能力 chip 与各自模态页页头同源)+ 相关链接。可用性一律来自 /api/available_model,
// 页面本身不维护任何静态模型表。

// 快速开始:固定取对话模态的首个能力(文本生成),与对话页的第一节同一段示例。
function Quickstart({ baseUrl, model }) {
  const { t } = useTranslation();
  const [language, setLanguage] = useState(LANGUAGES[0].id);

  const capability = CATALOG.chat.capabilities[0];
  const code = useMemo(() => {
    const build = capability.examples?.[language];
    return typeof build === 'function' ? build({ baseUrl, apiKey: API_KEY_PLACEHOLDER, model: model || MODEL_PLACEHOLDER }) : '';
  }, [capability.examples, language, baseUrl, model]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('apiCatalogPage.overview.quickstart')}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <ExampleTabs items={LANGUAGES} activeId={language} onSelect={setLanguage} />
        <CodeBlock
          code={code}
          language={LANGUAGES.find((l) => l.id === language)?.hljs || 'bash'}
          wrap
          maxHeightClass="max-h-96"
          copyLabel={t('common.copy')}
          copiedLabel={t('apiCatalogPage.copied')}
          onCopy={() => toast.success(t('apiCatalogPage.copied'))}
          onCopyError={() => toast.error(t('apiCatalogPage.copyFailed'))}
        />
        <p className="text-xs text-muted-foreground">{t('apiCatalogPage.keyHint')}</p>
      </CardContent>
    </Card>
  );
}

Quickstart.propTypes = {
  baseUrl: PropTypes.string,
  model: PropTypes.string
};

// 单张模态卡:模态名 + 一句话 + 能力 chip + 推荐模型(上下文 / 价格)+ 跳转。
// 「查看接口」去文档页,「在控制台打开」去控制台页。
function ModalityCard({ entry, models, siteInfo, showPlayground, loading, error }) {
  const { t } = useTranslation();

  const sections = useMemo(() => buildCapabilitySections(entry.capabilities, models, siteInfo), [entry.capabilities, models, siteInfo]);
  const recommended = useMemo(() => pickSectionRecommended(sections), [sections]);
  const to = `/panel/api/docs/${entry.key}`;

  return (
    <Card className="flex flex-col">
      <CardHeader className="gap-1">
        <CardTitle className="text-base">{t(entry.apiNameKey)}</CardTitle>
        <p className="text-sm text-muted-foreground">{t(entry.introKey)}</p>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col gap-3">
        <CapabilityChips sections={sections} loading={loading} error={error} />

        {recommended ? (
          <div className="space-y-2 rounded-lg border border-border p-3">
            <p className="text-xs text-muted-foreground">{t('apiCatalogPage.recommended')}</p>
            <ModelFacts model={recommended} />
          </div>
        ) : (
          // 取数未完成 / 失败前说「暂无可用模型」会把加载中或故障说成不支持,原因文案与 chip 同源。
          <p className="text-sm text-muted-foreground">{t(capabilityEmptyKey(null, { loading, error }))}</p>
        )}

        <div className="mt-auto flex flex-wrap items-center gap-2 pt-1">
          <Button asChild variant="outline" size="sm">
            <Link to={to}>
              {t('apiCatalogPage.overview.viewApi')}
              <ArrowRight className="size-4" />
            </Link>
          </Button>
          {showPlayground && (
            <Button asChild variant="ghost" size="sm">
              <Link to={`/panel/api/${entry.key}`}>{t('apiCatalogPage.openPlayground')}</Link>
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

ModalityCard.propTypes = {
  entry: PropTypes.object.isRequired,
  models: PropTypes.array,
  siteInfo: PropTypes.object,
  showPlayground: PropTypes.bool,
  loading: PropTypes.bool,
  error: PropTypes.bool
};

// 相关链接:站内两条(模型广场 / API Key)+ 站点文档(未配置文档站时不渲染)。
function RelatedLinks({ docsHref }) {
  const { t } = useTranslation();

  const items = [
    { id: 'models', to: '/panel/model_price' },
    { id: 'keys', to: '/panel/token' },
    ...(docsHref ? [{ id: 'docs', href: docsHref }] : [])
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('apiCatalogPage.overview.links.title')}</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3 sm:grid-cols-3">
        {items.map((item) => {
          const body = (
            <>
              <span className="flex items-center gap-1.5 text-sm font-medium text-foreground">
                {t(`apiCatalogPage.overview.links.${item.id}`)}
                {item.href ? <ExternalLink className="size-4" /> : <ArrowRight className="size-4" />}
              </span>
              <span className="text-xs text-muted-foreground">{t(`apiCatalogPage.overview.links.${item.id}Desc`)}</span>
            </>
          );
          const className = 'flex flex-col gap-1 rounded-lg border border-border p-3 transition-colors hover:bg-muted/50';
          return item.href ? (
            <a key={item.id} href={item.href} target="_blank" rel="noopener noreferrer" className={className}>
              {body}
            </a>
          ) : (
            <Link key={item.id} to={item.to} className={className}>
              {body}
            </Link>
          );
        })}
      </CardContent>
    </Card>
  );
}

RelatedLinks.propTypes = {
  docsHref: PropTypes.string
};

export default function ApiOverview() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const { currentOrgId } = useOrg();
  const { models, loading, error, reload } = useCatalogModels();

  const baseUrl = resolveServerAddress(siteInfo?.server_address);
  const docsHref = siteInfo?.docs_link ? `${siteInfo.docs_link.replace(/\/+$/, '')}${CATALOG.chat.docsPath}` : null;
  // 与对话页同一条件:组织上下文与关闭内置对话时都没有试用区,卡片上也不给入口。
  const showPlayground = !currentOrgId && siteInfo?.builtin_chat_enabled !== false;

  // 快速开始的模型:对话模态推荐模型,取不到时用占位模型名。
  const quickstartModel = useMemo(
    () => pickSectionRecommended(buildCapabilitySections(CATALOG.chat.capabilities, models, siteInfo))?.id || '',
    [models, siteInfo]
  );

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t('apiCatalogPage.overview.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('apiCatalogPage.overview.intro')}</p>
      </div>

      {error && <ModelsError onRetry={reload} />}

      <Quickstart baseUrl={baseUrl} model={quickstartModel} />

      <div className="grid gap-4 md:grid-cols-2">
        {Object.values(CATALOG).map((entry) => (
          <ModalityCard
            key={entry.key}
            entry={entry}
            models={models}
            siteInfo={siteInfo}
            showPlayground={entry.key === 'chat' && showPlayground}
            loading={loading}
            error={error}
          />
        ))}
      </div>

      <RelatedLinks docsHref={docsHref} />
    </div>
  );
}
