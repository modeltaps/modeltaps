import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Braces, Bug, Database, FileText, Languages, ListChecks, Table2, TerminalSquare } from 'lucide-react';

import ExampleGrid from '../components/ExampleGrid';

// ==============================|| PLAYGROUND — CHAT EMPTY STATE ||============================== //
// 还没有任何消息时的落地内容：2 行 × 4 列示例卡。每张卡除了提示词还带一份参数补丁，
// 点一下把提示词填进输入框、参数写进会话（不支持该参数的模型由 buildParams 自己滤掉）。
// 卡面文案（标题 / 说明 / 提示词）全部走 i18n，本文件只登记图标、能力 tag 与参数。

export const CHAT_EXAMPLES = [
  { id: 'stream', icon: TerminalSquare, tags: ['code', 'stream'] },
  { id: 'schema', icon: Braces, tags: ['code', 'json'], settings: { jsonMode: true } },
  { id: 'debug', icon: Bug, tags: ['reasoning'], settings: { thinking: true } },
  { id: 'review', icon: ListChecks, tags: ['code', 'reasoning'], settings: { thinking: true } },
  { id: 'summarize', icon: FileText, tags: ['longContext'], settings: { maxTokens: '1024' } },
  { id: 'extract', icon: Table2, tags: ['data', 'json'], settings: { jsonMode: true } },
  { id: 'sql', icon: Database, tags: ['data', 'code'] },
  { id: 'translate', icon: Languages, tags: ['translate'] }
];

export default function ChatEmptyState({ onPick }) {
  const { t } = useTranslation();

  const items = CHAT_EXAMPLES.map((example) => ({
    id: example.id,
    icon: <example.icon className="size-4" />,
    title: t(`playgroundConsole.chat.examples.${example.id}.title`),
    description: t(`playgroundConsole.chat.examples.${example.id}.desc`),
    tags: example.tags.map((tag) => t(`playgroundConsole.chat.tags.${tag}`)),
    prompt: t(`playgroundConsole.chat.examples.${example.id}.prompt`),
    settings: example.settings
  }));

  return (
    <ExampleGrid
      className="w-full"
      columns={4}
      items={items}
      onPick={onPick}
      title={t('playgroundConsole.chat.empty.title')}
      subtitle={t('playgroundConsole.chat.empty.subtitle')}
    />
  );
}

ChatEmptyState.propTypes = {
  onPick: PropTypes.func
};
