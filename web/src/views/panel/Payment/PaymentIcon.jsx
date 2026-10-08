import PropTypes from 'prop-types';

import { sanitizeHtml } from 'utils/sanitize';

// Renders a payment gateway icon. The `icon` value is either raw inline SVG
// markup or an image URL. Tailwind/HTML port of the v1 `ui-component/PaymentIcon`.
export default function PaymentIcon({ icon, size = 24 }) {
  if (!icon) return null;

  const isSvgContent = icon.trim().startsWith('<svg');

  if (isSvgContent) {
    return (
      <span
        className="inline-flex items-center justify-center [&>svg]:size-full"
        style={{ width: size, height: size }}
        dangerouslySetInnerHTML={{ __html: sanitizeHtml(icon) }}
      />
    );
  }

  return <img src={icon} alt="payment icon" className="object-contain" style={{ width: size, height: size }} />;
}

PaymentIcon.propTypes = {
  icon: PropTypes.string,
  size: PropTypes.number
};
