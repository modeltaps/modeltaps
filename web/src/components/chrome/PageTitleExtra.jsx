import { createContext, useContext } from 'react';
import { createPortal } from 'react-dom';
import PropTypes from 'prop-types';

// ==============================|| CHROME — PAGE TITLE EXTRA (inline title slot) ||============================== //
// Lets a routed page render controls inline to the RIGHT of the PageHeader title,
// left-aligned on the same row (e.g. the Token page's Base URL group), separate
// from the right-aligned action slot (PageActions). MainLayout owns the slot DOM
// node and shares it through this context; the slot host uses `display:contents`
// so an empty slot adds zero DOM/spacing to every other page.

export const PageTitleExtraContext = createContext(null);

export default function PageTitleExtra({ children }) {
  const slot = useContext(PageTitleExtraContext);
  if (!slot) return null;
  return createPortal(children, slot);
}

PageTitleExtra.propTypes = {
  children: PropTypes.node
};
