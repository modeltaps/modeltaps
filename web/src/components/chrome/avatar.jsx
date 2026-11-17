import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| CHROME — USER AVATAR ||============================== //
// Fallback without an uploaded avatar: initials on a neutral grey, with a hairline ring.
// Shared by the sidebar account row and the public header so a person looks the same in both.

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
      className={cn(
        'inline-flex shrink-0 items-center justify-center rounded-full bg-muted font-medium text-muted-foreground ring-1 ring-inset ring-border',
        className,
        textClass
      )}
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
