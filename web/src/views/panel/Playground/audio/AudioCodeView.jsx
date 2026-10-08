import { useTranslation } from 'react-i18next';

import CodeView from '../components/CodeView';
import useAudioSession from './useAudioSession';

// ==============================|| PLAYGROUND — AUDIO CODE VIEW ||============================== //
// 「代码」tab 跟随当前子方向（合成 / 转写）与当前音色、参数实时重生成；密钥恒为占位符，
// 上传文件只给占位路径（面板拿不到磁盘路径）。

export default function AudioCodeView() {
  const { t } = useTranslation();
  const { snippets, model, endpoint } = useAudioSession();

  return (
    <CodeView
      snippets={snippets}
      meta={{ model: model?.id, endpoint }}
      labels={{
        copy: t('playgroundConsole.code.copy'),
        copied: t('playgroundConsole.code.copied'),
        sync: t('playgroundConsole.code.sync'),
        keyHint: t('playgroundConsole.code.keyHint')
      }}
    />
  );
}
