import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { marked } from 'marked';
import { Loader2 } from 'lucide-react';

import { API } from 'utils/api';
import { showError } from 'utils/common';
import { sanitizeHtml } from 'utils/sanitize';
import Hero from './home/Hero';
import 'assets/css/content-viewer.css';

// ==============================|| PUBLIC HOME (shadcn) ||============================== //
// Landing at `/`: a single screen with the headline, one action and the gateway animation;
// shown to visitors whether signed in or not.
// When HomePageContent is set in the admin settings, it replaces the marketing page:
// URL → full-page iframe; otherwise Markdown/HTML rendered via the content viewer.

export default function Home() {
  const { t } = useTranslation();
  const [homePageContent, setHomePageContent] = useState('');
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);

  const displayHomePageContent = useCallback(async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    try {
      const res = await API.get('/api/home_page_content');
      const { success, message, data } = res.data;
      if (success) {
        setHomePageContent(data);
        localStorage.setItem('home_page_content', data);
      } else {
        showError(message);
        setHomePageContent(t('home.loadingErr'));
      }
    } catch (error) {
      setHomePageContent(t('home.loadingErr'));
    }

    setHomePageContentLoaded(true);
  }, [t]);

  useEffect(() => {
    displayHomePageContent();
  }, [displayHomePageContent]);

  if (!homePageContentLoaded) {
    return (
      <div className="flex min-h-[calc(100vh-8rem)] items-center justify-center">
        <Loader2 className="size-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (homePageContent === t('home.loadingErr')) {
    return (
      <div className="flex min-h-[calc(100vh-8rem)] items-center justify-center px-4">
        <p className="text-sm text-destructive">{t('home.loadingErr')}</p>
      </div>
    );
  }

  if (homePageContent !== '') {
    const isUrl = homePageContent.startsWith('http://') || homePageContent.startsWith('https://');
    if (isUrl) {
      return <iframe title="content-frame" src={homePageContent} className="h-[calc(100vh-8rem)] w-full border-0" />;
    }

    const isHtml = homePageContent.trim().startsWith('<') && homePageContent.includes('</');
    const html = sanitizeHtml(isHtml ? homePageContent : marked.parse(homePageContent));

    return (
      <div className="mx-auto max-w-3xl px-4 py-10">
        <div
          className="content-viewer text-base leading-relaxed [&_img]:h-auto [&_img]:max-w-full"
          dangerouslySetInnerHTML={{ __html: html }}
        />
      </div>
    );
  }

  return (
    <div className="bg-background">
      <Hero />
    </div>
  );
}
