import { useTranslation } from 'react-i18next';
import { Navigate, useLocation, useParams } from 'react-router';

import { findActiveItem } from '@/components/chrome/nav-config';
import CatalogLayout from './CatalogLayout';
import { getCatalogEntry, DEFAULT_MODALITY } from './catalog';

// ==============================|| PANEL — API CATALOG (DOCS) ||============================== //
// `/panel/api/docs/:modality` 的接口文档页:按路由参数渲染对应模态的能力目录;
// 未登记的模态回落到「对话」。控制台在 `/panel/api/:modality`。

export default function ApiCatalog() {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const { modality } = useParams();
  const entry = getCatalogEntry(modality);

  if (!entry) return <Navigate to={`/panel/api/docs/${DEFAULT_MODALITY}`} replace />;

  // 标题优先由 MainLayout 的 PageHeader 提供(导航登记该路径后);未登记时页面自带标题,
  // 避免目录页在侧栏接入前没有标题。
  const hasNavHeader = Boolean(findActiveItem(pathname));

  return (
    <div className="space-y-4">
      {!hasNavHeader && <h1 className="text-2xl font-semibold tracking-tight">{t(entry.titleKey)}</h1>}
      <CatalogLayout entry={entry} />
    </div>
  );
}
