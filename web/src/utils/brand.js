// ==============================|| BRAND NAME FALLBACK ||============================== //
// Use the build-time VITE_APP_NAME brand when the backend returns no system_name.

export function brandName(siteInfoSystemName) {
  return siteInfoSystemName || import.meta.env.VITE_APP_NAME || 'Modeltaps';
}
