import { useTranslation } from 'react-i18next';

// The i18n instance for components that render the language picker. Persisting a choice is done
// explicitly by setAppLanguage, so a detected browser language is never stored as a preference.
const useI18n = () => useTranslation().i18n;

export default useI18n;
