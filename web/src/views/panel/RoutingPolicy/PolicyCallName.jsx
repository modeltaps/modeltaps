import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Copy } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { copy } from 'utils/common';

// The model name API keys use for a policy (policy/<name> for own policies), with a copy button.
// hoverCopy: on desktop the copy button shows when the surrounding `group` (a row or card) is
// hovered or focused; touch screens always show it.
export default function PolicyCallName({ name, className = '', hoverCopy = false }) {
  const { t } = useTranslation();
  return (
    <span className={`inline-flex min-w-0 items-center gap-1 ${className}`}>
      <span className="truncate font-mono text-sm">{name}</span>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className={`size-6 shrink-0 text-muted-foreground ${hoverCopy ? 'focus-visible:opacity-100 md:opacity-0 md:group-hover:opacity-100' : ''}`}
        aria-label={t('routingPolicy.copyCallName')}
        title={t('routingPolicy.copyCallName')}
        onClick={(e) => {
          e.stopPropagation();
          copy(name, t('routingPolicy.callName'));
        }}
      >
        <Copy className="size-3.5" />
      </Button>
    </span>
  );
}

PolicyCallName.propTypes = {
  name: PropTypes.string.isRequired,
  className: PropTypes.string,
  hoverCopy: PropTypes.bool
};
