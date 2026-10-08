import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import CodeView from '../components/CodeView';
import { CHAT_PATH } from '../chat/useChatSession';
import { buildSnippets } from '../shared/codegen';
import { columnRequest } from './useCompareSession';

// ==============================|| PLAYGROUND — COMPARE CODE VIEW ||============================== //
// 对比页的「查看代码」：顶部分段控件在列间切换（「列 N · 模型名」），下面是该列的等效请求，
// 语言 tab 与复制沿用 CodeView。请求体与发送共用 columnRequest，列覆盖的参数只进该列代码。
// 输入框里还没发出去的那条只算进会收到它的列：同步输入时每列，否则只有激活列。只吃 props。

const SEG_CLASS = 'rounded-md px-3 py-1.5 text-sm font-medium transition-colors';

export default function CompareCodeView({ session, baseUrl }) {
  const { t } = useTranslation();
  const [pickedId, setPickedId] = useState(null);

  const { columns, options, sync, activeId } = session;
  const column = columns.find((item) => item.id === pickedId) || columns.find((item) => item.id === activeId) || columns[0];
  const model = options.find((item) => item.id === column?.modelId) || null;
  const pending = sync || column?.id === activeId ? session.input : '';

  const request = useMemo(
    () => columnRequest({ system: session.system, settings: session.settings }, column, model, pending),
    [session.system, session.settings, column, model, pending]
  );
  const snippets = useMemo(
    () => buildSnippets({ modality: 'chat', baseUrl, model: request.model, params: request.params, messages: request.messages }),
    [baseUrl, request]
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 px-4 pt-4">
        <div role="tablist" aria-label={t('playgroundConsole.chat.codeTitle')} className="inline-flex flex-wrap rounded-lg bg-muted p-0.5">
          {columns.map((item, index) => {
            const selected = item.id === column?.id;
            const name = t('playgroundConsole.compare.column', { index: index + 1 });
            return (
              <button
                key={item.id}
                type="button"
                role="tab"
                aria-selected={selected}
                onClick={() => setPickedId(item.id)}
                className={cn(
                  SEG_CLASS,
                  selected ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {item.modelId ? `${name} · ${item.modelId}` : name}
              </button>
            );
          })}
        </div>
      </div>
      <CodeView
        snippets={snippets}
        meta={{ model: request.model, endpoint: CHAT_PATH }}
        labels={{
          copy: t('common.copy'),
          copied: t('apiCatalogPage.copied'),
          sync: t('playgroundConsole.code.sync'),
          keyHint: t('playground.curlHint')
        }}
      />
    </div>
  );
}

CompareCodeView.propTypes = {
  session: PropTypes.object.isRequired,
  baseUrl: PropTypes.string
};
