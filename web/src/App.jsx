import { useEffect } from 'react';
import { useDispatch } from 'react-redux';

import { SET_THEME } from 'store/actions';
import { I18nextProvider } from 'react-i18next';
// routing
import Routes from 'routes';

// project imports
import NavigationScroll from 'layout/NavigationScroll';

// auth
import UserProvider from 'contexts/UserContext';
import StatusProvider from 'contexts/StatusContext';
import OrgProvider from 'contexts/OrgContext';
import { NoticeProvider, NoticeDialogs } from 'ui-component/notice';
import { Toaster } from '@/components/ui/sonner';

// locales
import i18n from 'i18n/i18n';

// ==============================|| APP ||============================== //

// Apply the shadcn `.dark`/`.light` class on <html> so styling is correct on
// every route from boot (public routes don't go through a layout otherwise).
const applyThemeClass = (theme) => {
  const root = document.documentElement;
  root.classList.toggle('dark', theme === 'dark');
  root.classList.toggle('light', theme !== 'dark');
};

const App = () => {
  const dispatch = useDispatch();

  useEffect(() => {
    const storedTheme = localStorage.getItem('theme');
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

    if (storedTheme) {
      dispatch({ type: SET_THEME, theme: storedTheme });
      applyThemeClass(storedTheme);
    } else {
      const systemTheme = mediaQuery.matches ? 'dark' : 'light';
      dispatch({ type: SET_THEME, theme: systemTheme });
      applyThemeClass(systemTheme);
    }
    const handleThemeChange = (e) => {
      const storedTheme = localStorage.getItem('theme');
      if (!storedTheme) {
        const systemTheme = e.matches ? 'dark' : 'light';
        dispatch({ type: SET_THEME, theme: systemTheme });
        applyThemeClass(systemTheme);
      }
    };

    mediaQuery.addEventListener('change', handleThemeChange);

    return () => {
      mediaQuery.removeEventListener('change', handleThemeChange);
    };
  }, [dispatch]);

  return (
    <NavigationScroll>
      <StatusProvider>
        <I18nextProvider i18n={i18n}>
          <NoticeProvider>
            <UserProvider>
              <OrgProvider>
                <Routes />
                <NoticeDialogs />
                <Toaster position="top-right" />
              </OrgProvider>
            </UserProvider>
          </NoticeProvider>
        </I18nextProvider>
      </StatusProvider>
    </NavigationScroll>
  );
};

export default App;
