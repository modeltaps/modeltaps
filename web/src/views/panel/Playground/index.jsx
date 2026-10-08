import { Fragment, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Navigate, useLocation, useParams, useSearchParams } from 'react-router';

import { findActiveItem } from '@/components/chrome/nav-config';
import PageActions from '@/components/chrome/PageActions';
import { resolveServerAddress } from 'utils/serverAddress';
import useCatalogModels from 'views/panel/ApiCatalog/useCatalogModels';
import { DEFAULT_MODALITY, resolveModality } from './modalities';
import { ConsoleContext } from './shared/ConsoleContext';

// ==============================|| PANEL — API CONSOLE ||============================== //
// `/panel/api/:modality` 的路由页：按模态注册表选装配层(未登记回落到对话)，把目录取数与
// 选中模型通过 ConsoleContext 交给各模态。页壳是最简结构(标题 + 右上动作 + 居中主区)，
// 正式页壳与输入框 chip 由组件任务提供。接口文档在 `/panel/api/docs/:modality`。
// 分层规则见 modalities.js 顶部注释。

export default function ApiConsole() {
  const { t } = useTranslation();
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

  if (!entry) return <Navigate to={`/panel/api/${DEFAULT_MODALITY}`} replace />;

  const Workspace = entry.workspace;
  const Actions = entry.actions;
  // 模态自己的上下文（会话 state）罩住整页，装配层拆分插槽时仍共享同一份 state。
  const ModalityProvider = entry.provider || Fragment;
  // 标题优先由 MainLayout 的 PageHeader 提供(导航登记该路径后);未登记时页面自带标题。
  const hasNavHeader = Boolean(findActiveItem(pathname));

  return (
    <ConsoleContext.Provider value={consoleValue}>
      <ModalityProvider>
        {/* 定高而不是 min-height：控制台页自己滚(消息区滚、composer 贴底)，
            高度不定的话整页会跟着长，输入框被顶出视口。 */}
        <div
          className={`flex min-h-[520px] flex-col gap-4 ${hasNavHeader ? 'h-[calc(100vh-11rem)] sm:h-[calc(100vh-8rem)]' : 'h-[calc(100vh-11rem)]'}`}
        >
          {/* 有导航标题时动作传送到 PageHeader 标题行右侧，页内不再单独占一行。 */}
          {hasNavHeader ? (
            Actions && (
              <PageActions>
                <Actions />
              </PageActions>
            )
          ) : (
            <div className="flex items-start justify-between gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">{t(entry.labelKey)}</h1>
              <div className="ml-auto flex items-center gap-2">{Actions ? <Actions /> : null}</div>
            </div>
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
