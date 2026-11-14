import { useEffect, useRef } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link, Navigate, useLocation, useParams } from 'react-router';
import { AudioLines, Clapperboard, Image, MessageSquare } from 'lucide-react';

import { cn } from '@/lib/utils';
import { findActiveItem } from '@/components/chrome/nav-config';
import PageTitleExtra from '@/components/chrome/PageTitleExtra';
import CatalogLayout from './CatalogLayout';
import { CATALOG, getCatalogEntry, DEFAULT_MODALITY } from './catalog';

// ==============================|| PANEL — API CATALOG (DOCS) ||============================== //
// `/panel/api/docs/:modality` 的接口文档页:按路由参数渲染对应模态的能力目录;
// 未登记的模态回落到「对话」。控制台在 `/panel/api/:modality`。
// The sidebar's 文档 item opens chat; the switch next to the page title moves between the
// modalities (same look as the playground's switch), each on its own route.

const SWITCH = [
  { id: 'chat', icon: MessageSquare, labelKey: 'playgroundConsole.modality.chat' },
  { id: 'image', icon: Image, labelKey: 'playgroundConsole.modality.image' },
  { id: 'speech', icon: AudioLines, labelKey: 'playgroundConsole.modality.audio' },
  { id: 'video', icon: Clapperboard, labelKey: 'playgroundConsole.modality.video' }
].filter((m) => CATALOG[m.id]);

function DocsModalitySwitch({ activeId }) {
  const { t } = useTranslation();
  // On narrow screens the strip scrolls sideways; keep the current one in view.
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
      aria-label={t('apiCatalogPage.modalities', { defaultValue: 'Docs' })}
      className="no-scrollbar relative flex max-w-full items-center gap-0.5 overflow-x-auto rounded-lg bg-control p-0.5"
    >
      {SWITCH.map(({ id, icon: Icon, labelKey }) => {
        const active = id === activeId;
        return (
          <Link
            key={id}
            to={`/panel/api/docs/${id}`}
            aria-current={active ? 'page' : undefined}
            className={cn(
              'flex h-7 shrink-0 items-center gap-1.5 rounded-md px-2.5 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              active ? 'bg-control-thumb text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
            )}
          >
            <Icon className="size-3.5" />
            {t(labelKey)}
          </Link>
        );
      })}
    </nav>
  );
}

DocsModalitySwitch.propTypes = { activeId: PropTypes.string };

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
      {hasNavHeader ? (
        <PageTitleExtra>
          <DocsModalitySwitch activeId={modality} />
        </PageTitleExtra>
      ) : (
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-2xl font-semibold tracking-tight">{t(entry.titleKey)}</h1>
          <DocsModalitySwitch activeId={modality} />
        </div>
      )}
      <CatalogLayout entry={entry} />
    </div>
  );
}
