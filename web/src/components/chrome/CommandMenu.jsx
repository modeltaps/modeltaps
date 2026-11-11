import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { Command, defaultFilter } from 'cmdk';
import { FileText } from 'lucide-react';
import { useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';

import { useOrg } from 'contexts/OrgContext';
import { useIsAdmin, useIsRoot } from 'utils/common';
import { matchesQuery } from '@/components/settings/settingsSearch';
import { commandSections, navLabel } from './nav-config';

// ==============================|| CHROME — COMMAND PALETTE (cmdk) ||============================== //
// The panel's one search (every search button opens it; ⌘K too). Lists the current workbench
// menu, the parts and groups of 设置 and, for admins, platform admin with every admin page and the
// platform settings groups, so admin pages are reachable from any context. Settings cards are
// indexed too, but only show once something is typed. On a settings page, "On this page" lists
// the page's cards (getPageEntries, read when the palette opens) and matches everything written
// on them.

// Values of "On this page" entries start with this; they match when every word of the query is
// in the card's text (fuzzy matching over a whole card would match almost anything). Everything
// else keeps cmdk's fuzzy matching.
const PAGE_ENTRY = 'page-card:';

export function commandFilter(value, search, keywords) {
  if (value.startsWith(PAGE_ENTRY)) return matchesQuery([value.slice(PAGE_ENTRY.length), ...(keywords || [])].join(' '), search) ? 1 : 0;
  return defaultFilter(value, search, keywords);
}

export default function CommandMenu({ open, setOpen, getPageEntries }) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();
  const { currentOrgId, orgRole, routingPolicyEnabled } = useOrg();
  const [search, setSearch] = useState('');

  useEffect(() => {
    if (!open) setSearch('');
  }, [open]);

  // The current page's cards, read each time the palette opens (they reflect what is rendered).
  const pageEntries = useMemo(() => (open && getPageEntries ? getPageEntries() : []), [open, getPageEntries]);
  const typing = search.trim() !== '';

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
      <Command className="flex flex-col" filter={commandFilter}>
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
          {typing && pageEntries.length > 0 && (
            <Command.Group
              heading={t('command.thisPage')}
              className="px-1 py-1 text-xs font-medium text-muted-foreground [&_[cmdk-group-items]]:mt-1 [&_[cmdk-group-items]]:space-y-0.5"
            >
              {pageEntries.map((entry) => (
                <Command.Item
                  key={entry.id}
                  value={`${PAGE_ENTRY}${entry.id} ${entry.title} ${entry.group}`}
                  keywords={[entry.text]}
                  onSelect={() => go(entry.to)}
                  className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-2 text-sm font-normal text-foreground aria-selected:bg-muted"
                >
                  <FileText className="size-4 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1 truncate">{entry.title}</span>
                  <span className="shrink-0 text-xs text-muted-foreground">{entry.group}</span>
                </Command.Item>
              ))}
            </Command.Group>
          )}
          {sections.map((section) => (
            <Command.Group
              key={section.id}
              heading={section.labelKey ? t(section.labelKey) : undefined}
              className="px-1 py-1 text-xs font-medium text-muted-foreground [&_[cmdk-group-items]]:mt-1 [&_[cmdk-group-items]]:space-y-0.5"
            >
              {section.items
                .filter((item) => !item.searchOnly || typing)
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

CommandMenu.propTypes = {
  open: PropTypes.bool,
  setOpen: PropTypes.func.isRequired,
  // () => [{ id, title, group, to, text }]: the current page's entries (SearchContext).
  getPageEntries: PropTypes.func
};
