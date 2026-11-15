import { useTranslation } from 'react-i18next';

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import ModelOwnedby from '../ModelOwnedby';
import ModelInfo from '../ModelInfo';
import PausedSupplies from '../ModelInfo/supply/PausedSupplies';
import PendingModels from '../ModelInfo/supply/PendingModels';
import LibraryRules from '../ModelInfo/LibraryRules';
import useRouteTab from 'hooks/useRouteTab';
import { CATALOG_TABS, LEGACY_CATALOG_TABS } from './modelCatalog';

// ==============================|| PANEL — MODELS (Tab 容器) ||============================== //
// 模型页承载四个 Tab:模型表(目录 + 价格 + 渠道合一,../ModelInfo)、厂商(../ModelOwnedby)、
// 暂停的供应(../ModelInfo/supply/PausedSupplies)与新上游模型(../ModelInfo/supply/PendingModels)。
// 当前 Tab 由路径单向驱动:`/panel/admin/models`(模型表)与 `/models/vendors|paused|pending|rules`
// (无本地状态,刷新/深链自动恢复);旧的 `?tab=` 值(含 price / info / ownedby)改写成路径。仅挂载当前 Tab 的组件:PageActions 走 portal,
// 同时挂载会互相残留。Tab 条渲染在页头下方、各 Tab 内容(统计 chip / 工具栏 / 表格)之上。

const TAB_TRIGGER_CLASS =
  '-mb-px rounded-none border-b-2 px-1 pb-2 pt-0 shadow-none bg-transparent data-[state=active]:bg-transparent data-[state=active]:shadow-none';

export default function ModelCatalog() {
  const { t } = useTranslation();
  const { tab, setTab, redirecting } = useRouteTab({ tabs: CATALOG_TABS, legacy: LEGACY_CATALOG_TABS });
  if (redirecting) return null;

  return (
    <div className="space-y-4">
      <Tabs value={tab} onValueChange={setTab}>
        <TabsList className="flex h-auto w-full justify-start gap-5 rounded-none border-b border-border bg-transparent p-0">
          {CATALOG_TABS.map((value) => (
            <TabsTrigger
              key={value}
              value={value}
              className={cn(TAB_TRIGGER_CLASS, tab === value ? 'border-foreground text-foreground' : 'border-transparent')}
            >
              {t(`pricingPage.catalogTabs.${value}`)}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      {tab === 'models' && <ModelInfo />}
      {tab === 'vendors' && <ModelOwnedby />}
      {tab === 'paused' && <PausedSupplies />}
      {tab === 'pending' && <PendingModels />}
      {tab === 'rules' && <LibraryRules />}
    </div>
  );
}
