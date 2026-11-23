import { LANGUAGES } from './languages';

// The language pickers' list, from languages.json.
const i18nList = LANGUAGES.map((l) => ({ lng: l.code, name: l.name, countryCode: l.country, shortCode: l.short, bcp47: l.bcp47 }));

export default i18nList;
