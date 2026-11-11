import PropTypes from 'prop-types';
import { Megaphone, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { useNotice } from './NoticeContext';
import { noticeSummary } from './noticeState';

// One row at the top of the panel while the site announcement is unread: its first line, a
// button that opens the full text and a close button. It does not interrupt the page, and it
// renders nothing when there is no announcement or the user has already read or closed it.
export function NoticeBanner({ className }) {
  const { t } = useTranslation();
  const { notice, unread, openNotice, dismissNotice } = useNotice();

  if (!notice || !unread) return null;

  const label = t('notice.announcement');
  const summary = noticeSummary(notice);

  return (
    <section
      aria-label={label}
      className={cn('flex items-center gap-3 rounded-xl border border-border bg-muted/50 py-1.5 pl-4 pr-1.5 text-sm', className)}
    >
      <Megaphone className="size-4 shrink-0 text-muted-foreground" />
      <p className="min-w-0 flex-1 truncate">
        <span className="font-medium text-foreground">{label}</span>
        {summary && <span className="ml-2.5 text-muted-foreground">{summary}</span>}
      </p>
      <Button type="button" variant="ghost" size="sm" className="shrink-0" onClick={openNotice}>
        {t('notice.view')}
      </Button>
      <button
        type="button"
        aria-label={t('common.close')}
        title={t('common.close')}
        onClick={dismissNotice}
        className="flex size-8 shrink-0 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <X className="size-4" />
      </button>
    </section>
  );
}

NoticeBanner.propTypes = {
  className: PropTypes.string
};
