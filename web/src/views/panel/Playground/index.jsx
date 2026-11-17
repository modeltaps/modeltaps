import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Link, Navigate, useLocation, useParams, useSearchParams } from 'react-router';

import { cn } from '@/lib/utils';
import { findActiveItem } from '@/components/chrome/nav-config';
import PageActions from '@/components/chrome/PageActions';
import PageTitleCenter from '@/components/chrome/PageTitleCenter';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { resolveServerAddress } from 'utils/serverAddress';
import useCatalogModels from 'views/panel/ApiCatalog/useCatalogModels';
import { DEFAULT_MODALITY, SWITCH_MODALITIES, modalityHref, modeOf, resolveModality } from './modalities';
import { ConsoleContext } from './shared/ConsoleContext';
import { workbench } from 'routes/paths';
import { useOrgSlug } from 'routes/useScope';

// ==============================|| PANEL — API CONSOLE ||============================== //
// `/panel/playground/:modality` 的路由页：按模态注册表选装配层(未登记回落到对话)，把目录取数与
// 选中模型通过 ConsoleContext 交给各模态。页壳是最简结构(标题 + 右上动作 + 居中主区)，
// 正式页壳与输入框 chip 由组件任务提供。接口文档在 `/panel/docs/:modality`。
// 分层规则见 modalities.js 顶部注释。

// The sidebar has one 演练场 item for every console; this segmented switch, centered on the page
// title row, moves between modalities (each keeps its own route). Sub-modes such as
// compare are not listed: their parent modality stays selected.
function ModalitySwitch({ activeId }) {
  const { t } = useTranslation();
  const orgSlug = useOrgSlug();
  // On narrow screens the switch scrolls sideways: keep the current modality in view (the strip
  // scrolls, never the page).
  const stripRef = useRef(null);
  useEffect(() => {
    const strip = stripRef.current;
    const link = strip?.querySelector('[aria-current="page"]');
    if (!strip || !link || strip.scrollWidth <= strip.clientWidth) return;
    strip.scrollTo({ left: Math.max(0, link.offsetLeft - (strip.clientWidth - link.offsetWidth) / 2) });
  }, [activeId]);
  return (
    <nav
      ref={stripRef}
      aria-label={t('nav.playground.modality')}
      className="no-scrollbar relative flex max-w-full items-center gap-0.5 overflow-x-auto rounded-lg bg-control p-0.5"
    >
      {SWITCH_MODALITIES.map((entry) => {
        const active = entry.id === activeId;
        const Icon = entry.icon;
        return (
          <Link
            key={entry.id}
            to={modalityHref(entry, orgSlug)}
            aria-current={active ? 'page' : undefined}
            className={cn(
              'flex h-7 shrink-0 items-center gap-1.5 rounded-md px-2.5 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              active ? 'bg-control-thumb text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
            )}
          >
            <Icon className="size-3.5" />
            {t(entry.labelKey)}
          </Link>
        );
      })}
    </nav>
  );
}

ModalitySwitch.propTypes = { activeId: PropTypes.string };

