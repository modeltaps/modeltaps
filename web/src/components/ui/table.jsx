import * as React from 'react';

import { cn } from '@/lib/utils';

const Table = React.forwardRef(({ className, ...props }, ref) => (
  <div className="relative w-full overflow-x-auto">
    <table ref={ref} className={cn('w-full caption-bottom text-sm', className)} {...props} />
  </div>
));
Table.displayName = 'Table';

const TableHeader = React.forwardRef(({ className, ...props }, ref) => (
  <thead ref={ref} className={cn('[&_tr]:border-b [&_tr]:border-border', className)} {...props} />
));
TableHeader.displayName = 'TableHeader';

const TableBody = React.forwardRef(({ className, ...props }, ref) => (
  <tbody ref={ref} className={cn('[&_tr:last-child]:border-0', className)} {...props} />
));
TableBody.displayName = 'TableBody';

const TableRow = React.forwardRef(({ className, ...props }, ref) => (
  <tr ref={ref} className={cn('border-b border-border transition-colors hover:bg-muted/50', className)} {...props} />
));
TableRow.displayName = 'TableRow';

const TableHead = React.forwardRef(({ className, ...props }, ref) => (
  <th
    ref={ref}
    className={cn(
      'h-11 whitespace-nowrap px-3 text-left align-middle text-xs font-medium text-muted-foreground [&:has([role=checkbox])]:pr-0',
      className
    )}
    {...props}
  />
));
TableHead.displayName = 'TableHead';

const TableCell = React.forwardRef(({ className, ...props }, ref) => (
  <td ref={ref} className={cn('whitespace-nowrap px-3 py-3 align-middle [&:has([role=checkbox])]:pr-0', className)} {...props} />
));
TableCell.displayName = 'TableCell';

const TableCaption = React.forwardRef(({ className, ...props }, ref) => (
  <caption ref={ref} className={cn('mt-4 text-sm text-muted-foreground', className)} {...props} />
));
TableCaption.displayName = 'TableCaption';

// Class for the one column that should take a table's spare width (usually the name): the other
// columns shrink to their content, and this one truncates inside it instead of pushing the table
// wider. Its content needs `min-w-0` / `truncate` to give way.
const GROW_COLUMN = 'w-full max-w-0 min-w-[14rem]';

export { GROW_COLUMN, Table, TableHeader, TableBody, TableRow, TableHead, TableCell, TableCaption };
