import { useTranslation } from 'react-i18next';

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useOrg } from 'contexts/OrgContext';
import useRouteTab from 'hooks/useRouteTab';
import LogPanel from './LogPanel';
import Midjourney from '../Midjourney';
import Task from '../Task';

// ==============================|| PANEL — LOG (Tab 容器) ||============================== //
// 日志页承载三类记录:请求日志(./LogPanel)、Midjourney 记录、Suno/Kling 异步任务记录。
// 当前 Tab 由路径单向驱动:`/panel/logs`、`/logs/midjourney`、`/logs/tasks`(无本地状态,刷新/深链
// 自动恢复;旧的 `?tab=` 改写成路径)。
// 组织上下文只有请求日志(MJ/异步任务接口无组织维度),非法或不适用的 tab 回落到 log。
// 仅挂载当前 Tab 的组件:PageActions 走 portal,同时挂载会让筛选/时间范围按钮互相残留。

const LOG_TABS = ['log', 'midjourney', 'task'];
const ORG_LOG_TABS = ['log'];
const SEGMENTS = { task: 'tasks' };

export default function Log() {
  const { t } = useTranslation();
  const { currentOrgId } = useOrg();
  const isOrgContext = Boolean(currentOrgId);
  const { tab, setTab, redirecting } = useRouteTab({ tabs: isOrgContext ? ORG_LOG_TABS : LOG_TABS, segments: SEGMENTS });
  if (redirecting) return null;

  return (
    <div className="space-y-4">
      <Tabs value={tab} onValueChange={setTab}>
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
