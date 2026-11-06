import { useMemo } from 'react';
import PropTypes from 'prop-types';

import ModelMultiSelect from '@/components/model/ModelMultiSelect';

// ==============================|| TOKEN — MODEL LIMIT SELECTOR ||============================== //
// Model allowlist picker for tokens and organization members, on the shared ModelMultiSelect:
// vendor groups with logos, newest first, checkboxes, search. Stores the selected model ids.
// `models` are catalog rows (see toLimitModels); only listed models can be picked, ids already
// saved but no longer in the catalog stay visible so they can be removed.

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

export default function ModelLimitSelector({ id, models = [], value = [], onChange, getGroupLabel }) {
  const options = useMemo(
    () =>
      models.map((m) => {
        const owner = m.vendor?.name || m.owned_by || '';
        return {
          id: m.id,
          vendor: owner ? { key: m.vendor?.slug || owner.toLowerCase(), name: owner, icon: m.vendor?.icon } : undefined,
          releasedAt: m.released_at || undefined,
          meta: m.groups?.length ? m.groups.map((g) => (getGroupLabel ? getGroupLabel(g) : g)).join(', ') : undefined
        };
      }),
    [models, getGroupLabel]
  );
  return <ModelMultiSelect id={id} value={value} onChange={(ids) => onChange?.(ids)} options={options} allowCreate={false} />;
}

ModelLimitSelector.propTypes = {
  id: PropTypes.string,
  models: PropTypes.arrayOf(PropTypes.object),
  value: PropTypes.arrayOf(PropTypes.string),
  onChange: PropTypes.func,
  getGroupLabel: PropTypes.func
};
