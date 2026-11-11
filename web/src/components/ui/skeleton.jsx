import { cn } from '@/lib/utils';

// Placeholder block for content that is still loading. A soft pulse says "loading" without
// drawing the eye; size it like the content it stands for so nothing jumps when it arrives.
function Skeleton({ className, ...props }) {
  return <div aria-hidden="true" className={cn('animate-pulse rounded-md bg-muted', className)} {...props} />;
}

export { Skeleton };
