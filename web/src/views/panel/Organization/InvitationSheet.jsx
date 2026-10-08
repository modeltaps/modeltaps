import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { FormField } from '@/components/ui/form-field';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import { API } from 'utils/api';
import { copy, showError, showSuccess, trims } from 'utils/common';
import { inviteLink } from './MembersTab';

// ==============================|| ORGANIZATION — CREATE INVITATION (Admin+) ||============================== //
// Directed (email/username, single-use) or link (max uses + expiry) invitation.

const pad = (n) => String(n).padStart(2, '0');
const toLocal = (unix) => {
  const d = new Date(unix * 1000);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
const fromLocal = (s) => Math.floor(new Date(s).getTime() / 1000);

export default function InvitationSheet({ open, onOpenChange, orgId, onSaved }) {
  const { t } = useTranslation();
  const [mode, setMode] = useState('directed');
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [role, setRole] = useState('member');
  const [maxUses, setMaxUses] = useState(1);
  const [expiredTime, setExpiredTime] = useState(0); // 0 = never
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setMode('directed');
    setEmail('');
    setUsername('');
    setRole('member');
    setMaxUses(1);
    setExpiredTime(0);
    setError('');
  }, [open]);

  const submit = async () => {
    const payload = { role, expired_time: expiredTime };
    if (mode === 'directed') {
      const e = trims(email);
      const u = trims(username);
      if (!e && !u) {
        setError(t('orgPage.invitations.emailOrUsernameRequired'));
        return;
      }
      payload.email = e;
      payload.username = u;
      payload.max_uses = 1;
    } else {
      payload.max_uses = Math.max(0, parseInt(maxUses, 10) || 0);
    }
    setSaving(true);
    try {
      const res = await API.post(`/api/org/${orgId}/invitations`, payload);
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('orgPage.invitations.createSuccess'));
        if (mode === 'link' && data?.token) copy(inviteLink(data.token), t('orgPage.invitations.copyLink'));
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
          <SheetTitle>{t('orgPage.invitations.create')}</SheetTitle>
          <SheetDescription>
            {mode === 'directed' ? t('orgPage.invitations.directedHint') : t('orgPage.invitations.linkHint')}
          </SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <Tabs value={mode} onValueChange={setMode}>
            <TabsList>
              <TabsTrigger value="directed">{t('orgPage.invitations.directed')}</TabsTrigger>
              <TabsTrigger value="link">{t('orgPage.invitations.link')}</TabsTrigger>
            </TabsList>
          </Tabs>

          {mode === 'directed' && (
            <>
              <FormField label={t('orgPage.invitations.email')} required={!trims(username)}>
                <Input
                  value={email}
                  onChange={(e) => {
                    setEmail(e.target.value);
                    setError('');
                  }}
                />
              </FormField>
              <FormField label={t('orgPage.invitations.username')} required={!trims(email)} error={error}>
                <Input
                  value={username}
                  onChange={(e) => {
                    setUsername(e.target.value);
                    setError('');
                  }}
                />
              </FormField>
            </>
          )}

          {mode === 'link' && (
            <FormField label={t('orgPage.invitations.maxUses')} hint={t('orgPage.invitations.maxUsesHint')}>
              <Input type="number" min={0} value={maxUses} onChange={(e) => setMaxUses(e.target.value)} />
            </FormField>
          )}

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

          <div className="flex items-center justify-between gap-4 py-1">
            <Label className="font-normal">{t('orgPage.invitations.neverExpires')}</Label>
            <Switch
              checked={expiredTime === 0}
              onCheckedChange={(c) => setExpiredTime(c ? 0 : Math.floor(Date.now() / 1000) + 7 * 86400)}
            />
          </div>
          {expiredTime !== 0 && (
            <FormField label={t('orgPage.invitations.expiryTime')}>
              <Input
                type="datetime-local"
                value={expiredTime > 0 ? toLocal(expiredTime) : ''}
                onChange={(e) => setExpiredTime(e.target.value ? fromLocal(e.target.value) : 0)}
              />
            </FormField>
          )}
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

InvitationSheet.propTypes = {
  open: PropTypes.bool.isRequired,
  onOpenChange: PropTypes.func.isRequired,
  orgId: PropTypes.number.isRequired,
  onSaved: PropTypes.func
};
