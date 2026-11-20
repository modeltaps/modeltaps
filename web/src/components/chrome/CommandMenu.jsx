import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { Command, defaultFilter } from 'cmdk';
import { FileText, ListChecks, Sparkles } from 'lucide-react';
import { useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';

import { useOrg } from 'contexts/OrgContext';
import { useIsAdmin, useIsRoot } from 'utils/common';
import { normalizeSynonyms, packTiers, scoreTiers, unpackTiers } from './commandScore';
import { loadCollectionItems } from './collectionIndex';
import SettingsAssistantDialog from './SettingsAssistantDialog';
import { cardTitleKey, groupLabelKey } from 'views/panel/Setting/registry';
import { commandSections, navLabel } from './nav-config';
import { orgSlugOf } from 'routes/paths';

// ==============================|| CHROME — COMMAND PALETTE (cmdk) ||============================== //
// The panel's one search (every search button opens it; ⌘K too). Lists the current workbench
// menu, the parts and groups of 设置 and, for admins, platform admin with every admin page and the
// platform settings groups, so admin pages are reachable from any context. Settings cards are
// indexed too, but only show once something is typed. On a settings page, "On this page" lists
// the page's cards (getPageEntries, read when the palette opens) and matches everything written
// on them.

// Values of "On this page" entries start with this; their keywords are the card's title, group and
// text in the scoring tiers (commandScore.js).
const PAGE_ENTRY = 'page-card:';

// Values of the items of collection cards (a payment gateway, an OIDC provider…): their keywords are
// the item's name, then the card and what else describes the item, in the scoring tiers.
const COLLECTION_ENTRY = 'collection-item:';

// The "Ask AI" entry (root only): its value only carries the prefix, and it matches any query,
// at the top when the query reads like a question, else below everything that matched.
const ASK_ENTRY = 'ask-ai:';
const QUESTION =
  /[?？]|怎么|怎样|如何|为什么|哪里|在哪|什么|能否|能不能|可以|是否|吗|嗎|麼|ですか|どこ|どう|なぜ|\b(how|why|where|what|which|can|should|does|do|is)\b/i;
export const looksLikeQuestion = (text) => QUESTION.test(text) || String(text).trim().split(/\s+/).length >= 4;

// Values of settings catalog entries: `setting-field:<key>|<scope>|<advanced 0/1>`, their search
// words in tiers in the keywords.
const FIELD_ENTRY = 'setting-field:';

// Filters a query may start with: @key:<part of a key>, @object:<scope> (site, channel, model,
// supply, allocation, user_group, routing_policy, organization, token) and @advanced (only the
// rarely changed settings). The rest of the query is matched as words.
export function parseSettingFilters(search) {
  const filters = { key: null, object: null, advanced: false, any: false };
  const words = [];
  for (const token of String(search || '')
    .trim()
    .split(/\s+/)) {
    const lower = token.toLowerCase();
    if (lower.startsWith('@key:')) filters.key = lower.slice(5);
    else if (lower.startsWith('@object:')) filters.object = lower.slice(8);
    else if (lower === '@advanced') filters.advanced = true;
    else {
      if (token) words.push(token);
      continue;
    }
    filters.any = true;
  }
  return { filters, text: words.join(' ') };
}

export const settingFieldValue = (entry) => `${FIELD_ENTRY}${entry.key}|${entry.scope}|${entry.advanced ? 1 : 0}`;

// Advanced settings are found too, a little below the others.
const ADVANCED_WEIGHT = 0.9;
// Menu entries (pages, cards) are named by their value; scattered letters still find them for a
// Latin query ("chnl" → Channels), at a score below any word match.
const FUZZY_WEIGHT = 0.15;
const LATIN = /^[\x20-\x7e]+$/;

// The palette's filter (cmdk), with the synonym table of the current language.
export function makeCommandFilter(synonyms = {}) {
  return function commandFilter(value, search, keywords) {
    const { filters, text } = parseSettingFilters(search);
    if (value.startsWith(FIELD_ENTRY)) {
      const [key, scope, advanced] = value.slice(FIELD_ENTRY.length).split('|');
      if (filters.key && !key.toLowerCase().includes(filters.key)) return 0;
      if (filters.object && scope !== filters.object) return 0;
      if (filters.advanced && advanced !== '1') return 0;
      if (!text) return filters.any ? 1 : 0;
      const score = scoreTiers(text, unpackTiers(keywords), synonyms);
      return advanced === '1' ? score * ADVANCED_WEIGHT : score;
    }
    // The @ filters only apply to settings.
    if (filters.any) return 0;
    if (value.startsWith(ASK_ENTRY)) return search.trim().length < 2 ? 0 : looksLikeQuestion(search) ? 1 : 0.01;
    if (value.startsWith(PAGE_ENTRY) || value.startsWith(COLLECTION_ENTRY)) return scoreTiers(search, unpackTiers(keywords), synonyms);
    const score = scoreTiers(search, { primary: [value], secondary: keywords || [] }, synonyms);
    if (score > 0 || !LATIN.test(search)) return score;
    return defaultFilter(value, search, keywords) * FUZZY_WEIGHT;
  };
}

export const commandFilter = makeCommandFilter();

const GROUP_CLASS = 'px-1 py-1 text-xs font-medium text-muted-foreground [&_[cmdk-group-items]]:mt-1 [&_[cmdk-group-items]]:space-y-0.5';

export default function CommandMenu({ open, setOpen, getPageEntries }) {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { t, i18n } = useTranslation();
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();
  const { currentOrg, orgRole, routingPolicyEnabled } = useOrg();
  const [search, setSearch] = useState('');
  // The question an "Ask AI" entry opened the assistant with.
  const [askQuestion, setAskQuestion] = useState(null);
  // The synonyms of the current language plus the English ones: people type terms of other
  // gateways in either.
  const filter = useMemo(() => {
    const table = (lng) => i18n.getResource(lng, 'translation', 'command.synonyms');
    return makeCommandFilter(normalizeSynonyms(table('en_US'), table(i18n.language)));
  }, [i18n, i18n.language]);

  useEffect(() => {
    if (!open) setSearch('');
  }, [open]);

  // The items of the collection cards, read each time the palette opens (platform admins only).
  const [collectionItems, setCollectionItems] = useState([]);
  useEffect(() => {
    if (!open || !isAdmin) return undefined;
    let alive = true;
    loadCollectionItems({ isRoot }).then((items) => alive && setCollectionItems(items));
    return () => {
      alive = false;
    };
  }, [open, isAdmin, isRoot]);

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
    // The URL's organization on its pages; the active one elsewhere (设置).
    orgSlug: orgSlugOf(pathname) ?? currentOrg?.slug ?? null,
    orgRole,
    features: { routingPolicy: routingPolicyEnabled }
  });

  // Every group with its entries: value and keywords as the filter reads them. While typing the
  // groups and their entries are ordered here by score: cmdk 1.1.1 never reorders groups (it looks
  // them up by an id they do not carry) and moves entries behind React's back.
  const pageGroup = typing &&
    pageEntries.length > 0 && {
      id: 'page',
      heading: t('command.thisPage'),
      entries: pageEntries.map((entry) => ({
        key: entry.id,
        value: `${PAGE_ENTRY}${entry.id}`,
        keywords: packTiers({ primary: [entry.title], secondary: [entry.group], tertiary: [entry.text] }),
        href: entry.to,
        content: (
          <>
            <FileText className="size-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate">{entry.title}</span>
            <span className="shrink-0 text-xs text-muted-foreground">{entry.group}</span>
          </>
        )
      }))
    };
  const sectionGroups = sections.map((section) => ({
    id: section.id,
    heading: section.labelKey ? t(section.labelKey) : undefined,
    entries: section.items
      .filter((item) => !item.searchOnly || typing)
      .map((item) => {
        const Icon = item.icon;
        if (item.setting) {
          return {
            key: `${section.id}-${item.id}`,
            value: settingFieldValue(item.setting),
            keywords: packTiers(item.search(t)),
            href: item.href,
            content: (
              <>
                <Icon className="size-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate">
                  {item.label(t)}
                  <span className="ml-2 text-xs text-muted-foreground">{item.prefix(t)}</span>
                  {item.setting.advanced && (
                    <span className="ml-1.5 rounded bg-muted px-1 py-px text-[10px] text-muted-foreground">{t('command.advanced')}</span>
                  )}
                </span>
                <span className="shrink-0 font-mono text-[11px] text-muted-foreground">{item.detail}</span>
              </>
            )
          };
        }
        const label = navLabel(t, item);
        return {
          key: `${section.id}-${item.id}`,
          value: `${section.admin ? `${t('nav.context.admin')} ` : ''}${label} ${item.id}`,
          keywords: undefined,
          href: item.href,
          content: (
            <>
              <Icon className="size-4 text-muted-foreground" />
              {label}
            </>
          )
        };
      })
  }));
  const collectionEntries = typing
    ? collectionItems.map(({ id, group, card, href, item, collection }) => {
        const name = collection.name(item, t) || `#${item.id}`;
        const where = `${t(groupLabelKey(group))} › ${t(cardTitleKey(card))}`;
        return {
          key: `collection-${id}`,
          value: `${COLLECTION_ENTRY}${id}`,
          keywords: packTiers({ primary: [name], secondary: [t(cardTitleKey(card)), ...collection.words(item, t).filter(Boolean)] }),
          href,
          content: (
            <>
              <ListChecks className="size-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate">
                {name}
                <span className="ml-2 text-xs text-muted-foreground">{where}</span>
              </span>
            </>
          )
        };
      })
    : [];
  const platformGroup = sectionGroups.find((group) => group.id === 'platform_settings');
  if (platformGroup) platformGroup.entries.push(...collectionEntries);
  const askGroup = typing &&
    isRoot && {
      id: 'ask',
      heading: t('settingsAssistant.group'),
      entries: [
        {
          key: 'ask-ai',
          value: ASK_ENTRY,
          keywords: undefined,
          onSelect: () => {
            setOpen(false);
            setAskQuestion(search.trim());
          },
          content: (
            <>
              <Sparkles className="size-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate">{t('settingsAssistant.entry', { question: search.trim() })}</span>
            </>
          )
        }
      ]
    };
  const groups = [pageGroup, ...sectionGroups, askGroup].filter(Boolean);
  if (typing) {
    // Entries and groups by score, best first (ties keep the menu order). cmdk would also move the
    // entries in the DOM; doing it in React keeps the two orders the same.
    const byScore = (list, score) =>
      list
        .map((x, i) => ({ x, i, score: score(x) }))
        .sort((a, b) => b.score - a.score || a.i - b.i)
        .map((y) => ({ ...y.x, score: y.score }));
    const sorted = byScore(
      groups.map((group) => ({ ...group, entries: byScore(group.entries, (entry) => filter(entry.value, search, entry.keywords)) })),
      (group) => group.entries[0]?.score || 0
    );
    groups.splice(0, groups.length, ...sorted);
  }

  const go = (url) => {
    setOpen(false);
    navigate(url);
  };

  return (
    <>
      <Command.Dialog
        open={open}
        onOpenChange={setOpen}
        label={t('command.search', { defaultValue: 'Search' })}
        overlayClassName="fixed inset-0 z-[1300] bg-black/40 backdrop-blur-sm"
        contentClassName="fixed left-1/2 top-[20%] z-[1301] w-full max-w-lg -translate-x-1/2 overflow-hidden rounded-2xl border border-border bg-card shadow-2xl"
      >
        <Command className="flex flex-col" filter={filter}>
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
            {groups.map((group) => (
              <Command.Group key={group.id} heading={group.heading} className={GROUP_CLASS}>
                {group.entries.map((entry) => (
                  <Command.Item
                    key={entry.key}
                    value={entry.value}
                    keywords={entry.keywords}
                    onSelect={entry.onSelect || (() => go(entry.href))}
                    className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-2 text-sm font-normal text-foreground aria-selected:bg-muted"
                  >
                    {entry.content}
                  </Command.Item>
                ))}
              </Command.Group>
            ))}
          </Command.List>
        </Command>
      </Command.Dialog>
      {askQuestion !== null && <SettingsAssistantDialog question={askQuestion} onClose={() => setAskQuestion(null)} />}
    </>
  );
}

CommandMenu.propTypes = {
  open: PropTypes.bool,
  setOpen: PropTypes.func.isRequired,
  // () => [{ id, title, group, to, text }]: the current page's entries (SearchContext).
  getPageEntries: PropTypes.func
};
