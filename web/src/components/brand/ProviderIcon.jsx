import PropTypes from 'prop-types';

import BrandIcon from './BrandIcon';
import { resolveBrandFromText, resolveChannelBrand } from './brandIcons';

// ==============================|| BRAND — PROVIDER / CHANNEL ICON ||============================== //
// Renders the provider mark for a channel via BrandIcon: (1) a mapped brand icon resolved from
// the numeric channel type id (authoritative) or brand tokens in the display name, then the
// manifest's channel type / base_url domain tables; (2) the backend-cached favicon of the
// base_url domain (/api/brand-icon/domain/{host}) for generic OpenAI-compatible channels;
// (3) a first-letter placeholder. Exported as both ProviderIcon (default) and ChannelIcon.
export default function ProviderIcon({ type, name, baseUrl, className }) {
  const brandKey = resolveChannelBrand(type) || resolveBrandFromText(name);
  return <BrandIcon brandKey={brandKey} channelType={type} baseUrl={baseUrl} fallbackText={name} className={className} />;
}

ProviderIcon.propTypes = {
  type: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  name: PropTypes.string,
  baseUrl: PropTypes.string,
  className: PropTypes.string
};

export const ChannelIcon = ProviderIcon;
