import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { MoreHorizontal, PanelRight, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';

// Enable switch: follows the row (the list reloads after a change) and is disabled while the
// request runs so it cannot be double-clicked.
export function StatusCell({ item, onToggle }) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const toggle = async () => {
    setPending(true);
    try {
      await onToggle(item);
    } finally {
      setPending(false);
    }
  };
  return <Switch size="sm" checked={!!item.enable} disabled={pending} aria-label={t('userGroup.enable')} onCheckedChange={toggle} />;
}

StatusCell.propTypes = { item: PropTypes.object.isRequired, onToggle: PropTypes.func.isRequired };

// Row menu: open the group, and delete (destructive, last). With `onToggle`, also enable / disable
// (the phone list, which has no switch).
export function ActionsCell({ item, onOpen, onDelete, onToggle }) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t('userPage.action')}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40">
        <DropdownMenuItem onClick={() => onOpen(item.id)}>
          <PanelRight className="size-4" />
          {t('objects.open')}
        </DropdownMenuItem>
        {onToggle && (
          <DropdownMenuItem onClick={() => onToggle(item)}>
            {item.enable ? t('userGroup.disableAction') : t('userGroup.enableAction')}
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive" onClick={() => onDelete(item)}>
          <Trash2 className="size-4" />
          {t('common.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

ActionsCell.propTypes = {
  item: PropTypes.object.isRequired,
  onOpen: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  onToggle: PropTypes.func
};
