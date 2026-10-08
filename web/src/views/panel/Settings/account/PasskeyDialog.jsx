import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2, Plus } from 'lucide-react';

import {
  deleteWebAuthnCredential,
  getWebAuthnCredentials,
  onWebAuthnRegister,
  showError,
  showSuccess,
  timestamp2string
} from 'utils/common';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';

// 通行密钥管理:列出已添加的、添加新的(可起名)、删除。界面上只叫「通行密钥」,不出现 WebAuthn / 凭据。
// emergency:外部账号体系下 root 的应急通行密钥,说明文案不同。

export default function PasskeyDialog({ open, onOpenChange, emergency = false, onChanged }) {
  const { t } = useTranslation();
  const [credentials, setCredentials] = useState([]);
  const [loading, setLoading] = useState(false);
  const [adding, setAdding] = useState(false);
  const [alias, setAlias] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [deleteId, setDeleteId] = useState(null);

  const load = async () => {
    setLoading(true);
    try {
      setCredentials(await getWebAuthnCredentials());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (open) load();
  }, [open]);

  const changed = () => {
    load();
    onChanged?.();
  };

  const confirmAdd = async () => {
    setSubmitting(true);
    try {
      await onWebAuthnRegister(showError, showSuccess, changed, alias.trim());
      setAdding(false);
      setAlias('');
    } finally {
      setSubmitting(false);
    }
  };

  const confirmDelete = async () => {
    if (deleteId) await deleteWebAuthnCredential(deleteId, showError, showSuccess, changed);
    setDeleteId(null);
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{emergency ? t('settingsPage.security.emergencyPasskey') : t('settingsPage.security.passkey')}</DialogTitle>
            <DialogDescription>
              {emergency ? t('settingsPage.security.emergencyPasskeyHint') : t('settingsPage.security.passkeyHint')}
            </DialogDescription>
          </DialogHeader>
          <DialogBody className="space-y-4">
            {credentials.length > 0 ? (
              <div className="space-y-2">
                {credentials.map((c) => (
                  <div key={c.id} className="flex items-center justify-between gap-4 rounded-md border border-border p-3">
                    <div className="min-w-0 text-sm">
                      <p className="truncate font-medium">
                        {c.alias && c.alias.trim() !== '' ? c.alias : t('settingsPage.security.passkeyUnnamed')}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {t('settingsPage.security.addedAt', { time: timestamp2string(c.created_time) })}
                      </p>
                    </div>
                    <Button variant="outline" size="sm" onClick={() => setDeleteId(c.id)} disabled={loading}>
                      {t('common.delete')}
                    </Button>
                  </div>
                ))}
              </div>
            ) : (
              !loading && <p className="text-sm text-muted-foreground">{t('settingsPage.security.passkeyNone')}</p>
            )}

            {adding ? (
              <div className="space-y-2 rounded-md border border-dashed border-border p-3">
                <Label htmlFor="passkey-alias">{t('settingsPage.security.passkeyName')}</Label>
                <Input
                  id="passkey-alias"
                  autoFocus
                  value={alias}
                  placeholder={t('settingsPage.security.passkeyNamePlaceholder')}
                  onChange={(e) => setAlias(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      confirmAdd();
                    }
                  }}
                  disabled={submitting}
                />
                <div className="flex justify-end gap-2">
                  <Button variant="outline" size="sm" onClick={() => setAdding(false)} disabled={submitting}>
                    {t('common.cancel')}
                  </Button>
                  <Button size="sm" onClick={confirmAdd} disabled={submitting}>
                    {submitting && <Loader2 className="size-4 animate-spin" />}
                    {t('common.confirm')}
                  </Button>
                </div>
              </div>
            ) : (
              <Button variant="outline" onClick={() => setAdding(true)} disabled={loading}>
                <Plus className="size-4" />
                {t('settingsPage.security.addPasskey')}
              </Button>
            )}
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t('common.close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!deleteId} onOpenChange={(o) => !o && setDeleteId(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('settingsPage.security.removePasskey')}</DialogTitle>
            <DialogDescription>{t('settingsPage.security.removePasskeyHint')}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteId(null)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmDelete}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

PasskeyDialog.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func,
  emergency: PropTypes.bool,
  onChanged: PropTypes.func
};
