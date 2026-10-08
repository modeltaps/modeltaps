import { useTranslation } from 'react-i18next';

import CodeView from '../components/CodeView';
import useImageSession from './useImageSession';

// ==============================|| PLAYGROUND — IMAGE CODE VIEW ||============================== //
// 「代码」tab：三语言片段由 shared/codegen 按当前模型 / 提示词 / 运行设置实时生成，
// 与真实请求同一份参数来源；密钥恒为占位符。

export default function ImageCodeView() {
  const { t } = useTranslation();
  const { snippets, model, endpoint } = useImageSession();

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
