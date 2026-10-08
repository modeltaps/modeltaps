import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { RefreshCw } from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';

// ==============================|| API CATALOG — MODELS ERROR ||============================== //
// /api/available_model 取数失败时的提示。目录页的可用性全部建立在这份数据上，取不到就必须
// 说「加载失败」并给重试入口 —— 渲染成「暂无可用模型」等于把网络故障说成站点不支持。

export default function ModelsError({ onRetry }) {
  const { t } = useTranslation();

  return (
    <Alert variant="error">
      <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
        <span>{t('apiCatalogPage.loadFailed')}</span>
        {onRetry && (
          <Button variant="outline" size="sm" onClick={onRetry}>
            <RefreshCw className="size-4" />
            {t('apiCatalogPage.retry')}
          </Button>
        )}
      </AlertDescription>
    </Alert>
  );
}

ModelsError.propTypes = {
  onRetry: PropTypes.func
};
