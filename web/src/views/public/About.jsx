import { useEffect, useState, useCallback } from 'react';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { API } from 'utils/api';
import { showError } from 'utils/common';
import { sanitizeHtml } from 'utils/sanitize';
import { Card } from '@/components/ui/card';
import 'assets/css/content-viewer.css';

// ==============================|| ABOUT (shadcn) ||============================== //

export default function About() {
  const { t } = useTranslation();
  const [about, setAbout] = useState('');
  const [aboutLoaded, setAboutLoaded] = useState(false);

  const displayAbout = useCallback(async () => {
    setAbout(localStorage.getItem('about') || '');
    try {
      const res = await API.get('/api/about');
      const { success, message, data } = res.data;
      if (success) {
        setAbout(data);
        localStorage.setItem('about', data);
      } else {
        showError(message);
        setAbout(t('about.loadingError'));
      }
    } catch (error) {
      setAbout(t('about.loadingError'));
    }

    setAboutLoaded(true);
  }, [t]);

  useEffect(() => {
    displayAbout();
  }, [displayAbout]);

  if (!aboutLoaded) {
    return (
      <div className="flex min-h-[calc(100vh-8rem)] items-center justify-center">
        <Loader2 className="size-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (about === '') {
    return (
      <div className="mx-auto max-w-3xl px-4 py-10">
        <Card className="p-6">
          <h1 className="mb-4 text-xl font-semibold">{t('about.aboutTitle')}</h1>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {t('about.aboutDescription')} <br />
            {t('about.projectRepo')}
            <a className="text-primary-text underline" href="https://github.com/modeltaps/modeltaps">
              https://github.com/modeltaps/modeltaps
            </a>
          </p>
        </Card>
      </div>
    );
  }

  if (about === t('about.loadingError')) {
    return (
      <div className="flex min-h-[calc(100vh-8rem)] items-center justify-center px-4">
        <p className="text-sm text-destructive">{t('about.loadingError')}</p>
      </div>
    );
  }

  const isUrl = about.startsWith('http://') || about.startsWith('https://');
  if (isUrl) {
    return <iframe title="content-frame" src={about} className="h-[calc(100vh-8rem)] w-full border-0" />;
  }

  const isHtml = about.trim().startsWith('<') && about.includes('</');
  const html = sanitizeHtml(isHtml ? about : marked.parse(about));

  return (
    <div className="mx-auto max-w-3xl px-4 py-10">
      <div
        className="content-viewer text-base leading-relaxed [&_img]:h-auto [&_img]:max-w-full"
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </div>
  );
}
