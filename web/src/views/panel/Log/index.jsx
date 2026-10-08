import { useEffect } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useOrg } from 'contexts/OrgContext';
import LogPanel from './LogPanel';
import Midjourney from '../Midjourney';
import Task from '../Task';

// ==============================|| PANEL — LOG (Tab 容器) ||============================== //
// 日志页承载三类记录:请求日志(./LogPanel)、Midjourney 记录、Suno/Kling 异步任务记录。
// 当前 Tab 由 ?tab=log|midjourney|task 单向驱动(无本地状态,刷新/深链自动恢复)。
// 组织上下文只有请求日志(MJ/异步任务接口无组织维度),非法或不适用的 tab 回落到 log。
// 仅挂载当前 Tab 的组件:PageActions 走 portal,同时挂载会让筛选/时间范围按钮互相残留。

const LOG_TABS = ['log', 'midjourney', 'task'];
const DEFAULT_TAB = 'log';

export default function Log() {
  const { t } = useTranslation();
  const { currentOrgId } = useOrg();
  const isOrgContext = Boolean(currentOrgId);
  const [searchParams, setSearchParams] = useSearchParams();

  const requestedTab = searchParams.get('tab');
  const tab = !isOrgContext && LOG_TABS.includes(requestedTab) ? requestedTab : DEFAULT_TAB;

  // URL 带非法/组织上下文不支持的 tab 时写回实际 Tab(replace,不污染历史)。
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
        <TabsList>
          <TabsTrigger value="log">{t('logPage.tabs.log')}</TabsTrigger>
          {!isOrgContext && <TabsTrigger value="midjourney">{t('logPage.tabs.midjourney')}</TabsTrigger>}
          {!isOrgContext && <TabsTrigger value="task">{t('logPage.tabs.task')}</TabsTrigger>}
        </TabsList>
      </Tabs>

      {tab === 'log' && <LogPanel />}
      {tab === 'midjourney' && <Midjourney />}
      {tab === 'task' && <Task />}
    </div>
  );
}
