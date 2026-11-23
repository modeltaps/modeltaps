import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Gauge, Loader2 } from 'lucide-react';

import { API } from 'utils/api';
import { Button } from '@/components/ui/button';
import { NumberField } from '@/components/ui/number-field';
import { Dialog, DialogContent, DialogBody, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { OptionList } from '@/components/settings/cards';
import { InlineRow } from '../Setting/parts';

// How many routing policies one user and one organization may create (root only). The caps
// live here, next to the policies they limit; links to them (the settings catalog, the supply
// allocation settings) open this dialog with ?highlight=<key>.
export const ROUTING_LIMIT_KEYS = ['PersonalRoutingPolicyMaxCount', 'OrganizationRoutingPolicyMaxCount'];

const asText = (v) => (v === undefined || v === null ? '' : String(v));

export default function RoutingLimits({ trigger = true }) {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const highlight = searchParams.get('highlight');
  const [open, setOpen] = useState(() => ROUTING_LIMIT_KEYS.includes(highlight));
  const [origin, setOrigin] = useState(null);
  const [values, setValues] = useState({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (ROUTING_LIMIT_KEYS.includes(highlight)) setOpen(true);
  }, [highlight]);

  useEffect(() => {
    if (!open) return undefined;
    let live = true;
    API.get('/api/option/')
      .then((res) => {
        if (!live || !res.data?.success) return;
        const found = Object.fromEntries(
          (res.data.data || []).filter((o) => ROUTING_LIMIT_KEYS.includes(o.key)).map((o) => [o.key, asText(o.value)])
        );
        setOrigin(found);
        setValues(found);
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [open]);

  const close = () => {
    setOpen(false);
    if (highlight) {
      const next = new URLSearchParams(searchParams);
      next.delete('highlight');
      setSearchParams(next, { replace: true });
    }
  };

  const changed = origin ? ROUTING_LIMIT_KEYS.filter((k) => asText(values[k]) !== asText(origin[k])) : [];
  const save = async () => {
    for (const key of changed) {
      const n = Number(values[key]);
      if (!Number.isInteger(n) || n < 0) return toast.error(t('routingPolicy.limits.invalid'));
    }
    setSaving(true);
    try {
      for (const key of changed) {
        const res = await API.put('/api/option/', { key, value: String(Number(values[key])) });
        if (!res.data?.success) return toast.error(res.data?.message);
      }
      toast.success(t('routingPolicy.limits.saved'));
      close();
    } catch {
      /* shown by the API client */
    } finally {
      setSaving(false);
    }
    return undefined;
  };

  return (
    <>
      {trigger && (
        <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
          <Gauge className="size-4" />
          {t('settingsUi.platform.cards.routing-limits.title')}
        </Button>
      )}
      <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('settingsUi.platform.cards.routing-limits.title')}</DialogTitle>
            <DialogDescription>{t('settingsUi.platform.cards.routing-limits.description')}</DialogDescription>
          </DialogHeader>
          <DialogBody>
            <OptionList>
              {ROUTING_LIMIT_KEYS.map((key) => (
                <InlineRow key={key} id={key}>
                  <NumberField
                    id={key}
                    min={0}
                    value={values[key] ?? ''}
                    disabled={!origin}
                    placeholder="20"
                    onChange={(next) => setValues((v) => ({ ...v, [key]: next }))}
                  />
                </InlineRow>
              ))}
            </OptionList>
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={close} disabled={saving}>
              {t('common.cancel')}
            </Button>
            <Button onClick={save} disabled={saving || changed.length === 0}>
              {saving && <Loader2 className="size-4 animate-spin" />}
              {t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

RoutingLimits.propTypes = { trigger: PropTypes.bool };
