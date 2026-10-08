import i18n from 'i18n/i18n';

// Component tests assert the Simplified Chinese copy they were written against. The app itself
// follows the browser language and falls back to English; pin the tests until they move to English.
await i18n.changeLanguage('zh_CN');
