import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search } from 'lucide-react';

import { Input } from '@/components/ui/input';
import UserAvatar, { accountDisplayName } from '@/components/chrome/avatar';
import { ContextMark } from '@/components/chrome/ContextSwitcher';
import { cn } from '@/lib/utils';

// ==============================|| AGENT DEVICE — OWNER PICKER ||============================== //
// Whose key it is: the user's own account, then each organization they belong to, one row each
// with its mark and the user's role there, the way consent screens list the accounts an app can
// reach. One choice: a key's limits, budget, models and usage belong to exactly one account, so an
// agent working for several organizations is authorized once per organization.
//
// Past a handful of accounts a search field appears and the list scrolls; organizations chosen
// recently come first.

export const PERSONAL = 'personal';
export const SEARCH_FROM = 6; // accounts, personal included
const RECENT_KEY = 'agent-device-recent-owners';
const RECENT_MAX = 10;

function readRecent() {
  try {
    const list = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]');
    return Array.isArray(list) ? list.map(String) : [];
  } catch {
    return [];
  }
}

// Called after an approval, so the next time that account is near the top.
export function rememberOwner(value) {
  if (!value || value === PERSONAL) return;
  try {
    const next = [String(value), ...readRecent().filter((v) => v !== String(value))].slice(0, RECENT_MAX);
    localStorage.setItem(RECENT_KEY, JSON.stringify(next));
  } catch {
    // storage unavailable: the order just stays alphabetical
  }
}

// Organization owners in display order: recently chosen first, then by name.
export function orderOwners(orgOwners) {
  const recent = readRecent();
  const rank = (o) => {
    const i = recent.indexOf(o.value);
    return i === -1 ? RECENT_MAX : i;
  };
  return [...orgOwners].sort((a, b) => rank(a) - rank(b) || a.label.localeCompare(b.label));
}

function OwnerRow({ owner, user, selected, onSelect }) {
  const { t } = useTranslation();
  const personal = owner.value === PERSONAL;
  const role = owner.org?.role;
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={() => onSelect(owner.value)}
      className={cn(
        'flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring',
        selected ? 'bg-muted' : 'hover:bg-muted/60'
      )}
    >
      {personal ? (
        <UserAvatar user={user} className="size-8" textClass="text-[11px]" />
      ) : (
        <ContextMark kind="org" org={owner.org} className="size-8" />
      )}
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium">{personal ? accountDisplayName(user) : owner.label}</span>
        <span className="block truncate text-xs text-muted-foreground">
          {personal
            ? t('agentDevice.personalAccount')
            : role
              ? t('agentDevice.orgAccount', { role: t(`org.roles.${role}`, { defaultValue: role }) })
              : t('agentDevice.orgAccountPlain')}
        </span>
      </span>
      <span
        aria-hidden="true"
        className={cn(
          'size-4 shrink-0 rounded-full',
          selected
            ? 'shadow-[inset_0_0_0_5px_var(--foreground)]'
            : 'shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--foreground)_35%,transparent)]'
        )}
      />
    </button>
  );
}

OwnerRow.propTypes = {
  owner: PropTypes.object.isRequired,
  user: PropTypes.object,
  selected: PropTypes.bool,
  onSelect: PropTypes.func.isRequired
};

export default function OwnerPicker({ owners, value, user, onChange }) {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const searchable = owners.length >= SEARCH_FROM;
  const q = query.trim().toLowerCase();
  const shown = q
    ? owners.filter((o) => (o.value === PERSONAL ? accountDisplayName(user) : o.label).toLowerCase().includes(q) || o.value === value)
    : owners;

  return (
    <div>
      {searchable && (
        <div className="relative mb-2">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t('agentDevice.searchAccounts')}
            aria-label={t('agentDevice.searchAccounts')}
            className="h-9 pl-9"
          />
        </div>
      )}
      <div
        role="radiogroup"
        aria-label={t('agentDevice.owner')}
        className={cn('-mx-2 space-y-0.5', searchable && 'max-h-[232px] overflow-y-auto overscroll-contain px-0')}
        data-testid="device-owner-list"
      >
        {shown.map((o) => (
          <OwnerRow key={o.value} owner={o} user={user} selected={value === o.value} onSelect={onChange} />
        ))}
        {shown.length === 0 && <p className="px-2 py-3 text-sm text-muted-foreground">{t('agentDevice.noAccountMatch')}</p>}
      </div>
    </div>
  );
}

OwnerPicker.propTypes = {
  owners: PropTypes.array.isRequired,
  value: PropTypes.string,
  user: PropTypes.object,
  onChange: PropTypes.func.isRequired
};
