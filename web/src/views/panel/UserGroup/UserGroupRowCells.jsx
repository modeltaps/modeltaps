import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { PanelRight, Trash2 } from 'lucide-react';

import { Switch } from '@/components/ui/switch';
import { RowActions } from '@/components/ui/row-actions';

// 启用开关:以行数据为准(操作成功后列表重拉),请求期间禁用防连点。
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
  return <Switch checked={!!item.enable} disabled={pending} aria-label={t('userGroup.enable')} onCheckedChange={toggle} />;
}

StatusCell.propTypes = { item: PropTypes.object.isRequired, onToggle: PropTypes.func.isRequired };

export function ActionsCell({ item, onOpen, onDelete }) {
  const { t } = useTranslation();
  return (
    <RowActions
      actions={[
        { label: t('objects.open'), icon: PanelRight, onClick: () => onOpen(item.id) },
        { label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => onDelete(item) }
      ]}
    />
  );
}

ActionsCell.propTypes = { item: PropTypes.object.isRequired, onOpen: PropTypes.func.isRequired, onDelete: PropTypes.func.isRequired };
