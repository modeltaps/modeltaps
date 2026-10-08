import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { MessageSquareText } from 'lucide-react';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useOrg } from 'contexts/OrgContext';

// ==============================|| DASHBOARD — QUICK START ||============================== //
// 入口只做站内跳转:不再申请试用 Key、也不再拼含 Key 的外链(SEC-18)。组织上下文下站内试用区
// 不可用(CatalogLayout 只覆盖个人 API Key),退化为跳 API 指南。

export default function QuickStartCard() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { currentOrgId } = useOrg();
  const inOrg = Boolean(currentOrgId);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('dashboard_index.quickStart')}</CardTitle>
        <p className="text-sm text-muted-foreground">{t(inOrg ? 'dashboard_index.quickStartOrgTip' : 'dashboard_index.quickStartTip')}</p>
      </CardHeader>
      <CardContent>
        <Button size="sm" onClick={() => navigate(inOrg ? '/panel/api' : '/panel/api/chat#playground')}>
          <MessageSquareText className="size-4" />
          {t(inOrg ? 'dashboard_index.quickStartOrgAction' : 'dashboard_index.quickStartAction')}
        </Button>
      </CardContent>
    </Card>
  );
}
