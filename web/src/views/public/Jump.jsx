import { useEffect } from 'react';
import { useLocation } from 'react-router';
import { useTranslation } from 'react-i18next';

// ==============================|| JUMP (deep-link redirect) ||============================== //

export default function Jump() {
  const { t } = useTranslation();
  const location = useLocation();

  useEffect(() => {
    const params = new URLSearchParams(location.search);
    const jump = params.get('url');
    const allowedUrls = ['opencat://', 'ama://'];
    if (jump && allowedUrls.some((url) => jump.startsWith(url))) {
      window.location.href = jump;
    }
  }, [location]);

  return <div className="px-4 py-10 text-center text-sm text-muted-foreground">{t('jump')}</div>;
}
