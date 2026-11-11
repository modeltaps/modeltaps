// ==============================|| BRAND NAME FALLBACK ||============================== //
// Use the build-time VITE_APP_NAME brand when the backend returns no system_name.

const DEFAULT_BRAND = 'Modeltaps';

export function brandName(siteInfoSystemName) {
  return siteInfoSystemName || import.meta.env.VITE_APP_NAME || DEFAULT_BRAND;
}

// The wordmark set next to the logo mark. The default brand is written all lowercase there to match
// the rounded drop mark; running text (titles, messages, docs) keeps "Modeltaps". A custom
// system_name is shown exactly as the operator typed it.
export function brandWordmark(siteInfoSystemName) {
  const name = brandName(siteInfoSystemName);
  return name === DEFAULT_BRAND ? name.toLowerCase() : name;
}

// The bundled wordmark face only has the letters of the default brand, so a custom name is set in
// the UI font instead of mixing the two letter by letter.
export function isDefaultBrand(siteInfoSystemName) {
  return brandName(siteInfoSystemName) === DEFAULT_BRAND;
}
