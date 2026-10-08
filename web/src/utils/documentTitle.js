// ==============================|| DOCUMENT TITLE (SINGLE WRITER) ||==============================
// The tab title is composed from two independently sourced parts: the plane
// prefix (MainLayout) and the brand (site status). Both writers go through the
// setters below so whichever resolves last keeps the other part intact.

import { brandName } from './brand';

const state = { prefix: null, brand: brandName() };

function render() {
  document.title = state.prefix ? `${state.prefix} · ${state.brand}` : state.brand;
}

export function setDocumentTitlePrefix(prefix) {
  state.prefix = prefix || null;
  render();
}

export function setDocumentTitleBrand(brand) {
  state.brand = brand;
  render();
}
