import { useEffect } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import ModelOwnedby from '../ModelOwnedby';
import ModelInfo from '../ModelInfo';
import { resolveCatalogTab } from './modelCatalog';

// ==============================|| PANEL — MODELS (Tab 容器) ||============================== //
// 模型页承载两个 Tab:模型表(目录 + 价格 + 渠道合一,../ModelInfo)与厂商(../ModelOwnedby)。
// 当前 Tab 由 ?tab=models|vendors 单向驱动(无本地状态,刷新/深链自动恢复);旧值
// price / info / ownedby 映射到新 Tab 并写回 URL。仅挂载当前 Tab 的组件:PageActions 走 portal,
// 同时挂载会互相残留。Tab 条渲染在页头下方、各 Tab 内容(统计 chip / 工具栏 / 表格)之上。

const TAB_TRIGGER_CLASS =
  '-mb-px rounded-none border-b-2 px-1 pb-2 pt-0 shadow-none bg-transparent data-[state=active]:bg-transparent data-[state=active]:shadow-none';

export default function ModelCatalog() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();

  const requestedTab = searchParams.get('tab');
  const tab = resolveCatalogTab(requestedTab);

  // URL 带旧值或非法 tab 时写回实际 Tab(replace,不污染历史)。
  useEffect(() => {
    if (!requestedTab || requestedTab === tab) return;
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        params.set('tab', tab);
        return params;
      },
      { replace: true }
    );
  }, [requestedTab, tab, setSearchParams]);

  const handleTabChange = (next) => {
    setSearchParams((prev) => {
      const params = new URLSearchParams(prev);
      params.set('tab', next);
      return params;
    });
  };

  return (
    <div className="space-y-4">
      <Tabs value={tab} onValueChange={handleTabChange}>
        <TabsList className="flex h-auto w-full justify-start gap-5 rounded-none border-b border-border bg-transparent p-0">
          {['models', 'vendors'].map((value) => (
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
    </div>
  );
}
