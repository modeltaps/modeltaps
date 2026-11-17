import { createContext, useContext } from 'react';
import { createPortal } from 'react-dom';
import PropTypes from 'prop-types';

// ==============================|| CHROME — PAGE TITLE CENTER (centered title-row slot) ||============================== //
// Lets a routed page put a view switch in the middle of the PageHeader title row (title left,
// actions right), e.g. the playground and API docs modality switches. On narrow screens the slot
// drops to its own full-width row under the title. MainLayout owns the slot DOM node and shares
// it through this context; while the slot is empty the header keeps its normal layout.

export const PageTitleCenterContext = createContext(null);

export default function PageTitleCenter({ children }) {
  const slot = useContext(PageTitleCenterContext);
  if (!slot) return null;
  return createPortal(children, slot);
}

PageTitleCenter.propTypes = {
  children: PropTypes.node
};
