import { useMemo } from 'react';
import PropTypes from 'prop-types';

import ModelMultiSelect from '@/components/model/ModelMultiSelect';
import { allowlistCovers, filterPermitted, isPattern } from 'utils/modelAllowlist';

// ==============================|| TOKEN — MODEL LIMIT SELECTOR ||============================== //
// Model allowlist picker for tokens and organization members, on the shared ModelMultiSelect:
// vendor groups with logos, newest first, checkboxes, search. Stores the selected model ids.
// `models` are catalog rows (see toLimitModels); only listed models can be picked, ids already
// saved but no longer in the catalog stay visible so they can be removed.
// `parents` are the allowlists of the layers above (ORG-15): only models every one permits are
// offered. Typing a prefix pattern ending in "*" (e.g. claude-fable-*) adds it as a rule, so new
// releases of the family are allowed without editing the list; it must stay inside the parents too.

// Catalog rows from GET /api/available_model ({ [id]: { owned_by, vendor, groups, price } }).
export function toLimitModels(data) {
  return Object.keys(data || {}).map((id) => {
    const row = data[id] || {};
    return {
      id,
      owned_by: row.owned_by,
      vendor: row.vendor,
      groups: row.groups,
      released_at: row.price?.model_info?.released_at
    };
  });
}

const NO_PARENTS = [];

export default function ModelLimitSelector({ id, models = [], value = [], onChange, getGroupLabel, parents = NO_PARENTS }) {
  const options = useMemo(() => {
    const permitted = new Set(
      filterPermitted(
        parents,
        models.map((m) => m.id)
      )
    );
    return models
      .filter((m) => permitted.has(m.id))
      .map((m) => {
        const owner = m.vendor?.name || m.owned_by || '';
        return {
          id: m.id,
          vendor: owner ? { key: m.vendor?.slug || owner.toLowerCase(), name: owner, icon: m.vendor?.icon } : undefined,
          releasedAt: m.released_at || undefined,
          meta: m.groups?.length ? m.groups.map((g) => (getGroupLabel ? getGroupLabel(g) : g)).join(', ') : undefined
        };
      });
  }, [models, getGroupLabel, parents]);
  return (
    <ModelMultiSelect
      id={id}
      value={value}
      onChange={(ids) => onChange?.(ids)}
      options={options}
      canCreateValue={(typed) => isPattern(typed) && typed.trim().length > 1 && parents.every((p) => allowlistCovers(p, typed))}
    />
  );
}

ModelLimitSelector.propTypes = {
  id: PropTypes.string,
  models: PropTypes.arrayOf(PropTypes.object),
  value: PropTypes.arrayOf(PropTypes.string),
  onChange: PropTypes.func,
  getGroupLabel: PropTypes.func,
  parents: PropTypes.arrayOf(PropTypes.arrayOf(PropTypes.string))
};
