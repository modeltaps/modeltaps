import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';

import { Segmented } from '@/components/objects';
import { settingEnumLabel } from '@/components/settings/catalog';
import { settingHref } from '@/components/settings/SettingHelp';

// How the call order below is read, and what it is ordered by: the order as it stands, or cheaper
// supplies first (SupplyCostOrder). Root changes it here; other admins see it.
export default function OrderBar({ costOrder, canEdit = false, onChange }) {
  const { t } = useTranslation();
  const value = costOrder === 'cost' ? 'cost' : 'priority';
  const options = ['priority', 'cost'].map((v) => ({ value: v, label: settingEnumLabel(t, 'SupplyCostOrder', v) }));
  return (
    <div className="space-y-1.5 rounded-xl border border-border bg-card px-3 py-2.5">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <p className="min-w-0 flex-1 text-[13px] text-muted-foreground">{t('channels.order.howItWorks')}</p>
        <span className="inline-flex items-center gap-2">
          <span className="text-[13px] text-muted-foreground">{t('channels.order.orderBy')}</span>
          {canEdit ? (
            <Segmented label={t('channels.order.orderBy')} options={options} value={value} onChange={(v) => v !== value && onChange(v)} />
          ) : (
            <span className="text-[13px]">{options.find((o) => o.value === value)?.label}</span>
          )}
        </span>
      </div>
      {value === 'cost' && (
        <p className="text-xs text-warning">
          {t('channels.order.cheaperFirstNote')}{' '}
          <Link to={settingHref('SupplyCostOrder')} className="underline underline-offset-2 hover:text-foreground">
            {t('channels.order.cheaperFirstSettings')}
          </Link>
        </p>
      )}
    </div>
  );
}

OrderBar.propTypes = {
  costOrder: PropTypes.string,
  canEdit: PropTypes.bool,
  onChange: PropTypes.func
};
