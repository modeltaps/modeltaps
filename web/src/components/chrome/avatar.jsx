import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| CHROME — USER AVATAR ||============================== //
// 头像回退：无上传头像时用名字首字母 + 按名字哈希出的固定色相(ChatGPT 式)。
// 侧栏顶部账号行与公开页头部共用，保证同一个人在两处长得一样。

export const accountDisplayName = (user) => user?.display_name || user?.email || user?.phone_number || user?.username || 'User';

export const initialsOf = (name) => {
  const parts = (name || '').trim().split(/\s+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return ((name || 'U').trim().slice(0, 2) || 'U').toUpperCase();
};

export const hueOf = (s) => {
  let h = 0;
  for (let i = 0; i < (s || '').length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return h % 360;
};

export default function UserAvatar({ user, className, textClass = 'text-xs' }) {
  const name = accountDisplayName(user);
  if (user?.avatar_url) {
    return (
      <span className={cn('inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted', className)}>
        <img src={user.avatar_url} alt={name} className="size-full object-cover" />
      </span>
    );
  }
  return (
    <span
      className={cn('inline-flex shrink-0 items-center justify-center rounded-full font-semibold text-white', className, textClass)}
      style={{ backgroundColor: `hsl(${hueOf(name)} 55% 42%)` }}
    >
      {initialsOf(name)}
    </span>
  );
}

UserAvatar.propTypes = {
  user: PropTypes.object,
  className: PropTypes.string,
  textClass: PropTypes.string
};
