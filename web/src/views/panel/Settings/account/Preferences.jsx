import { useTranslation } from 'react-i18next';

import useTheme from 'hooks/useTheme';
import i18nList from 'i18n/i18nList';
import { setAppLanguage } from 'utils/userSetting';
import { ActionCard } from '@/components/settings/cards';
import { SelectRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / APPEARANCE ||============================== //
// Theme and language, applied as soon as they are picked, so there is no save button (both persist
// themselves: the theme in localStorage and the account preferences, the language through
// setAppLanguage).

export default function AppearanceCard() {
  const { t, i18n } = useTranslation();
  const { theme, setTheme } = useTheme();
  const language = (i18nList.find((item) => item.lng === i18n.language) || i18nList[0]).lng;
  return (
    <ActionCard
      title={t('settingsUi.personal.cards.appearance.title')}
      description={t('settingsUi.personal.cards.appearance.description')}
      highlight="appearance"
    >
      <div className="grid gap-5 sm:grid-cols-2">
        <SelectRow
          id="pref-theme"
          label={t('theme.title', { defaultValue: 'Theme' })}
          description={t('settingsUi.personal.cards.appearance.themeHint')}
          value={theme}
          onChange={(v) => v !== theme && setTheme(v)}
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
          value={language}
          onChange={(v) => v !== language && setAppLanguage(v)}
          options={i18nList.map((item) => ({ value: item.lng, label: item.name }))}
        />
      </div>
    </ActionCard>
  );
}
