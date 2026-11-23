import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import i18nList from 'i18n/i18nList';
import useI18n from 'hooks/useI18n';
import { setAppLanguage } from 'utils/userSetting';
import Flag from './Flag';
import LanguageOption, { languageButtonClass, languageOptionClass } from './LanguageOption';

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
      <button type="button" onClick={() => setOpen(true)} aria-haspopup="dialog" aria-label={title} className={languageButtonClass}>
        <Flag country={current.countryCode} className="w-4" />
        {current.name}
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-sm" aria-labelledby="language-picker-title">
          <div className="flex items-center justify-between border-b border-border py-3 pl-5 pr-3">
            <h2 id="language-picker-title" className="text-base font-semibold">
              {title}
            </h2>
            <Button variant="ghost" size="icon" className="size-8" aria-label={t('common.close')} onClick={() => setOpen(false)}>
              <X className="size-4" />
            </Button>
          </div>
          <ul className="flex flex-col gap-1 p-2">
            {i18nList.map((item) => {
              const active = item.lng === current.lng;
              return (
                <li key={item.lng}>
                  <button
                    type="button"
                    aria-current={active ? 'true' : undefined}
                    onClick={() => pick(item.lng)}
                    className={languageOptionClass(active)}
                  >
                    <LanguageOption item={item} active={active} uiTag={current.bcp47} />
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
