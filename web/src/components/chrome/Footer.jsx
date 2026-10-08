import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { LogoMark } from './Logo';
import { brandName } from 'utils/brand';
import { sanitizeHtml } from 'utils/sanitize';

// ==============================|| CHROME — FOOTER ||============================== //
// Matches the v1 auth footer: [R-mark] Modeltaps © 2026. Honors custom footer_html.

export default function Footer() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);

  if (siteInfo.footer_html) {
    return (
      <footer className="flex items-center justify-center px-4 py-5">
        <div
          className="custom-footer text-center text-sm text-muted-foreground"
          dangerouslySetInnerHTML={{ __html: sanitizeHtml(siteInfo.footer_html) }}
        />
      </footer>
    );
  }

  return (
    <footer className="flex items-center justify-center gap-1.5 px-4 py-5 text-muted-foreground">
      <LogoMark className="h-3 w-auto shrink-0" />
      <span className="text-[0.8125rem] font-medium leading-none">{brandName(siteInfo.system_name)}</span>
      <span className="text-[0.8125rem] leading-none">© 2026</span>
      {siteInfo.docs_link && (
        <a
          href={siteInfo.docs_link}
          target="_blank"
          rel="noopener noreferrer"
          className="text-[0.8125rem] leading-none hover:text-foreground hover:underline"
        >
          {t('common.docs')}
        </a>
      )}
    </footer>
  );
}
