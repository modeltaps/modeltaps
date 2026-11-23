import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Check, Globe, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import i18nList from 'i18n/i18nList';
import useI18n from 'hooks/useI18n';
import { setAppLanguage } from 'utils/userSetting';

// ==============================|| CHROME — FOOTER LANGUAGE PICKER ||============================== //
// The public pages' language choice: a quiet button at the right end of the footer showing the
// current language, opening a dialog with every language. The static home page renders the same
// thing without React (static-home/StaticHome.jsx).

export default function LanguagePicker() {
  const i18n = useI18n();
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  const current = i18nList.find((item) => item.lng === i18n.language) || i18nList[0];
  const title = t('language.select', { defaultValue: 'Language' });

  const pick = (lng) => {
    setAppLanguage(lng);
    setOpen(false);
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
        aria-label={title}
        className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-[0.8125rem] leading-none text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
      >
        <Globe className="size-4" />
        {current.name}
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-xs" aria-labelledby="language-picker-title">
          <div className="flex items-center justify-between px-5 pb-2 pt-4">
            <h2 id="language-picker-title" className="text-base font-semibold">
              {title}
            </h2>
            <Button variant="ghost" size="icon" className="size-8" aria-label={t('common.close')} onClick={() => setOpen(false)}>
              <X className="size-4" />
            </Button>
          </div>
          <ul className="flex flex-col gap-0.5 px-2 pb-3">
            {i18nList.map((item) => {
              const active = item.lng === current.lng;
              return (
                <li key={item.lng}>
                  <button
                    type="button"
                    lang={item.lng.replace('_', '-')}
                    aria-current={active ? 'true' : undefined}
                    onClick={() => pick(item.lng)}
                    className={cn(
                      'flex w-full items-center justify-between rounded-lg px-3 py-2.5 text-left text-sm transition-colors hover:bg-muted',
                      active && 'font-medium'
                    )}
                  >
                    {item.name}
                    {active && <Check className="size-4" />}
                  </button>
                </li>
              );
            })}
          </ul>
        </DialogContent>
      </Dialog>
    </>
  );
}
