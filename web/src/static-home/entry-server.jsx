import { renderToStaticMarkup } from 'react-dom/server';
import { createInstance } from 'i18next';
import { I18nextProvider, initReactI18next } from 'react-i18next';
import { Provider } from 'react-redux';
import { legacy_createStore as createStore } from 'redux';
import { MemoryRouter } from 'react-router';

import { resources } from 'i18n/resources';
import i18nList from 'i18n/i18nList';
import StaticHome from './StaticHome';
import { STATIC_TOKENS } from './tokens';

// Build-time entry (vite SSR build, driven by scripts/prerender-home.mjs): renders the static home
// page for one language and returns the markup plus the strings the document head needs.

export const LANGUAGES = i18nList.map((item) => item.lng);
export { STATIC_TOKENS };

export function renderHome(lng) {
  const i18n = createInstance();
  i18n.use(initReactI18next).init({
    resources,
    lng,
    fallbackLng: 'en_US',
    initAsync: false,
    interpolation: { escapeValue: false }
  });

  // The components read the site name from the store; the server substitutes the token.
  const store = createStore(() => ({ siteInfo: { system_name: STATIC_TOKENS.brand }, account: {} }));

  const html = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <Provider store={store}>
        <MemoryRouter>
          <StaticHome lng={lng} />
        </MemoryRouter>
      </Provider>
    </I18nextProvider>
  );

  const t = i18n.getFixedT(lng);
  return {
    html,
    tagline: t('home.hero.tagline'),
    catchphrase: t('home.hero.catchphrase')
  };
}
