import { Check, Globe } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import i18nList from 'i18n/i18nList';
import useI18n from 'hooks/useI18n';
import { setAppLanguage } from 'utils/userSetting';
import { useTranslation } from 'react-i18next';

// ==============================|| AUTH — i18n LANGUAGE SWITCHER ||============================== //

export default function LanguageSwitcher() {
  const i18n = useI18n();
  const { t } = useTranslation();

  const currentLang = i18n.language || 'zh_CN';
  const current = i18nList.find((item) => item.lng === currentLang) || i18nList[0];
  const ariaLabel = t('language.select', { defaultValue: 'Select language' });

  // The shared DropdownMenu already closes on Esc/outside click; arrow keys
  // move focus between items here so the menu is fully keyboard operable.
  const onMenuKeyDown = (e) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    const items = Array.from(e.currentTarget.querySelectorAll('[role="menuitem"]'));
    if (items.length === 0) return;
    const idx = items.indexOf(document.activeElement);
    const next = e.key === 'ArrowDown' ? (idx + 1) % items.length : (idx - 1 + items.length) % items.length;
    items[next].focus();
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" aria-label={ariaLabel} className="gap-1.5 px-2">
          <Globe className="size-5" />
          <span className="text-sm font-medium uppercase">{current.shortCode}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-40" onKeyDown={onMenuKeyDown}>
        {i18nList.map((item) => {
          const isActive = item.lng === currentLang;
          return (
            <DropdownMenuItem key={item.lng} onClick={() => setAppLanguage(item.lng)} className={cn(isActive && 'font-semibold')}>
              <span className="flex w-4 justify-center text-foreground">{isActive && <Check className="size-4" />}</span>
              {item.name}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
