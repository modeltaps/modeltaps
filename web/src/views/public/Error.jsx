import { useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';

import NotFound from 'assets/images/404.svg';
import { Button } from '@/components/ui/button';

// ==============================|| 404 / NOT FOUND (shadcn) ||============================== //

export default function NotFoundView() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <div className="mx-auto flex min-h-[calc(100vh-8rem)] max-w-md flex-col items-center justify-center px-4 py-12 text-center">
      <img src={NotFound} alt="404" className="mx-auto my-8 h-64 sm:my-12" />
      <Button size="lg" onClick={() => navigate(-1)}>
        {t('common.back')}
      </Button>
    </div>
  );
}
