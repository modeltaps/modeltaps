import { Fragment, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Command } from 'cmdk';
import { ChevronDown, Plus } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { FloatingPanel } from '@/components/ui/floating';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { channelTypeSections } from 'constants/ChannelConstants';
import { useProviderLabel } from './ProviderPicker';

// "New channel": a searchable dropdown of providers by category; picking one opens the new
// channel sheet for it. Providers already in use come first, then the common ones.
export default function NewChannelMenu({ onPick, present = [] }) {
  const { t } = useTranslation();
  const labelOf = useProviderLabel();
  const [open, setOpen] = useState(false);
  const triggerRef = useRef(null);
  const panelRef = useRef(null);

  useEffect(() => {
    if (!open) return undefined;
    const onDocClick = (e) => {
      if (triggerRef.current?.contains(e.target) || panelRef.current?.contains(e.target)) return;
      setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const sections = useMemo(() => {
    const all = channelTypeSections(labelOf);
    const byType = new Map(all.flatMap((s) => s.options).map((o) => [o.type, o]));
    const used = present.map((type) => byType.get(type)).filter(Boolean);
    const usedSet = new Set(used.map((o) => o.type));
    const common = (all.find((s) => s.key === 'common')?.options || []).filter((o) => !usedSet.has(o.type));
    const listed = new Set([...usedSet, ...common.map((o) => o.type)]);
    return [
      ...(used.length ? [{ key: 'inUse', heading: t('channels.newMenu.inUse'), options: used }] : []),
      ...(common.length ? [{ key: 'common', heading: t('providerPicker.category.common'), options: common }] : []),
      ...all
        .filter((s) => s.key !== 'common')
        .map((s) => ({ key: s.key, heading: t(`providerPicker.category.${s.key}`), options: s.options.filter((o) => !listed.has(o.type)) }))
        .filter((s) => s.options.length > 0)
    ];
  }, [labelOf, present, t]);

  const pick = (type) => {
    setOpen(false);
    onPick(type);
  };

  return (
    <>
      <Button ref={triggerRef} className="rounded-lg" aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen((o) => !o)}>
        <Plus />
        {t('channels.newChannel')}
        <ChevronDown className="opacity-70" />
      </Button>
      <FloatingPanel ref={panelRef} open={open} anchorRef={triggerRef} align="end" clamp={false}>
        <Command
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.preventDefault();
              setOpen(false);
            }
          }}
          className="w-80 overflow-hidden rounded-xl border border-border bg-popover text-popover-foreground shadow-lg"
        >
          <div className="border-b border-border px-3">
            <Command.Input
              autoFocus
              placeholder={t('providerPicker.search')}
              className="h-10 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
          </div>
          <Command.List className="max-h-[360px] overflow-y-auto p-1">
            <Command.Empty className="px-2 py-4 text-center text-sm text-muted-foreground">{t('providerPicker.empty')}</Command.Empty>
            {sections.map((s) => (
              <Fragment key={s.key}>
                <Command.Group
                  heading={s.heading}
                  className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:text-muted-foreground"
                >
                  {s.options.map((o) => (
                    <Command.Item
                      key={`${s.key}-${o.value}`}
                      value={`${o.label} ${s.key}`}
                      onSelect={() => pick(o.type)}
                      className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 text-sm outline-none data-[selected=true]:bg-muted"
                    >
                      <ProviderIcon type={o.type} name={o.label} className="size-4 shrink-0" />
                      <span className="truncate">{o.label}</span>
                      <span className="ml-auto shrink-0 pl-3 text-xs text-muted-foreground">{t(`providerPicker.auth.${o.auth}`)}</span>
                    </Command.Item>
                  ))}
                </Command.Group>
              </Fragment>
            ))}
          </Command.List>
        </Command>
      </FloatingPanel>
    </>
  );
}

NewChannelMenu.propTypes = { onPick: PropTypes.func.isRequired, present: PropTypes.arrayOf(PropTypes.number) };
