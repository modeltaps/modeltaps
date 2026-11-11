import { useEffect, useState } from 'react';
import { Command } from 'cmdk';
import { useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';

import { useOrg } from 'contexts/OrgContext';
import { useIsAdmin, useIsRoot } from 'utils/common';
import { commandSections, navLabel } from './nav-config';

// ==============================|| CHROME — COMMAND PALETTE (cmdk) ||============================== //
// Lists the current workbench menu, personal settings and, for admins, platform admin with every
// admin page and the platform settings groups, so admin pages are reachable from any context.
// Settings cards are indexed too, but only show once something is typed.

export default function CommandMenu({ open, setOpen }) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();
  const { currentOrgId, orgRole, routingPolicyEnabled } = useOrg();
  const [search, setSearch] = useState('');

  useEffect(() => {
    if (!open) setSearch('');
  }, [open]);

  useEffect(() => {
    const onKeyDown = (e) => {
      if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((v) => !v);
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [setOpen]);

  const sections = commandSections({
    isAdmin,
    isRoot,
    orgId: currentOrgId,
    orgRole,
    features: { routingPolicy: routingPolicyEnabled }
  });

  const go = (url) => {
    setOpen(false);
    navigate(url);
  };

  return (
    <Command.Dialog
      open={open}
      onOpenChange={setOpen}
      label={t('command.search', { defaultValue: 'Search' })}
      overlayClassName="fixed inset-0 z-[1300] bg-black/40 backdrop-blur-sm"
      contentClassName="fixed left-1/2 top-[20%] z-[1301] w-full max-w-lg -translate-x-1/2 overflow-hidden rounded-2xl border border-border bg-card shadow-2xl"
    >
      <Command className="flex flex-col">
        <div className="border-b border-border px-3">
          <Command.Input
            value={search}
            onValueChange={setSearch}
            placeholder={t('command.placeholder')}
            className="h-12 w-full bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
          />
        </div>
        <Command.List className="max-h-80 overflow-y-auto p-2">
          <Command.Empty className="py-6 text-center text-sm text-muted-foreground">{t('command.empty')}</Command.Empty>
          {sections.map((section) => (
            <Command.Group
              key={section.id}
              heading={section.labelKey ? t(section.labelKey) : undefined}
              className="px-1 py-1 text-xs font-medium text-muted-foreground [&_[cmdk-group-items]]:mt-1 [&_[cmdk-group-items]]:space-y-0.5"
            >
              {section.items
                .filter((item) => !item.searchOnly || search.trim() !== '')
                .map((item) => {
                  const Icon = item.icon;
                  const label = navLabel(t, item);
                  return (
                    <Command.Item
                      key={`${section.id}-${item.id}`}
                      value={`${section.admin ? `${t('nav.context.admin')} ` : ''}${label} ${item.id}`}
                      onSelect={() => go(item.href)}
                      className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-2 text-sm font-normal text-foreground aria-selected:bg-muted"
                    >
                      <Icon className="size-4 text-muted-foreground" />
                      {label}
                    </Command.Item>
                  );
                })}
            </Command.Group>
          ))}
        </Command.List>
      </Command>
    </Command.Dialog>
  );
}
