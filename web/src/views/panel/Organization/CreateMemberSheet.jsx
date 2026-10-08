import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import { API } from 'utils/api';
import { showError, showSuccess, trims } from 'utils/common';

// ==============================|| ORGANIZATION — CREATE MEMBER ACCOUNT (Admin+; ORG-1) ||============================== //
// 组织内代建成员账号:创建新用户并加入本组织。后端 POST /api/org/:id/members/account
// （事务建号入组,角色不得高于操作者）。预算可在创建后用 MemberLimitsSheet 设置；
// API Key 可在账号建好后由管理员为其创建（复用既有组织 API Key 流程）。

export default function CreateMemberSheet({ open, onOpenChange, orgId, onSaved }) {
  const { t } = useTranslation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [email, setEmail] = useState('');
  const [role, setRole] = useState('member');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setUsername('');
    setPassword('');
    setDisplayName('');
    setEmail('');
    setRole('member');
    setError('');
  }, [open]);

  const submit = async () => {
    const u = trims(username);
    if (!u) {
      setError(t('orgPage.members.usernameRequired'));
      return;
    }
    if (!password) {
      setError(t('orgPage.members.passwordRequired'));
      return;
    }
    setSaving(true);
    try {
      const res = await API.post(`/api/org/${orgId}/members/account`, {
        username: u,
        password,
        display_name: trims(displayName),
        email: trims(email),
        role
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.members.createAccountSuccess'));
        onSaved?.();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{t('orgPage.members.createAccount')}</SheetTitle>
          <SheetDescription>{t('orgPage.members.createAccountHint')}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField label={t('orgPage.members.username')} required>
            <Input
              value={username}
              onChange={(e) => {
                setUsername(e.target.value);
                setError('');
              }}
            />
          </FormField>
          <FormField label={t('orgPage.members.password')} required>
            <Input
              type="password"
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                setError('');
              }}
            />
          </FormField>
          <FormField label={t('orgPage.members.displayName')}>
            <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
          </FormField>
          <FormField label={t('orgPage.members.email')}>
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </FormField>
          <FormField label={t('orgPage.members.role')}>
            <Select value={role} onValueChange={setRole}>
              <SelectTrigger>
                <span className="truncate">{t(`org.roles.${role}`)}</span>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="member">{t('org.roles.member')}</SelectItem>
                <SelectItem value="admin">{t('org.roles.admin')}</SelectItem>
              </SelectContent>
            </Select>
          </FormField>
          {error && <p className="text-sm text-destructive">{error}</p>}
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

CreateMemberSheet.propTypes = {
  open: PropTypes.bool.isRequired,
  onOpenChange: PropTypes.func.isRequired,
  orgId: PropTypes.number.isRequired,
  onSaved: PropTypes.func
};
