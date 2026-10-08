import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Inbox, Info } from 'lucide-react';

import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Combobox } from '@/components/ui/combobox';
import CodeBlock from '@/components/ui/code-block';
import { toast } from '@/components/ui/sonner';
import { cn } from '@/lib/utils';
import { resolveServerAddress } from 'utils/serverAddress';

// curated 默认优先级(子串匹配,兼容 anthropic/claude-opus-4.8 这类带供应商前缀的 id)
const CURATED = [/claude-opus/i, /claude-sonnet/i, /gpt-5/i, /gemini.*pro/i];
const curatedPick = (ids) => {
  for (const re of CURATED) {
    const hit = ids.find((id) => re.test(id));
    if (hit) return hit;
  }
  return ids[0] || '';
};

// API Key 密钥脱敏展示(与 TokenTable 一致);命令里用真实 sk-{key}。
const maskKey = (k) => (k.length > 9 ? `sk-${k.slice(0, 3)}***${k.slice(-6)}` : `sk-${k}`);

// OpenClaw 官网(桌面版 App / CLI 下载入口);未安装 OpenClaw 的用户由此前往安装。
const OPENCLAW_SITE_URL = 'https://openclaw.ai';

// 「接入 OpenClaw」弹窗:选中 API Key 可用模型后,给出两条接入路径 ——「一键接入」拼出托管脚本
// openclaw-setup.sh 的一键命令(桌面版 App / CLI 通用),「配置文件」给出等价 openclaw.json 片段。
// 模型数据源:用该 Key 鉴权请求 /v1/models(服务端已做分组∩API Key 白名单过滤),
// 默认选中按 curated 优先级 → 列表第一个。
export default function OpenClawConnectDialog({ open, onOpenChange, token, serverAddress }) {
  const { t } = useTranslation();
  const [models, setModels] = useState([]);
  const [model, setModel] = useState('');
  const [loading, setLoading] = useState(false);
  const [tab, setTab] = useState('desktop');

  // 激活态用品牌主色高亮,与 TokenUsageDialog 的 Tabs 样式一致。
  const activeTabClass = 'data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm';

  const base = resolveServerAddress(serverAddress);
  const rawKey = token?.key || '';
  const key = rawKey ? `sk-${rawKey}` : 'sk-xxxxxx';

  // provider id 按站点 host 区分:正式站用 modeltaps,其余(本地/内网/测试域名)用 modeltaps-local。
  const providerId = useMemo(() => {
    let host;
    try {
      host = new URL(base).hostname.toLowerCase();
    } catch {
      host = '';
    }
    return host === 'modeltaps.com' || host === 'www.modeltaps.com' ? 'modeltaps' : 'modeltaps-local';
  }, [base]);

  useEffect(() => {
    if (!open) return;
    let alive = true;
    setLoading(true);
    setModels([]);
    setModel('');

    // 用该 Key 鉴权拉取其真实可用模型(服务端已做分组∩白名单过滤);同源相对路径避免 CORS。
    fetch('/v1/models', { headers: rawKey ? { Authorization: `Bearer sk-${rawKey}` } : {} })
      .then((res) => (res.ok ? res.json() : null))
      .then((body) => {
        if (!alive) return;
        const data = Array.isArray(body?.data) ? body.data : [];
        const list = data
          .filter((m) => m && typeof m.id === 'string')
          .map((m) => ({ id: m.id }));
        setModels(list);
        setModel(curatedPick(list.map((m) => m.id)));
        setLoading(false);
      })
      .catch(() => {
        if (alive) setLoading(false);
      });

    return () => {
      alive = false;
    };
  }, [open, token, rawKey]);

  // 桌面版一键命令:入参走环境变量,交由托管脚本 openclaw-setup.sh 三层降级完成接入。
  const desktopCommand = useMemo(
    () =>
      `MODELTAPS_BASE_URL="${base}/v1" \\
MODELTAPS_API_KEY="${key}" \\
MODELTAPS_MODEL="${model}" \\
MODELTAPS_PROVIDER_ID="${providerId}" \\
bash -c "$(curl -fsSL ${base}/openclaw-setup.sh)"`,
    [base, key, model, providerId]
  );

  // 与命令等价的 openclaw.json 片段(随所选模型联动);用 JSON.stringify 保证合法。
  const config = useMemo(
    () =>
      JSON.stringify(
        {
          models: {
            providers: {
              [providerId]: {
                baseUrl: `${base}/v1`,
                apiKey: key,
                api: 'openai-completions',
                models: [{ id: model, name: model }]
              }
            }
          },
          agents: {
            defaults: {
              model: {
                primary: `${providerId}/${model}`
              },
              models: {
                [`${providerId}/${model}`]: {}
              }
            }
          }
        },
        null,
        2
      ),
    [base, key, model, providerId]
  );

  const hasModels = models.length > 0;
  // Combobox 按 label 过滤,label 即模型 id 以便按模型 id 搜索。
  const modelOptions = useMemo(() => models.map((m) => ({ value: m.id, label: m.id })), [models]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader className="gap-1 px-6 py-4">
          <DialogTitle>{t('token_index.openclawConnectTitle')}</DialogTitle>
          <DialogDescription>{t('token_index.openclawConnectDesc')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4 px-6 py-4">
          <div className="grid gap-1.5">
            <span className="text-sm font-medium text-foreground">{t('token_index.openclawBaseUrl')}</span>
            <code className="block truncate rounded-md border border-border bg-muted/40 px-3 py-2 font-mono text-xs">{`${base}/v1`}</code>
          </div>
          <div className="grid gap-1.5">
            <span className="text-sm font-medium text-foreground">{t('token_index.openclawApiKey')}</span>
            <code className="block truncate rounded-md border border-border bg-muted/40 px-3 py-2 font-mono text-xs">{maskKey(rawKey)}</code>
          </div>
          <div className="grid gap-1.5">
            <span className="text-sm font-medium text-foreground">{t('token_index.openclawModel')}</span>
            {loading ? (
              <p className="py-2 text-sm text-muted-foreground">{t('token_index.testCommandLoadingModels')}</p>
            ) : hasModels ? (
              <Combobox
                value={model}
                onValueChange={setModel}
                options={modelOptions}
                placeholder={t('token_index.openclawModelPlaceholder')}
                searchPlaceholder={t('token_index.openclawModelSearchPlaceholder')}
                emptyText={t('token_index.openclawModelSearchEmpty')}
              />
            ) : (
              <div className={cn('flex flex-col items-center justify-center gap-1.5 rounded-md border border-border px-4 py-6 text-center')}>
                <Inbox className="size-6 text-muted-foreground/60" aria-hidden="true" />
                <p className="max-w-sm text-sm text-muted-foreground">{t('token_index.openclawNoModel')}</p>
              </div>
            )}
          </div>
          {hasModels && !loading && (
            <Tabs value={tab} onValueChange={setTab}>
              <TabsList className="grid h-auto w-full grid-cols-2 p-1">
                <TabsTrigger value="desktop" className={activeTabClass}>
                  {t('token_index.openclawTabDesktop')}
                </TabsTrigger>
                <TabsTrigger value="config" className={activeTabClass}>
                  {t('token_index.openclawTabConfig')}
                </TabsTrigger>
              </TabsList>
              <TabsContent value="desktop" className="space-y-4">
                <div className="flex items-start gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                  <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                  <span>
                    {t('token_index.openclawDesktopDesc')}{' '}
                    {t('token_index.openclawInstallPrompt')}{' '}
                    <a
                      href={OPENCLAW_SITE_URL}
                      target="_blank"
                      rel="noreferrer"
                      className="font-medium text-primary underline-offset-2 hover:underline"
                    >
                      {t('token_index.openclawInstallSite')}
                    </a>
                  </span>
                </div>
                <CodeBlock
                  code={desktopCommand}
                  language="bash"
                  copyLabel={t('token_index.copyCommand')}
                  copiedLabel={t('token_index.commandCopiedShort')}
                  onCopy={() => toast.success(t('token_index.commandCopied'))}
                  onCopyError={() => toast.error(desktopCommand)}
                />
                <p className="text-sm text-muted-foreground">{t('token_index.openclawDesktopFooter')}</p>
              </TabsContent>
              <TabsContent value="config" className="space-y-4">
                <CodeBlock
                  code={config}
                  language="json"
                  copyLabel={t('token_index.openclawCopyConfig')}
                  copiedLabel={t('token_index.commandCopiedShort')}
                  onCopy={() => toast.success(t('token_index.openclawConfigCopied'))}
                  onCopyError={() => toast.error(config)}
                />
                <p className="text-sm text-muted-foreground">{t('token_index.openclawConfigFooter')}</p>
                <p className="text-xs text-muted-foreground">{t('token_index.openclawConfigInstallNote')}</p>
              </TabsContent>
            </Tabs>
          )}
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

OpenClawConnectDialog.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  token: PropTypes.object,
  serverAddress: PropTypes.string
};
