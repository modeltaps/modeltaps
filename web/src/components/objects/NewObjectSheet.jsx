import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { ObjectSheet, SheetBar } from './ObjectSheet';

// Right-side sheet that creates an object. It asks only for what the object cannot do without;
// everything else starts from defaults and is changed later in the object's panel. The footer
// says what is still missing (or what happens on create) and holds Cancel / Create. Closing
// with something typed asks first.
export default function NewObjectSheet({
  titleId,
  size = 'sm',
  icon,
  title,
  subtitle,
  touched,
  status,
  ready = true,
  busy,
  createLabel,
  onCreate,
  onClose,
  children
}) {
  const { t } = useTranslation();
  const requestClose = () => {
    if (!touched || window.confirm(t('objects.new.discardConfirm'))) onClose();
  };
  return (
    <ObjectSheet
      titleId={titleId}
      size={size}
      icon={icon}
      title={title}
      subtitle={subtitle}
      onClose={requestClose}
      bodyClassName="[&>form>*:first-child]:pt-0"
      footer={
        <SheetBar status={status}>
          <Button variant="outline" className="rounded-lg" onClick={requestClose}>
            {t('common.cancel')}
          </Button>
          <Button className="rounded-lg" disabled={!ready || busy} onClick={onCreate}>
            {busy && <Loader2 className="animate-spin" />}
            {createLabel || t('objects.new.create')}
          </Button>
        </SheetBar>
      }
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (ready && !busy) onCreate();
        }}
      >
        {children}
        <button type="submit" hidden aria-hidden="true" tabIndex={-1} />
      </form>
    </ObjectSheet>
  );
}

NewObjectSheet.propTypes = {
  titleId: PropTypes.string.isRequired,
  size: PropTypes.oneOf(['lg', 'md', 'sm']),
  icon: PropTypes.node,
  title: PropTypes.node,
  subtitle: PropTypes.node,
  touched: PropTypes.bool,
  status: PropTypes.node,
  ready: PropTypes.bool,
  busy: PropTypes.bool,
  createLabel: PropTypes.node,
  onCreate: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  children: PropTypes.node
};
