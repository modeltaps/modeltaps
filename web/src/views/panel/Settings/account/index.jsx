import PropTypes from 'prop-types';

import useHighlightTarget from 'hooks/useHighlightTarget';
import Preferences from './Preferences';
import Privacy from './Privacy';
import Profile from './Profile';
import Security from './Security';
import Tokens from './Tokens';

// ==============================|| SETTINGS — ACCOUNT GROUP ROUTER ||============================== //
// 「个人账号」分组的 section 分派;section id 由 Settings/sections.js 的 ACCOUNT_SECTIONS 定义。

const COMPONENTS = {
  profile: Profile,
  security: Security,
  tokens: Tokens,
  privacy: Privacy,
  preferences: Preferences
};

export default function AccountSection({ section }) {
  // ?highlight=log-io 等深链高亮在分组层挂一次即可,覆盖所有 account section。
  useHighlightTarget();
  const Component = COMPONENTS[section];
  return Component ? <Component /> : null;
}

AccountSection.propTypes = { section: PropTypes.string.isRequired };
