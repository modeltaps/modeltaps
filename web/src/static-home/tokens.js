// Placeholders the prerendered home page carries for values only the server knows. The Go side
// (router/home.go) replaces them per request; keep both lists in sync.
export const STATIC_TOKENS = {
  // site name for running text, HTML-escaped by the server
  brand: '__MT_BRAND__',
  // site name next to the logo mark (the default brand is lowercased there)
  wordmark: '__MT_WORDMARK__',
  // absolute site origin for canonical / hreflang links
  origin: '__MT_ORIGIN__',
  // docs link URL; the server drops everything between the docs markers when none is configured
  docsUrl: '__MT_DOCS_URL__',
  docsStart: '<!--mt:docs-->',
  docsEnd: '<!--/mt:docs-->'
};
