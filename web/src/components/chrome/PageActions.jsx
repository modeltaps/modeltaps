import { createContext, useContext } from 'react';
import { createPortal } from 'react-dom';
import PropTypes from 'prop-types';

// ==============================|| CHROME — PAGE ACTIONS (header slot) ||============================== //
// Lets a routed page render its primary actions into the centralized PageHeader's
// action slot (title left, actions right) without each page re-implementing the
// title block. MainLayout owns the slot DOM node and shares it through this
// context; pages wrap their buttons in <PageActions> to portal them into it.

export const PageActionsContext = createContext(null);

export default function PageActions({ children }) {
  const slot = useContext(PageActionsContext);
  if (!slot) return null;
  return createPortal(children, slot);
}

PageActions.propTypes = {
  children: PropTypes.node
};
