import { Fragment } from 'react';
import PropTypes from 'prop-types';
import { MoreHorizontal } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator
} from '@/components/ui/dropdown-menu';

// Three-tier responsive toolbar degradation:
//   >= lg : every action is a full icon+label button (no "more" menu)
//   md-lg : secondary buttons drop the label (icon-only + tooltip); primary keeps it
//   < md  : secondary actions collapse into a "more" menu (full labels inside);
//           primary actions stay direct with their label
// Mark search/refresh/create-class actions with `primary` so they never degrade.
// `children` (optional) is rendered after the label and stays always visible
// (e.g. a selection count).
export default function ResponsiveToolbarButton({ icon: Icon, iconClassName, label, children, primary = false, ...props }) {
  if (primary) {
    return (
      <Button aria-label={typeof label === 'string' ? label : undefined} {...props}>
        <Icon className={cn('size-4', iconClassName)} />
        <span>{label}</span>
        {children}
      </Button>
    );
  }
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button aria-label={typeof label === 'string' ? label : undefined} {...props}>
            <Icon className={cn('size-4', iconClassName)} />
            <span className="hidden lg:inline">{label}</span>
            {children}
          </Button>
        </TooltipTrigger>
        <TooltipContent className="lg:hidden">{label}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

ResponsiveToolbarButton.propTypes = {
  icon: PropTypes.elementType.isRequired,
  iconClassName: PropTypes.string,
  label: PropTypes.node,
  children: PropTypes.node,
  primary: PropTypes.bool
};

// Group of secondary toolbar actions. At >= md each action renders as its own
// ResponsiveToolbarButton (icon-only between md and lg); below md the buttons
// are hidden and the same actions collapse into a single "more" dropdown whose
// items keep the full label. onClick/disabled are shared between both renderings.
export function ResponsiveToolbarActions({ actions, size, align = 'end' }) {
  const { t } = useTranslation();
  return (
    <>
      {actions.map((action) => (
        <ResponsiveToolbarButton
          key={action.key}
          variant={action.variant ?? 'outline'}
          size={size}
          icon={action.icon}
          iconClassName={action.iconClassName}
          label={action.label}
          onClick={action.onClick}
          disabled={action.disabled}
          className={cn('max-md:hidden', action.buttonClassName)}
        />
      ))}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size={size} className="md:hidden" aria-label={t('common.more')}>
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align={align}>
          {actions.map((action) => (
            <Fragment key={action.key}>
              {action.separatorBefore && <DropdownMenuSeparator />}
              <DropdownMenuItem disabled={action.disabled} className={action.itemClassName} onClick={action.onClick}>
                <action.icon /> {action.label}
              </DropdownMenuItem>
            </Fragment>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );
}

ResponsiveToolbarActions.propTypes = {
  actions: PropTypes.arrayOf(
    PropTypes.shape({
      key: PropTypes.string.isRequired,
      icon: PropTypes.elementType.isRequired,
      iconClassName: PropTypes.string,
      label: PropTypes.node,
      onClick: PropTypes.func,
      disabled: PropTypes.bool,
      variant: PropTypes.string,
      buttonClassName: PropTypes.string,
      itemClassName: PropTypes.string,
      separatorBefore: PropTypes.bool
    })
  ).isRequired,
  size: PropTypes.string,
  align: PropTypes.string
};
