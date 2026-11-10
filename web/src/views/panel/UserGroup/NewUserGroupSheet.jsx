import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Users } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { Field, NewObjectSheet, ObjectTile } from '@/components/objects';
import { API } from 'utils/api';
import { showError } from 'utils/common';

// Defaults of a new group (as the earlier form filled in); only the identifier and the name
// are asked for.
const DEFAULTS = { ratio: 1, public: false, api_rate: 300, promotion: false, min: 0, max: 0, enable: true };

export default function NewUserGroupSheet({ onClose, onCreated }) {
  const { t } = useTranslation();
  const [form, setForm] = useState({ symbol: '', name: '' });
  const [busy, setBusy] = useState(false);
  const symbol = form.symbol.trim();
  const name = form.name.trim();
  const missing = [!symbol && t('userGroup.symbol'), !name && t('userGroup.name')].filter(Boolean);

  const create = async () => {
    setBusy(true);
    try {
      const res = await API.post('/api/user_group/', { ...DEFAULTS, symbol, name });
      const { success, message } = res.data;
      if (success) onCreated(symbol);
      else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  return (
    <NewObjectSheet
      titleId="new-user-group-title"
      icon={
        <ObjectTile>
          <Users />
        </ObjectTile>
      }
      title={t('userGroup.create')}
      subtitle={t('userGroup.new.subtitle')}
      touched={Boolean(form.symbol || form.name)}
      ready={missing.length === 0}
      busy={busy}
      status={missing.length ? t('objects.new.missing', { items: missing.join(t('objects.new.listSep')) }) : t('userGroup.new.defaults')}
      onCreate={create}
      onClose={onClose}
    >
      <Field label={t('userGroup.symbol')} htmlFor="ng-symbol" hint={t('userGroup.symbolTip')}>
        <Input
          id="ng-symbol"
          autoFocus
          className="font-mono text-[13px]"
          value={form.symbol}
          onChange={(e) => setForm((f) => ({ ...f, symbol: e.target.value }))}
        />
      </Field>
      <Field label={t('userGroup.name')} htmlFor="ng-name" hint={t('userGroup.nameTip')}>
        <Input id="ng-name" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
      </Field>
    </NewObjectSheet>
  );
}

NewUserGroupSheet.propTypes = { onClose: PropTypes.func.isRequired, onCreated: PropTypes.func.isRequired };
