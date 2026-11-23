import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { BrandLockup } from './Logo';
import LanguagePicker from './LanguagePicker';
import { brandWordmark, isDefaultBrand } from 'utils/brand';
import { sanitizeHtml } from 'utils/sanitize';

// ==============================|| CHROME — FOOTER ||============================== //
// Matches the v1 auth footer: [R-mark] Modeltaps © 2026. Honors custom footer_html. The language
// picker sits at its right end, in both forms.

export default function Footer() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);

  if (siteInfo.footer_html) {
    return (
      <footer className="grid grid-cols-[1fr_auto_1fr] items-center gap-2 px-4 py-5 sm:px-6">
        <div
          className="custom-footer col-start-2 text-center text-sm text-muted-foreground"
          dangerouslySetInnerHTML={{ __html: sanitizeHtml(siteInfo.footer_html) }}
        />
        <div className="flex justify-end">
          <LanguagePicker />
        </div>
      </footer>
    );
  }

  return (
    <footer className="grid grid-cols-[1fr_auto_1fr] items-center gap-2 px-4 py-5 text-muted-foreground sm:px-6">
      <div className="col-start-2 flex items-center gap-1.5">
        <BrandLockup size={12} text={brandWordmark(siteInfo.system_name)} wordmark={isDefaultBrand(siteInfo.system_name)} />
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
      </div>
      <div className="flex justify-end">
        <LanguagePicker />
      </div>
    </footer>
  );
}
