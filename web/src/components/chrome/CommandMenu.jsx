import { useEffect } from 'react';
import { Command } from 'cmdk';
import { useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { useIsAdmin, useIsRoot } from 'utils/common';
import { filterSections, navLabel } from './nav-config';

// ==============================|| CHROME — COMMAND PALETTE (cmdk) ||============================== //

export default function CommandMenu({ open, setOpen }) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();
  const siteInfo = useSelector((state) => state.siteInfo);

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

  const sections = filterSections({
    isAdmin,
    isRoot,
    invoiceEnabled: Boolean(siteInfo.UserInvoiceMonth),
    chatEnabled: siteInfo.builtin_chat_enabled !== false
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
      contentClassName="fixed left-1/2 top-[20%] z-[1301] w-full max-w-lg -translate-x-1/2 overflow-hidden rounded-xl border border-border bg-card shadow-2xl"
    >
      <Command className="flex flex-col">
        <div className="border-b border-border px-3">
          <Command.Input
            placeholder={t('command.placeholder')}
            className="h-12 w-full bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
          />
        </div>
        <Command.List className="max-h-80 overflow-y-auto p-2">
          <Command.Empty className="py-6 text-center text-sm text-muted-foreground">
            {t('command.empty')}
          </Command.Empty>
          {sections.map((section) => (
            <Command.Group
              key={section.id}
              heading={t(section.id)}
              className="px-1 py-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground [&_[cmdk-group-items]]:mt-1 [&_[cmdk-group-items]]:space-y-0.5"
            >
              {section.items.map((item) => {
                const Icon = item.icon;
                const label = navLabel(t, item);
                return (
                  <Command.Item
                    key={item.id}
                    value={`${label} ${item.id}`}
                    onSelect={() => go(item.url)}
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
