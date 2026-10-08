import PropTypes from 'prop-types';

import BrandIcon from './BrandIcon';

// ==============================|| BRAND — MODEL ICON ||============================== //
// Renders the provider mark for a model name via the manifest model-prefix table (e.g.
// gpt-4o → OpenAI, claude-3 → Claude). Unknown models degrade to a first-letter placeholder.
export default function ModelIcon({ model, className }) {
  return <BrandIcon model={model} fallbackText={model} className={className} />;
}

ModelIcon.propTypes = {
  model: PropTypes.string,
  className: PropTypes.string
};
