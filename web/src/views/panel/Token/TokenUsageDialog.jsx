import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import axios from 'axios';
import { Inbox } from 'lucide-react';

import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import CodeBlock from '@/components/ui/code-block';
import { Combobox } from '@/components/ui/combobox';
import { toast } from '@/components/ui/sonner';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { resolveServerAddress } from 'utils/serverAddress';

const FALLBACK_MODEL = 'gpt-4o-mini';
// 三标签页内容区定高:最高的 Claude 命令为 5 行,取略高于其自然高度的固定值,
// 使 OpenAI/Claude/Gemini 与空态之间切换时弹窗尺寸稳定不跳变。
const PANEL_MIN_H = 'min-h-[180px]';

// 「测试命令」弹窗:把当前 API Key 的真 key + 站点地址 + 各协议可用模型拼成可直接运行的 curl。
// 支持三种协议(与 API Key 页/后端中继一致),用户复制到终端即可验证 key 是否可用:
//   - OpenAI  : POST /v1/chat/completions              Authorization: Bearer
//   - Claude  : POST /claude/v1/messages               x-api-key + anthropic-version(max_tokens 必填)
//   - Gemini  : POST /gemini/v1beta/models/{m}:generateContent  x-goog-api-key
// 后端 tokenAuth 会剥离 "Bearer "/"sk-" 前缀,故三协议均沿用 sk-{key}。
// OpenAI 用 /api/available_model;Claude/Gemini 用该 Key 调各自中继端点,取该协议真正可路由的模型。
export default function TokenUsageDialog({ open, onOpenChange, token, serverAddress }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  // 各协议均保存「完整可选模型列表 + 当前选中模型」:列表喂给选择器,选中值拼进 curl。
  const [openaiModels, setOpenaiModels] = useState([]);
  const [model, setModel] = useState(FALLBACK_MODEL);
  const [gemini, setGemini] = useState({ status: 'idle', model: '', models: [] });
  const [claude, setClaude] = useState({ status: 'idle', model: '', models: [] });
  const [protocol, setProtocol] = useState('openai');

  const base = resolveServerAddress(serverAddress);
  const key = token?.key ? `sk-${token.key}` : 'sk-xxxxxx';
  const geminiEnabled = siteInfo?.GeminiAPIEnabled;
  const claudeEnabled = siteInfo?.ClaudeAPIEnabled;

  // 页签数量决定 TabsList 的等宽栅格列数(1~3),静态类名以便 Tailwind 编译期采集。
  const tabCount = 1 + (geminiEnabled ? 1 : 0) + (claudeEnabled ? 1 : 0);
  const gridColsClass = { 1: 'grid-cols-1', 2: 'grid-cols-2', 3: 'grid-cols-3' }[tabCount];
  // 激活态用品牌主色高亮,与灰底页签列拉开对比、与下方代码块形成清晰的「选中→内容」层级。
  const activeTabClass = 'data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm';

  useEffect(() => {
    if (!open) return;
    let alive = true;
    // 优先 API Key 白名单里的模型,否则取第一个可用模型,保证命令开箱即用
    const wl = token?.setting?.limits?.limit_model_setting;
    const pick = (ids) => {
      if (!ids.length) return '';
      if (wl?.enabled && wl.models?.length && ids.includes(wl.models[0])) return wl.models[0];
      return ids[0];
    };

    API.get('/api/available_model')
      .then((res) => {
        const { success, data } = res.data;
        if (!alive || !success || !data) return;
        const ids = Object.keys(data);
        setOpenaiModels(ids);
        const p = pick(ids);
        if (p) setModel(p);
      })
      .catch(() => {});

    // Gemini/Claude 各调协议中继端点(用该 Key 直连,避开 API 拦截器的 401 登出/报错弹窗)
    if (geminiEnabled) {
      setGemini({ status: 'loading', model: '', models: [] });
      axios
        .get('/gemini/v1beta/models', { headers: { 'x-goog-api-key': key } })
        .then((res) => {
          if (!alive) return;
          const ids = (res.data?.models || []).map((m) => (m.name || '').replace(/^models\//, '')).filter(Boolean);
          const p = pick(ids);
          setGemini(p ? { status: 'ready', model: p, models: ids } : { status: 'empty', model: '', models: [] });
        })
        .catch(() => {
          if (alive) setGemini({ status: 'error', model: '', models: [] });
        });
    }

    if (claudeEnabled) {
      setClaude({ status: 'loading', model: '', models: [] });
      axios
        .get('/claude/v1/models', { headers: { 'x-api-key': key, 'anthropic-version': '2023-06-01' } })
        .then((res) => {
          if (!alive) return;
          const ids = (res.data?.data || []).map((m) => m.id).filter(Boolean);
          const p = pick(ids);
          setClaude(p ? { status: 'ready', model: p, models: ids } : { status: 'empty', model: '', models: [] });
        })
        .catch(() => {
          if (alive) setClaude({ status: 'error', model: '', models: [] });
        });
    }

    return () => {
      alive = false;
    };
  }, [open, token, key, geminiEnabled, claudeEnabled]);

  const commands = useMemo(
    () => ({
      openai: `curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer ${key}" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "${model}", "messages": [{"role": "user", "content": "Hello"}]}'`,
      gemini: `curl ${base}/gemini/v1beta/models/${gemini.model}:generateContent \\
  -H "x-goog-api-key: ${key}" \\
  -H "Content-Type: application/json" \\
  -d '{"contents": [{"parts": [{"text": "Hello"}]}]}'`,
      claude: `curl ${base}/claude/v1/messages \\
  -H "x-api-key: ${key}" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "${claude.model}", "max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}]}'`
    }),
    [base, key, model, gemini.model, claude.model]
  );

  // curl 命令按终端原样展示:保留 `\` 续行与缩进,长 key/JSON 走横向滚动而非逐字符硬折行。
  const renderCommand = (code) => (
    <CodeBlock
      code={code}
      language="bash"
      copyLabel={t('token_index.copyCommand')}
      copiedLabel={t('token_index.commandCopiedShort')}
      onCopy={() => toast.success(t('token_index.commandCopied'))}
      onCopyError={() => toast.error(code)}
    />
  );

  // 加载中/空态统一用定高居中容器,与命令块同高,切换标签页不跳变。
  const renderCenteredState = (children) => (
    <div className={cn('flex flex-col items-center justify-center gap-1.5 px-4 text-center', PANEL_MIN_H)}>{children}</div>
  );

  // 模型选择器:命令块上方放一个可搜索单选框,选中即改写 curl 里的 model 字段。
  // 这里的语义是「从该协议真实可路由的模型里严格选一个」,而非输入任意别名,
  // 故用 ui/combobox(单选+搜索)而非 model-combobox(自由输入创建款)。列表为空则不渲染,空态展示不变。
  const renderModelSelect = (id, value, onSelect, models) => {
    if (!models.length) return null;
    return (
      <div className="mb-3 space-y-1.5">
        <label htmlFor={id} className="block text-sm font-medium text-foreground">
          {t('token_index.testCommandModelLabel')}
        </label>
        <Combobox
          id={id}
          value={value}
          onValueChange={onSelect}
          options={models.map((m) => ({ value: m, label: m }))}
          searchPlaceholder={t('token_index.testCommandModelSearch')}
          emptyText={t('token_index.testCommandModelEmpty')}
        />
      </div>
    );
  };

  // Gemini/Claude 标签页:加载中给提示,就绪展示选择器+命令,无模型/加载失败解释原因并指出出路(不展示注定失败的命令)
  const renderProtocolTab = (id, state, command, onSelect) => {
    if (state.status === 'loading') {
      return renderCenteredState(<p className="text-sm text-muted-foreground">{t('token_index.testCommandLoadingModels')}</p>);
    }
    if (state.status === 'ready' && state.model) {
      return (
        <>
          {renderModelSelect(id, state.model, onSelect, state.models)}
          {renderCommand(command)}
        </>
      );
    }
    return renderCenteredState(
      <>
        <Inbox className="size-6 text-muted-foreground/60" aria-hidden="true" />
        <p className="text-sm font-medium text-foreground">{t('token_index.testCommandNoModelTitle')}</p>
        <p className="max-w-sm text-sm text-muted-foreground">{t('token_index.testCommandNoModel')}</p>
        <p className="max-w-sm text-sm text-muted-foreground">{t('token_index.testCommandNoModelHint')}</p>
      </>
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader className="gap-1 px-6 py-4">
          <DialogTitle>{t('token_index.testCommandTitle')}</DialogTitle>
          <DialogDescription>{t('token_index.testCommandDesc')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="px-6 py-4">
          <Tabs value={protocol} onValueChange={setProtocol}>
            <TabsList className={cn('grid h-auto w-full p-1', gridColsClass)}>
              <TabsTrigger value="openai" className={activeTabClass}>
                {t('token_index.openaiApi')}
              </TabsTrigger>
              {claudeEnabled && (
                <TabsTrigger value="claude" className={activeTabClass}>
                  {t('token_index.claudeApi')}
                </TabsTrigger>
              )}
              {geminiEnabled && (
                <TabsTrigger value="gemini" className={activeTabClass}>
                  {t('token_index.geminiApi')}
                </TabsTrigger>
              )}
            </TabsList>
            <TabsContent value="openai" className={PANEL_MIN_H}>
              {renderModelSelect('token-test-model-openai', model, setModel, openaiModels)}
              {renderCommand(commands.openai)}
            </TabsContent>
            {claudeEnabled && (
              <TabsContent value="claude" className={PANEL_MIN_H}>
                {renderProtocolTab('token-test-model-claude', claude, commands.claude, (m) => setClaude((s) => ({ ...s, model: m })))}
              </TabsContent>
            )}
            {geminiEnabled && (
              <TabsContent value="gemini" className={PANEL_MIN_H}>
                {renderProtocolTab('token-test-model-gemini', gemini, commands.gemini, (m) => setGemini((s) => ({ ...s, model: m })))}
              </TabsContent>
            )}
          </Tabs>
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

TokenUsageDialog.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  token: PropTypes.object,
  serverAddress: PropTypes.string
};
