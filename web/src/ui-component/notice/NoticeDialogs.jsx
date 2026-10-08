import { X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody } from '@/components/ui/dialog';
import { useNotice } from './NoticeContext';
import ContentViewer from '../ContentViewer';

export const NoticeDialogs = () => {
  const { t } = useTranslation();
  const { isOpen, closeNotice, notice } = useNotice();

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && closeNotice()}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle id="customized-dialog-title">{t('notice.announcement')}</DialogTitle>
        </DialogHeader>
        <button
          type="button"
          aria-label="close"
          onClick={closeNotice}
          className="absolute right-3 top-3 rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          <X size="1em" />
        </button>
        <DialogBody>
          <ContentViewer content={notice || ''} loading={false} containerStyle={{ backgroundColor: 'transparent' }} />
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
};
