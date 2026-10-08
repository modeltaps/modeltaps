import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Link } from 'react-router';
import { ArrowRight, ExternalLink } from 'lucide-react';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import CodeBlock from '@/components/ui/code-block';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { toast } from '@/components/ui/sonner';
import { resolveServerAddress } from 'utils/serverAddress';
import ModelPrice from 'views/public/ModelPrice';
import CapabilityChips from './CapabilityChips';
import CapabilitySection, { API_KEY_PLACEHOLDER, ExampleTabs } from './CapabilitySection';
import ModelsError from './ModelsError';
import RecommendedModelCard from './RecommendedModelCard';
import useCatalogModels from './useCatalogModels';
import { buildCapabilitySections, pickSectionRecommended } from './availability';

// ==============================|| API CATALOG — SHARED LAYOUT ||============================== //
// 能力驱动的骨架:页头(API 名 + 一句话 + 能力 chip)→ 逐能力小节(说明 + 端点 + 模型选择器
// + 多语言示例)→ 推荐模型卡 → 可用模型 → 文档链接。试用已独立成控制台页(`/panel/api/:modality`)。
// 页面内容全部来自 catalog/*.js 的数据描述,可用性只看 /api/available_model 的目录字段;
// 本组件只负责渲染与运行时取值(Base URL 取站点真实地址,密钥用占位符)。
// 尚未迁移到能力 schema 的模态(图像/语音/视频)走下方的旧版渲染分支,保证页面不报错。
// 示例的密钥占位符与示例 Tab 行与能力小节共用一份实现(见 CapabilitySection)。

function Section({ title, children, className }) {
  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

Section.propTypes = {
  title: PropTypes.node,
  children: PropTypes.node,
  className: PropTypes.string
};

// 端点清单 + 可选的站内跳转链接(如异步任务的日志页 Tab),主端点区与扩展小节共用。
function EndpointList({ endpoints, link }) {
  const { t } = useTranslation();

  return (
    <>
      <ul className="space-y-2">
        {endpoints.map((ep) => (
          <li key={`${ep.method} ${ep.path}`} className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[0.6875rem] font-semibold uppercase text-foreground">
              {ep.method}
            </span>
            <code className="font-mono text-sm">{ep.path}</code>
            <span className="text-xs text-muted-foreground">{t(ep.descKey)}</span>
          </li>
        ))}
      </ul>
      {link && (
        <Link to={link.to} className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">
          {t(link.labelKey)}
          <ArrowRight className="size-4" />
        </Link>
      )}
    </>
  );
}

EndpointList.propTypes = {
  endpoints: PropTypes.array.isRequired,
  link: PropTypes.object
};

// 旧版(端点列表 + 单语言示例)渲染分支:仅供尚未迁移到能力 schema 的模态使用。
function LegacyBody({ entry, baseUrl }) {
  const { t } = useTranslation();
  const [activeExample, setActiveExample] = useState(entry.examples?.[0]?.id);

  const examples = useMemo(
    () => (entry.examples || []).map((ex) => ({ ...ex, code: ex.build({ baseUrl, apiKey: API_KEY_PLACEHOLDER }) })),
    [entry.examples, baseUrl]
  );
  const current = examples.find((ex) => ex.id === activeExample) || examples[0];

  return (
    <>
      <Section title={t('apiCatalogPage.endpoints')}>
        <EndpointList endpoints={entry.endpoints || []} link={entry.link} />
      </Section>

      {entry.groups?.map((group) => (
        <Section key={group.id} title={t(group.titleKey)}>
          <EndpointList endpoints={group.endpoints} link={group.link} />
        </Section>
      ))}

      <Section title={t('apiCatalogPage.examples')}>
        {examples.length > 1 && (
          <ExampleTabs
            className="mb-3"
            items={examples.map((ex) => ({ id: ex.id, label: t(ex.labelKey) }))}
            activeId={current?.id}
            onSelect={setActiveExample}
          />
        )}
        {current && (
          <CodeBlock
            code={current.code}
            language="bash"
            wrap
            maxHeightClass="max-h-96"
            copyLabel={t('common.copy')}
            copiedLabel={t('apiCatalogPage.copied')}
            onCopy={() => toast.success(t('apiCatalogPage.copied'))}
            onCopyError={() => toast.error(t('apiCatalogPage.copyFailed'))}
          />
        )}
        <p className="mt-2 text-xs text-muted-foreground">{t('apiCatalogPage.keyHint')}</p>
      </Section>
    </>
  );
}

LegacyBody.propTypes = {
  entry: PropTypes.object.isRequired,
  baseUrl: PropTypes.string
};

export default function CatalogLayout({ entry }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const { models, loading, error, reload } = useCatalogModels();

  const baseUrl = resolveServerAddress(siteInfo?.server_address);
  const docsHref = siteInfo?.docs_link ? `${siteInfo.docs_link.replace(/\/+$/, '')}${entry.docsPath}` : null;

  // 每个能力的可用模型:只按后端目录字段筛选,不看渠道。概览页的模态卡走同一个函数。
  // siteInfo 决定挂了站点开关的能力(Claude / Gemini 协议入口)是否参与渲染。
  const sections = useMemo(() => buildCapabilitySections(entry.capabilities, models, siteInfo), [entry.capabilities, models, siteInfo]);
  const recommended = useMemo(() => pickSectionRecommended(sections), [sections]);

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        {entry.apiNameKey && <h2 className="text-lg font-semibold tracking-tight">{t(entry.apiNameKey)}</h2>}
        <p className="text-sm text-muted-foreground">{t(entry.introKey)}</p>
        <CapabilityChips sections={sections} loading={loading} error={error} />
      </div>

      {error && <ModelsError onRetry={reload} />}

      {sections.length > 0 ? (
        sections.map(({ capability, models: capModels }) => (
          <CapabilitySection
            key={capability.id}
            capability={capability}
            models={capModels}
            baseUrl={baseUrl}
            apiKey={API_KEY_PLACEHOLDER}
            loading={loading}
            error={error}
          />
        ))
      ) : (
        <LegacyBody entry={entry} baseUrl={baseUrl} />
      )}

      {sections.length > 0 && <p className="text-xs text-muted-foreground">{t('apiCatalogPage.keyHint')}</p>}

      <RecommendedModelCard model={recommended} />

      <Section title={t('apiCatalogPage.models')}>
        <ModelPrice embedded initialModality={entry.modality} />
      </Section>

      {entry.extras && (
        <CollapsibleSection title={t(entry.extras.titleKey)} description={entry.extras.hintKey ? t(entry.extras.hintKey) : undefined}>
          {entry.extras.groups.map((group) => (
            <div key={group.id} className="space-y-2">
              <p className="text-sm font-medium text-foreground">{t(group.titleKey)}</p>
              <EndpointList endpoints={group.endpoints} link={group.link} />
            </div>
          ))}
        </CollapsibleSection>
      )}

      {docsHref && (
        <a
          href={docsHref}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline"
        >
          {t('apiCatalogPage.docs')}
          <ExternalLink className="size-4" />
        </a>
      )}
    </div>
  );
}

CatalogLayout.propTypes = {
  entry: PropTypes.object.isRequired
};
