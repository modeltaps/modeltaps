import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { UserPlus } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { Field, NewObjectSheet, ObjectTile } from '@/components/objects';
import { API } from 'utils/api';
import { showError } from 'utils/common';

// New user: a username and a password, which is all the create endpoint reads besides the
// display name (it defaults to the username). The user starts as a common user in the default
// group; role, group, balance and profile are changed in the user's panel.
export default function NewUserSheet({ onClose, onCreated }) {
  const { t } = useTranslation();
  const [form, setForm] = useState({ username: '', password: '' });
  const [busy, setBusy] = useState(false);
  const username = form.username.trim();
  const passwordOk = form.password.length >= 8 && form.password.length <= 64;

  const missing = [!username && t('userPage.username'), !form.password && t('userPage.password')].filter(Boolean);
  let status;
  if (missing.length) status = t('objects.new.missing', { items: missing.join(t('objects.new.listSep')) });
  else if (!passwordOk) status = <span className="text-destructive">{t('userPage.passwordLength')}</span>;
  else status = t('userPage.new.defaults');

  const create = async () => {
    setBusy(true);
    try {
      const res = await API.post('/api/user/', { username, password: form.password });
      const { success, message } = res.data;
      if (success) onCreated(username);
      else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  return (
    <NewObjectSheet
      titleId="new-user-title"
      icon={
        <ObjectTile>
          <UserPlus />
        </ObjectTile>
      }
      title={t('userPage.createUser')}
      subtitle={t('userPage.new.subtitle')}
      touched={Boolean(form.username || form.password)}
      ready={Boolean(username) && passwordOk}
      busy={busy}
      status={status}
      onCreate={create}
      onClose={onClose}
    >
      <Field label={t('userPage.username')} htmlFor="nu-username">
        <Input
          id="nu-username"
          autoFocus
          autoComplete="off"
          value={form.username}
          onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))}
        />
      </Field>
      <Field label={t('userPage.password')} htmlFor="nu-password" hint={t('userPage.new.passwordHint')}>
        <Input
          id="nu-password"
          type="password"
          autoComplete="new-password"
          value={form.password}
          onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
        />
      </Field>
    </NewObjectSheet>
  );
}

NewUserSheet.propTypes = { onClose: PropTypes.func.isRequired, onCreated: PropTypes.func.isRequired };