// Toggle between a modality and its multi-model sub-mode (chat ↔ compare). On the way in the
// selected model rides along as ?model= and becomes the first column.
function ModeToggle({ entry, modelId }) {
  const { t } = useTranslation();
  const orgSlug = useOrgSlug();
  const mode = modeOf(entry);
  if (!mode) return null;
  const on = entry.id === mode.id;
  const target = on ? SWITCH_MODALITIES.find((m) => m.id === mode.parent) : mode;
  const href = modalityHref(target, orgSlug) + (!on && modelId ? `?model=${encodeURIComponent(modelId)}` : '');
  const Icon = mode.icon;
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          {/* 同模态栏一样的轨道 + 滑块：开着时滑块亮起，关着时只剩轨道。 */}
          <Link
            to={href}
            aria-pressed={on}
            className="flex rounded-lg bg-control p-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <span
              className={cn(
                'flex h-7 items-center gap-1.5 rounded-md px-2.5 text-[13px] font-medium transition-colors',
                on ? 'bg-control-thumb text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <Icon className="size-3.5" />
              {t(mode.labelKey)}
            </span>
          </Link>
        </TooltipTrigger>
        <TooltipContent>{t(`nav_subtitles.api_${mode.id}`)}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

ModeToggle.propTypes = { entry: PropTypes.object.isRequired, modelId: PropTypes.string };

export default function ApiConsole() {
  const { t } = useTranslation();
  const orgSlug = useOrgSlug();
  const { pathname } = useLocation();
  const { modality } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const siteInfo = useSelector((state) => state.siteInfo);
  const { models, loading, error, reload } = useCatalogModels();

  const entry = resolveModality(modality);
  // ?model= 只作用于首次落地：之后由用户在模型选择器里改。
  const [selectedId, setSelectedId] = useState(() => searchParams.get('model') || '');

  useEffect(() => {
    if (!selectedId && models.length > 0) setSelectedId(models[0].id);
  }, [models, selectedId]);

  // 选中模型回写 ?model=，刷新 / 复制链接落到同一模型；只动这一项（语音页的 ?tab= 等原样保留），
  // replace 不新增历史记录。多列模态（对比）没有单一模型：既不写也不清 ?model=。
  const multiModel = Boolean(entry?.multiModel);
  useEffect(() => {
    if (multiModel || !selectedId || searchParams.get('model') === selectedId) return;
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        params.set('model', selectedId);
        return params;
      },
      { replace: true }
    );
  }, [multiModel, selectedId, searchParams, setSearchParams]);

  const selectedModel = useMemo(() => models.find((m) => m.id === selectedId) || null, [models, selectedId]);
  const setSelectedModel = useCallback((model) => setSelectedId(typeof model === 'string' ? model : model?.id || ''), []);

  const consoleValue = useMemo(
    () => ({
      models,
      baseUrl: resolveServerAddress(siteInfo?.server_address),
      loading,
      error,
      reload,
      selectedModel,
      setSelectedModel
    }),
    [models, siteInfo?.server_address, loading, error, reload, selectedModel, setSelectedModel]
  );

  if (!entry) return <Navigate to={workbench.playground(DEFAULT_MODALITY, orgSlug)} replace />;

  const Workspace = entry.workspace;
  const Actions = entry.actions;
  // 模态自己的上下文（会话 state）罩住整页，装配层拆分插槽时仍共享同一份 state。
  const ModalityProvider = entry.provider || Fragment;
  // 标题优先由 MainLayout 的 PageHeader 提供(导航登记该路径后);未登记时页面自带标题。
  const hasNavHeader = Boolean(findActiveItem(pathname));

  const modalitySwitch = <ModalitySwitch activeId={entry.parent || entry.id} />;
  const actions = (
    <>
      <ModeToggle entry={entry} modelId={selectedId} />
      {Actions && (
        <div className="flex items-center gap-1">
          <Actions />
        </div>
      )}
    </>
  );

  return (
    <ConsoleContext.Provider value={consoleValue}>
      <ModalityProvider>
        {/* 标题行：左标题、中间模态栏、右边模式开关与本模态的动作（PageHeader 插槽）。 */}
        {hasNavHeader && (
          <>
            <PageTitleCenter>{modalitySwitch}</PageTitleCenter>
            <PageActions>{actions}</PageActions>
          </>
        )}
        {/* 定高而不是 min-height：控制台页自己滚(消息区滚、composer 贴底)，
            高度不定的话整页会跟着长，输入框被顶出视口。 */}
        {/* 5.5rem = main 上内边距 + PageHeader 标题行与下边距 + main 下内边距；
            md 以下模态栏在标题下单独一行，再减 2.75rem（行高 + 行间距）。 */}
        <div
          className={cn(
            'flex min-h-[520px] flex-col gap-4',
            hasNavHeader ? 'h-[calc(100vh-8.25rem)] md:h-[calc(100vh-5.5rem)]' : 'h-[calc(100vh-5.5rem)]'
          )}
        >
          {!hasNavHeader && (
            <>
              <h1 className="text-2xl font-semibold tracking-tight">{t(entry.labelKey)}</h1>
              <div className="flex shrink-0 items-center gap-2">
                <div className="min-w-0">{modalitySwitch}</div>
                <div className="ml-auto flex shrink-0 items-center gap-2">{actions}</div>
              </div>
            </>
          )}
          {/* min-h-0：flex 子项默认 min-height:auto，内容一多就把定高撑破，
              滚动落到 MainLayout 的 main 上、composer 被顶出视口。 */}
          <div className={`mx-auto flex w-full min-h-0 flex-1 flex-col ${multiModel || entry.fullWidth ? 'max-w-none' : 'max-w-3xl'}`}>
            <Workspace />
          </div>
        </div>
      </ModalityProvider>
    </ConsoleContext.Provider>
  );
}
