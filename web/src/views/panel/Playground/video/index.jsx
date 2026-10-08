import { useTranslation } from 'react-i18next';
import { Clapperboard } from 'lucide-react';

import ModalityPlaceholder from '../components/ModalityPlaceholder';

// ==============================|| PLAYGROUND — VIDEO (ASSEMBLY) ||============================== //
// 视频还没有可运行的控制台：页面是一张空页面(图标 + 一句话 + 接口文档入口)，
// 不给运行按钮，避免出现点不动的控件。

export function VideoWorkspace() {
  const { t } = useTranslation();

  return (
    <ModalityPlaceholder
      icon={Clapperboard}
      title={t('playgroundConsole.modality.video')}
      description={t('playgroundConsole.comingSoonDesc')}
      docsHref="/panel/api/docs/video"
      docsLabel={t('playgroundConsole.viewDocs')}
    />
  );
}
