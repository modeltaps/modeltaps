import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { ObjectSheet, SheetBar } from './ObjectSheet';
import useTabDrafts from './useTabDrafts';

// Tab bar and tab content of an object panel, in one bordered box. The open tab carries a dot
// while it has unsaved changes. A small object with a single tab shows its content without a
// tab bar.
export function ObjectTabs({ tabs, current, onSelect, dirty, label, children }) {
  const { t } = useTranslation();
  if (tabs.length === 1) {
    return <div className="rounded-2xl border border-border bg-card px-5 pb-5 pt-1">{children}</div>;
  }
  return (
    <div className="rounded-2xl border border-border bg-card">
      <div role="tablist" aria-label={label} className="flex gap-0.5 overflow-x-auto border-b border-border p-1">
        {tabs.map((x) => (
          <button
            key={x.key}
            type="button"
            role="tab"
            aria-selected={x.key === current}
            onClick={() => onSelect(x.key)}
            className={cn(
              'inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-3 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              x.key === current ? 'bg-control text-foreground' : 'text-muted-foreground hover:bg-foreground/[0.05] hover:text-foreground'
            )}
          >
            {x.label}
            {x.key === current && dirty && <span className="size-1.5 rounded-full bg-warning" aria-label={t('objects.panel.unsaved')} />}
          </button>
        ))}
      </div>
      <div role="tabpanel" className="px-5 pb-5 pt-1">
        {children}
      </div>
    </div>
  );
}

ObjectTabs.propTypes = {
  tabs: PropTypes.arrayOf(PropTypes.shape({ key: PropTypes.string.isRequired, label: PropTypes.node })).isRequired,
  current: PropTypes.string,
  onSelect: PropTypes.func.isRequired,
  dirty: PropTypes.bool,
  label: PropTypes.string,
  children: PropTypes.node
};

// Footer of a tab that saves on its own: unsaved marker or the scope of the save, Discard and
// Save.
export function SaveBar({ dirty, saving, scope, saveLabel, onDiscard, onSave }) {
  const { t } = useTranslation();
  return (
    <SheetBar
      statusClassName={dirty ? 'text-foreground' : undefined}
      status={
        <>
          {dirty && <span className="mr-2 inline-block size-1.5 rounded-full bg-warning align-[1px]" />}
          {dirty ? t('objects.panel.unsaved') : scope || t('objects.panel.scope')}
        </>
      }
    >
      <Button variant="outline" className="rounded-lg" disabled={!dirty || saving} onClick={onDiscard}>
        {t('objects.panel.discard')}
      </Button>
      <Button className="rounded-lg" disabled={!dirty || saving} onClick={onSave}>
        {saving && <Loader2 className="animate-spin" />}
        {saveLabel || t('objects.panel.save')}
      </Button>
    </SheetBar>
  );
}

SaveBar.propTypes = {
  dirty: PropTypes.bool,
  saving: PropTypes.bool,
  scope: PropTypes.node,
  saveLabel: PropTypes.node,
  onDiscard: PropTypes.func.isRequired,
  onSave: PropTypes.func.isRequired
};

// Right-side panel of one object: header (icon, title, subtitle, actions, close) and a box with
// one tab per part of the object. Each tab saves on its own through `onSave(patch, { tab })`
// (which resolves to `{ success, message }`), shows its own Discard / Save footer, marks unsaved
// changes and asks before they are dropped. Read-only tabs (no `patch`) have no footer.
//
// `tabs` are `{ key, label, make, patch, Body, saveLabel }`; `Body` receives
// `{ object, draft, setDraft, ctx }` and is remounted on Discard. `ctx` is handed to `make`,
// `patch` and `Body` (with `version` added).
export default function ObjectPanel({
  titleId,
  size = 'lg',
  object,
  error,
  errorTitle,
  icon,
  title,
  subtitle,
  subtitleMono,
  actions,
  tabs,
  tab,
  onTabChange,
  onClose,
  onSave,
  onSaved,
  ctx,
  tabsLabel,
  scope,
  notice,
  children
}) {
  const { t } = useTranslation();
  const current = tabs.find((x) => x.key === tab) || tabs[0];
  const { ready, dirty, draft, setDraft, version, saving, requestClose, switchTab, discard, save } = useTabDrafts({
    object,
    tab: current,
    ctx,
    onTabChange,
    onClose,
    onSave,
    onSaved
  });
  const Body = current.Body;

  return (
    <ObjectSheet
      titleId={titleId}
      size={size}
      icon={object ? icon : null}
      title={object ? title : error ? errorTitle || t('objects.notFound') : '…'}
      subtitle={object ? subtitle : null}
      subtitleMono={subtitleMono}
      actions={object ? actions : null}
      onClose={requestClose}
      footer={
        ready && current.patch ? (
          <SaveBar dirty={dirty} saving={saving} scope={scope} saveLabel={current.saveLabel} onDiscard={discard} onSave={save} />
        ) : null
      }
    >
      {error ? (
        <p className="py-16 text-center text-sm text-muted-foreground">{error}</p>
      ) : !object ? (
        // Shaped like a panel body: the tab bar, a field and a few label / value rows.
        <div className="space-y-6 pt-1" aria-busy="true">
          <div className="flex gap-2">
            {[56, 48, 48, 56].map((w, i) => (
              <Skeleton key={i} className="h-7 rounded-md" style={{ width: w }} />
            ))}
          </div>
          <div className="space-y-2">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-10 w-full rounded-lg" />
          </div>
          <div className="space-y-4">
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className="flex items-center justify-between gap-6">
                <Skeleton className="h-3.5 w-20" />
                <Skeleton className="h-3.5 w-32" />
              </div>
            ))}
          </div>
        </div>
      ) : (
        <>
          {notice}
          <ObjectTabs tabs={tabs} current={current.key} onSelect={switchTab} dirty={dirty} label={tabsLabel}>
            {ready && <Body key={version} object={object} draft={draft} setDraft={setDraft} ctx={{ ...ctx, version }} />}
          </ObjectTabs>
        </>
      )}
      {children}
    </ObjectSheet>
  );
}

ObjectPanel.propTypes = {
  titleId: PropTypes.string.isRequired,
  size: PropTypes.oneOf(['lg', 'md', 'sm']),
  object: PropTypes.object,
  error: PropTypes.node,
  errorTitle: PropTypes.node,
  icon: PropTypes.node,
  title: PropTypes.node,
  subtitle: PropTypes.node,
  subtitleMono: PropTypes.bool,
  actions: PropTypes.node,
  tabs: PropTypes.array.isRequired,
  tab: PropTypes.string,
  onTabChange: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  onSave: PropTypes.func,
  onSaved: PropTypes.func,
  ctx: PropTypes.object,
  tabsLabel: PropTypes.string,
  scope: PropTypes.node,
  // Shown above the tabs once the object has loaded, e.g. what needs doing on it.
  notice: PropTypes.node,
  children: PropTypes.node
};
