import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { API } from 'utils/api';

// What names a group, in the order the dialog lists it (model.UserGroupReferences).
export const REFERENCE_KINDS = ['users', 'shadow_accounts', 'tokens', 'backup_tokens', 'channels'];

export const hasReferences = (refs) => REFERENCE_KINDS.some((k) => Number(refs?.[k]) > 0);

// The successor offered first: default, else the first other group.
export const defaultSuccessor = (groups, symbol) => {
  const others = groups.filter((g) => g.symbol !== symbol);
  return (others.find((g) => g.symbol === 'default') || others[0])?.symbol || '';
};

const groupLabel = (g) => (g ? g.name || g.symbol : '');

// Deleting a group: what still names it is listed, and moves to the successor group chosen here
// (accounts and API keys take it; channels drop the group, and one left without a group takes the
// successor). A group nothing names is deleted as it is.
export default function DeleteUserGroupDialog({ group, onClose, onDeleted }) {
  const { t } = useTranslation();
  const [refs, setRefs] = useState(null);
  const [groups, setGroups] = useState([]);
  const [successor, setSuccessor] = useState('');
  const [error, setError] = useState('');
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (!group) return undefined;
    let alive = true;
    Promise.all([
      API.get(`/api/user_group/${group.id}/references`),
      API.get('/api/user_group/', { params: { page: 1, size: 100, order: '-id' } })
    ])
      .then(([r, g]) => {
        if (!alive) return;
        if (!r.data.success) throw new Error(r.data.message);
        const list = g.data?.data?.data || [];
        setRefs(r.data.data);
        setGroups(list);
        setSuccessor(defaultSuccessor(list, group.symbol));
      })
      .catch((e) => alive && setError(e.message));
    return () => {
      alive = false;
      setRefs(null);
      setError('');
    };
  }, [group]);

  // The group goes (null) as the dialog closes after a delete: nothing is listed then.
  const used = Boolean(group) && hasReferences(refs);
  const remove = async () => {
    setDeleting(true);
    try {
      const res = await API.delete(`/api/user_group/${group.id}`, { params: used ? { successor } : undefined });
      if (!res.data.success) {
        setError(res.data.message);
        return;
      }
      onDeleted(group);
    } catch (e) {
      setError(e.message);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Dialog open={Boolean(group)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('userGroup.delete.title', { name: group?.name || group?.symbol })}</DialogTitle>
        </DialogHeader>
        <DialogBody className="space-y-3 text-sm">
          {!refs && !error && <Skeleton className="h-16 w-full" />}
          {refs && !used && <p className="text-muted-foreground">{t('common.deleteConfirm', { title: group?.name })}</p>}
          {used && (
            <>
              <p>{t('userGroup.delete.inUse')}</p>
              <ul className="list-disc space-y-0.5 pl-5 text-muted-foreground">
                {REFERENCE_KINDS.filter((k) => Number(refs[k]) > 0).map((k) => (
                  <li key={k}>{t(`userGroup.delete.refs.${k}`, { count: Number(refs[k]) })}</li>
                ))}
              </ul>
              <div className="space-y-1.5">
                <p className="font-medium">{t('userGroup.delete.successor')}</p>
                <Select value={successor} onValueChange={setSuccessor}>
                  <SelectTrigger>
                    <SelectValue>{groupLabel(groups.find((g) => g.symbol === successor))}</SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {groups
                      .filter((g) => g.symbol !== group?.symbol)
                      .map((g) => (
                        <SelectItem key={g.symbol} value={g.symbol}>
                          <span>{g.name || g.symbol}</span>
                          <span className="ml-2 font-mono text-xs text-muted-foreground">{g.symbol}</span>
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">{t('userGroup.delete.successorHint')}</p>
              </div>
            </>
          )}
          {error && <p className="text-destructive">{error}</p>}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t('userPage.cancel')}
          </Button>
          <Button variant="destructive" disabled={!refs || deleting || (used && !successor)} onClick={remove}>
            {used ? t('userGroup.delete.moveAndDelete') : t('common.delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

DeleteUserGroupDialog.propTypes = {
  group: PropTypes.shape({ id: PropTypes.number, name: PropTypes.string, symbol: PropTypes.string }),
  onClose: PropTypes.func.isRequired,
  onDeleted: PropTypes.func.isRequired
};
