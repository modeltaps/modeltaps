import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Pencil, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { RowActions } from '@/components/ui/row-actions';

export function ActionsCell({ item, manageMenu, onEdit }) {
  const { t } = useTranslation();
  const [openDelete, setOpenDelete] = useState(false);

  return (
    <>
      <RowActions
        actions={[
          { label: t('common.edit'), icon: Pencil, onClick: () => onEdit(item.id) },
          { label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => setOpenDelete(true) }
        ]}
      />

      <Dialog open={openDelete} onOpenChange={setOpenDelete}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <p className="text-sm text-muted-foreground">{t('common.deleteConfirm', { title: item.command })}</p>
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpenDelete(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                setOpenDelete(false);
                manageMenu(item.id, 'delete');
              }}
            >
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
