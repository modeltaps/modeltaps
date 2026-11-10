import { Database, KeyRound, ShieldCheck, SlidersHorizontal, User, Users, Wallet } from 'lucide-react';

import MembersTab from '../Organization/MembersTab';
import GeneralSection from './org/GeneralSection';
import BudgetSection from './org/BudgetSection';

// ==============================|| PANEL — SETTINGS SECTION REGISTRY ||============================== //
// Single source of truth for the two settings groups: organization sections register their
// content component here; personal account sections are dispatched by `./account` by id.
// Both render with the in-page subnav of Settings/index.jsx.

export const ORG_SECTIONS = [
  { id: 'general', labelKey: 'settingsPage.sections.general', fallback: 'General', icon: SlidersHorizontal, component: GeneralSection },
  { id: 'members', labelKey: 'settingsPage.sections.members', fallback: 'Members', icon: Users, component: MembersTab },
  { id: 'budget', labelKey: 'settingsPage.sections.budget', fallback: 'Budget', icon: Wallet, component: BudgetSection }
];

export const ACCOUNT_SECTIONS = [
  { id: 'profile', labelKey: 'settingsPage.sections.profile', fallback: 'Profile', icon: User },
  { id: 'security', labelKey: 'settingsPage.sections.security', fallback: 'Sign-in & security', icon: ShieldCheck },
  { id: 'tokens', labelKey: 'settingsPage.sections.tokens', fallback: 'API Tokens', icon: KeyRound },
  { id: 'privacy', labelKey: 'settingsPage.sections.privacy', fallback: 'Data & privacy', icon: Database },
  { id: 'preferences', labelKey: 'settingsPage.sections.preferences', fallback: 'Preferences', icon: SlidersHorizontal }
];

export const DEFAULT_ACCOUNT_SECTION = ACCOUNT_SECTIONS[0].id;

export const orgSectionIds = ORG_SECTIONS.map(({ id }) => id);
export const accountSectionIds = ACCOUNT_SECTIONS.map(({ id }) => id);

export const orgSettingsUrl = (orgId, section) => `/panel/settings/org/${orgId}/${section}`;
export const accountSettingsUrl = (section) => `/panel/settings/account/${section}`;
