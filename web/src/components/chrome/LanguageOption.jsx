import PropTypes from 'prop-types';
import { Check } from 'lucide-react';

import { cn } from '@/lib/utils';
import Flag from './Flag';

// ==============================|| CHROME — LANGUAGE OPTION ||============================== //
// One row of the language dialog, shared by the footer picker and the static home page: the
// country flag, the language's own name and, under it, its name in the page's language (from
// Intl, so no strings to translate), then a check on the current one.

// The footer button that opens the dialog: the current language's flag and name.
export const languageButtonClass =
  'inline-flex shrink-0 cursor-pointer items-center gap-2 whitespace-nowrap rounded-md px-2 py-1.5 text-[0.8125rem] leading-none text-muted-foreground transition-colors hover:bg-muted hover:text-foreground';

export const languageOptionClass = (active) =>
  cn(
    'flex min-h-13 w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition-colors hover:bg-muted focus-visible:bg-muted focus-visible:outline-none',
    active && 'bg-muted'
  );

function nameIn(uiTag, tag) {
  try {
    return new Intl.DisplayNames([uiTag], { type: 'language', languageDisplay: 'standard' }).of(tag) || '';
  } catch {
    return '';
  }
}

export default function LanguageOption({ item, active, uiTag }) {
  // Chinese is offered as plain 中文, so its translated name drops the script too.
  const translated = nameIn(uiTag, item.bcp47.split('-')[0]);
  return (
    <>
      <Flag country={item.countryCode} className="w-6" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span lang={item.bcp47} className={cn('truncate text-sm leading-5', active && 'font-medium')}>
          {item.name}
        </span>
        {translated && translated !== item.name && <span className="truncate text-xs leading-4 text-muted-foreground">{translated}</span>}
      </span>
      <Check className={cn('size-4 shrink-0', !active && 'invisible')} aria-hidden="true" />
    </>
  );
}

LanguageOption.propTypes = {
  item: PropTypes.shape({ lng: PropTypes.string, name: PropTypes.string, countryCode: PropTypes.string, bcp47: PropTypes.string })
    .isRequired,
  active: PropTypes.bool,
  uiTag: PropTypes.string.isRequired
};
