import { useTranslation } from 'react-i18next';
import { useRouteError } from 'react-router';
import { RefreshCw, TriangleAlert } from 'lucide-react';

import { Button } from '@/components/ui/button';
import ErrorState from '@/components/ErrorState';
import config from 'config';

// A lazily loaded page whose file is gone: the site was redeployed while this tab was open.
const STALE_CHUNK = /dynamically imported module|Importing a module script failed|Unable to preload CSS|ChunkLoadError/i;

export function isStaleChunkError(error) {
  return STALE_CHUNK.test(`${error?.name || ''} ${error?.message || ''}`);
}

// Shown when a page throws while rendering, instead of the router's built-in error screen. The app
// tree above it is gone, so every way out is a full page load.
export default function RouteError() {
  const { t } = useTranslation();
  const error = useRouteError();
  if (import.meta.env.DEV) console.error(error);
  const stale = isStaleChunkError(error);
  const home = `${config.basename.replace(/\/+$/, '')}/`;
  const detail = import.meta.env.DEV && (error?.stack || error?.message || String(error));

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <ErrorState
        className="min-h-screen"
        icon={stale ? RefreshCw : TriangleAlert}
        title={stale ? t('errorPage.updateTitle') : t('errorPage.crashTitle')}
        description={stale ? t('errorPage.updateBody') : t('errorPage.crashBody')}
        actions={
          <>
            {!stale && (
              <Button variant="outline" asChild>
                <a href={home}>{t('errorPage.goHome')}</a>
              </Button>
            )}
            <Button onClick={() => window.location.reload()}>{t('errorPage.reload')}</Button>
          </>
        }
      >
        {detail && (
          <details className="mt-10 w-full max-w-2xl">
            <summary className="cursor-pointer text-xs text-muted-foreground">{t('errorPage.details')}</summary>
            <pre className="mt-2 max-h-72 text-left overflow-auto rounded-md bg-muted p-3 font-mono text-xs text-muted-foreground">
              {detail}
            </pre>
          </details>
        )}
      </ErrorState>
    </div>
  );
}
