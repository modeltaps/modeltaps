import { Globe, LogIn, CreditCard, Route, ShieldCheck, Plug, Landmark } from 'lucide-react';

// ==============================|| SYSTEM SETTINGS — THEME METADATA ||============================== //
// The seven system-settings themes as plain metadata (no page components), so the
// admin sidebar can list them without pulling the settings page into its chunk.
// `Setting/index.jsx` attaches the page component for each id; the sidebar reads
// `icon` / `labelKey` from here. Single source of truth for ids and order.

export const THEME_META = [
  { id: 'site', icon: Globe, labelKey: 'setting_index.sectionNav.site', fallback: 'Site' },
  { id: 'auth', icon: LogIn, labelKey: 'setting_index.sectionNav.auth', fallback: 'Login & Registration' },
  { id: 'billing', icon: CreditCard, labelKey: 'setting_index.sectionNav.billing', fallback: 'Billing & Payments' },
  { id: 'relay', icon: Route, labelKey: 'setting_index.sectionNav.relay', fallback: 'Gateway Behavior' },
  { id: 'privacy', icon: ShieldCheck, labelKey: 'setting_index.sectionNav.privacy', fallback: 'Logs & Privacy' },
  { id: 'integrations', icon: Plug, labelKey: 'setting_index.sectionNav.integrations', fallback: 'Integrations' },
  {
    id: 'organization',
    icon: Landmark,
    labelKey: 'setting_index.sectionNav.organization',
    fallback: 'Organization Policy',
    rootOnly: true
  }
];
