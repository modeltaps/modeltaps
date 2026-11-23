import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Loader2, Settings2 } from 'lucide-react';

import { API } from 'utils/api';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { NumberField } from '@/components/ui/number-field';
import { Dialog, DialogContent, DialogBody, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { OptionList } from '@/components/settings/cards';
import { InlineRow, ToggleRow } from '../Setting/parts';
import { Emphasis } from '@/components/settings/SettingHelp';

// The platform settings of routing policies (root only), next to the policies they govern:
// whether the decision call is billed to the caller, and whether users and organizations may
// create policies of their own and how many. Links to any of them (the settings catalog, older
// settings page links) open this dialog with ?highlight=<key>.
const OWN_SWITCH = 'UserRoutingPolicyEnabled';
const LIMIT_KEYS = ['PersonalRoutingPolicyMaxCount', 'OrganizationRoutingPolicyMaxCount'];
export const ROUTING_SETTING_KEYS = ['RoutingDecisionBillingEnabled', OWN_SWITCH, ...LIMIT_KEYS];

const asText = (v) => (v === undefined || v === null ? '' : String(v));
const card = (t, part) => t(`settingsUi.platform.cards.routing-settings.${part}`);

export default function RoutingSettings() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const highlight = searchParams.get('highlight');
  const [open, setOpen] = useState(() => ROUTING_SETTING_KEYS.includes(highlight));
  const [origin, setOrigin] = useState(null);
  const [values, setValues] = useState({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (ROUTING_SETTING_KEYS.includes(highlight)) setOpen(true);
  }, [highlight]);

  useEffect(() => {
    if (!open) return undefined;
    let live = true;
    API.get('/api/option/')
      .then((res) => {
        if (!live || !res.data?.success) return;
        const found = Object.fromEntries(
          (res.data.data || []).filter((o) => ROUTING_SETTING_KEYS.includes(o.key)).map((o) => [o.key, asText(o.value)])
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

  const set = (key, value) => setValues((v) => ({ ...v, [key]: value }));
  const changed = origin ? ROUTING_SETTING_KEYS.filter((k) => asText(values[k]) !== asText(origin[k])) : [];
  const save = async () => {
    for (const key of changed.filter((k) => LIMIT_KEYS.includes(k))) {
      const n = Number(values[key]);
      if (!Number.isInteger(n) || n < 0) return toast.error(t('routingPolicy.limits.invalid'));
    }
    setSaving(true);
    try {
      for (const key of changed) {
        const value = LIMIT_KEYS.includes(key) ? String(Number(values[key])) : values[key];
        const res = await API.put('/api/option/', { key, value });
        if (!res.data?.success) return toast.error(res.data?.message);
      }
      toast.success(card(t, 'saved'));
      close();
    } catch {
      /* shown by the API client */
    } finally {
      setSaving(false);
    }
    return undefined;
  };

  const toggle = (key) => (
    <InlineRow key={key} id={key}>
      <Switch id={key} checked={values[key] === 'true'} disabled={!origin} onCheckedChange={(v) => set(key, v ? 'true' : 'false')} />
    </InlineRow>
  );
  const ownOn = values[OWN_SWITCH] === 'true';
  return (
    <>
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
        <Settings2 className="size-4" />
        {card(t, 'title')}
      </Button>
      <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{card(t, 'title')}</DialogTitle>
            <DialogDescription>{card(t, 'description')}</DialogDescription>
          </DialogHeader>
          <DialogBody className="space-y-5">
            <section className="space-y-2">
              <h3 className="text-xs font-medium text-muted-foreground">{card(t, 'billing')}</h3>
              <OptionList>
                {/* The one thing to know about this switch: where the fee comes from. */}
                <ToggleRow
                  id="RoutingDecisionBillingEnabled"
                  description={<Emphasis text={card(t, 'billingSource')} />}
                  checked={values.RoutingDecisionBillingEnabled === 'true'}
                  disabled={!origin}
                  onCheckedChange={(v) => set('RoutingDecisionBillingEnabled', v ? 'true' : 'false')}
                />
              </OptionList>
            </section>
            <section className="space-y-2">
              <h3 className="text-xs font-medium text-muted-foreground">{card(t, 'own')}</h3>
              <OptionList>
                {toggle(OWN_SWITCH)}
                {/* The caps only matter while own policies are allowed. */}
                {LIMIT_KEYS.map((key) => (
                  <InlineRow key={key} id={key}>
                    <NumberField
                      id={key}
                      min={0}
                      value={values[key] ?? ''}
                      disabled={!origin || !ownOn}
                      placeholder="20"
                      onChange={(next) => set(key, next)}
                    />
                  </InlineRow>
                ))}
              </OptionList>
            </section>
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
