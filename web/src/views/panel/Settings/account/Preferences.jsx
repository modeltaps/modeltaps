import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import useTheme from 'hooks/useTheme';
import i18nList from 'i18n/i18nList';
import { setAppLanguage } from 'utils/userSetting';
import { FormCard } from '@/components/settings/cards';
import { SelectRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / APPEARANCE ||============================== //
// Theme and language, applied when the card is saved (both persist themselves: the theme in
// localStorage and the account preferences, the language through setAppLanguage).

export default function AppearanceCard() {
  const { t, i18n } = useTranslation();
  const { theme, setTheme } = useTheme();
  const language = (i18nList.find((item) => item.lng === i18n.language) || i18nList[0]).lng;
  const [draft, setDraft] = useState({ theme, language });

  useEffect(() => setDraft({ theme, language }), [theme, language]);

  const dirty = draft.theme !== theme || draft.language !== language;

  const save = () => {
    if (draft.theme !== theme) setTheme(draft.theme);
    if (draft.language !== language) setAppLanguage(draft.language);
  };

  return (
    <FormCard
      title={t('settingsUi.personal.cards.appearance.title')}
      description={t('settingsUi.personal.cards.appearance.description')}
      hint={t('settingsUi.personal.cards.appearance.hint')}
      highlight="appearance"
      dirty={dirty}
      onSave={save}
      onReset={() => setDraft({ theme, language })}
    >
      <div className="grid gap-5 sm:grid-cols-2">
        <SelectRow
          id="pref-theme"
          label={t('theme.title', { defaultValue: 'Theme' })}
          description={t('settingsPage.preferences.themeHint')}
          value={draft.theme}
          onChange={(v) => setDraft((d) => ({ ...d, theme: v }))}
          options={[
            { value: 'system', label: t('theme.auto', { defaultValue: 'System' }) },
            { value: 'light', label: t('theme.light', { defaultValue: 'Light' }) },
            { value: 'dark', label: t('theme.dark', { defaultValue: 'Dark' }) }
          ]}
        />
        <SelectRow
          id="pref-language"
          label={t('language.select', { defaultValue: 'Language' })}
          description={t('settingsUi.personal.cards.appearance.languageHint')}
          value={draft.language}
          onChange={(v) => setDraft((d) => ({ ...d, language: v }))}
          options={i18nList.map((item) => ({ value: item.lng, label: item.name }))}
        />
      </div>
    </FormCard>
  );
}
