import { useTranslation } from 'react-i18next';
import { useRouteError } from 'react-router';

import { Button } from '@/components/ui/button';

// Shown when a page throws while rendering, instead of the router's built-in error screen.
export default function RouteError() {
  const { t } = useTranslation();
  const error = useRouteError();
  if (import.meta.env.DEV) console.error(error);
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 p-6 text-center">
      <p className="text-base font-medium text-foreground">{t('common.error')}</p>
      <p className="max-w-md text-sm text-muted-foreground">{t('common.reloadHint')}</p>
      <Button onClick={() => window.location.reload()}>{t('common.reload')}</Button>
    </div>
  );
}
