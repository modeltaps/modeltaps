import { useContext, useSyncExternalStore } from 'react';
import PropTypes from 'prop-types';
import { ReactReduxContext } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowUpRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

// ==============================|| LIST PAGE — FIRST-RUN EMPTY STATE ||============================== //
// Shown in place of the toolbar and table when a page has nothing yet and no filter is applied:
// a large line icon, a title, one sentence on what the feature does, and the create action. The
// small in-table ListEmptyState stays for tabs, data-only lists and "no match" results.

// Docs are published per language: English at the root, the rest under a locale folder.
const DOCS_LOCALE_PREFIX = { zh_CN: '/zh', zh_HK: '/hk', ja_JP: '/ja' };

export function docsUrl(base, lang, path) {
  if (!base || !path) return '';
  return `${base.replace(/\/+$/, '')}${DOCS_LOCALE_PREFIX[lang] || ''}${path}`;
}

const noop = () => () => {};

// The site's docs link, or '' when the component renders outside a redux Provider (page tests).
function useDocsLink() {
  const store = useContext(ReactReduxContext)?.store;
  return useSyncExternalStore(store ? store.subscribe : noop, () => store?.getState().siteInfo?.docs_link || '');
}

export default function PageEmptyState({ icon: Icon, title, description, docsPath, action, className }) {
  const { t, i18n } = useTranslation();
  const docsLink = useDocsLink();
  const href = docsUrl(docsLink, i18n.language, docsPath);

  return (
    <div className={cn('flex min-h-[70vh] w-full flex-col items-center justify-center gap-4 px-6 py-16 text-center', className)}>
      {Icon && <Icon className="size-[5.5rem] text-muted-foreground" strokeWidth={1} aria-hidden="true" />}
      <div className="flex flex-col gap-1">
        <h2 className="text-xl font-medium text-muted-foreground">{title}</h2>
        {description && <p className="mx-auto mt-2 w-full max-w-[600px] text-balance text-sm text-muted-foreground">{description}</p>}
        {(href || action) && (
          <div className="mx-auto mt-6 flex flex-row flex-wrap items-center justify-center gap-2">
            {href && (
              <Button variant="outline" asChild>
                <a href={href} target="_blank" rel="noopener noreferrer">
                  {t('listPage.readDocs')} <ArrowUpRight className="size-3 text-muted-foreground" />
                </a>
              </Button>
            )}
            {action}
          </div>
        )}
      </div>
    </div>
  );
}

PageEmptyState.propTypes = {
  icon: PropTypes.elementType,
  title: PropTypes.node.isRequired,
  description: PropTypes.node,
  // Path of the matching guide page, e.g. "/guide/tokens"; the button shows only when the site
  // has a docs link configured.
  docsPath: PropTypes.string,
  action: PropTypes.node,
  className: PropTypes.string
};
