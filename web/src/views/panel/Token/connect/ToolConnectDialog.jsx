import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowUpRight, Bot, CircleCheck, CircleDashed, Inbox, Loader2, Plug, ListChecks } from 'lucide-react';

import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Combobox } from '@/components/ui/combobox';
import { Button } from '@/components/ui/button';
import CodeBlock from '@/components/ui/code-block';
import { toast } from '@/components/ui/sonner';
import BrandIcon from '@/components/brand/BrandIcon';
import { docsUrl } from '@/components/list-page/PageEmptyState';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { maskSecret } from 'utils/maskSecret';
import { resolveServerAddress } from 'utils/serverAddress';
import { PasteKeyField, normalizePastedKey, pastedKeyMatches } from '../KeySecret';
import { PairingCodeReveal, buildAgentPrompt, useSkillUrl } from '../PairingCode';
import { availableTools, buildToolGuide, findTool, pickModel } from './tools';

// "Connect a tool" (AGENT-8): pick a tool, then copy its configuration with this site's address,
// the key and a default model filled in. The configurations come from ./tools.js. Keys are
// stored as hashes only (SEC-47): the snippets hold the real key only when it is known (`secret`,
// right after this page created or reset it) or pasted; otherwise a placeholder. The "Let the AI
// set it up" tab hands the key over with a pairing code instead, so it never enters the chat.

const activeTabClass = 'data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm';

const CACHE_ICON = { verified: CircleCheck, unverified: CircleDashed, depends: CircleDashed };

function ToolCard({ tool, selected, onSelect }) {
  const { t } = useTranslation();
  const name = tool.name || t('connectTool.other.name');
  return (
    <button
      type="button"
      onClick={() => onSelect(tool.id)}
      aria-pressed={selected}
      data-testid={`tool-${tool.id}`}
      className={cn(
        'flex min-w-0 items-center gap-2.5 rounded-lg border bg-field px-3 py-2.5 text-left transition-colors',
        selected ? 'border-primary ring-1 ring-primary' : 'border-border hover:border-foreground/30'
      )}
    >
      {tool.icon ? (
        <BrandIcon icon={tool.icon} fallbackText={name} className="size-5 shrink-0" />
      ) : (
        <Plug className="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
      )}
      <span className="min-w-0">
        <span className="block truncate text-sm font-medium text-foreground">{name}</span>
        <span className="block truncate text-xs text-muted-foreground">{t(`connectTool.kind.${tool.kind}`)}</span>
      </span>
    </button>
  );
}

ToolCard.propTypes = { tool: PropTypes.object.isRequired, selected: PropTypes.bool, onSelect: PropTypes.func.isRequired };

function GuideBlock({ block, tool }) {
  const { t } = useTranslation();
  return (
    <div className="space-y-1.5">
      {block.path && <p className="font-mono text-xs text-muted-foreground">{block.path}</p>}
      <CodeBlock
        code={block.code}
        language={block.language}
        copyLabel={t('token_index.copy')}
        copiedLabel={t('token_index.commandCopiedShort')}
        onCopy={() => toast.success(t('connectTool.copied'))}
        onCopyError={() => toast.error(block.code)}
      />
      {block.note && <p className="text-xs text-muted-foreground">{t(`connectTool.note.${block.note}`, { tool: tool.name })}</p>}
    </div>
  );
}

GuideBlock.propTypes = { block: PropTypes.object.isRequired, tool: PropTypes.object.isRequired };

// The "Let the AI set it up" tab: the agent reads skill.md and redeems a pairing code. A code
// is known right after creation (`pairing`); otherwise one can be issued here, which gives the
// key a new secret once the agent redeems it.
function AgentTab({ token, tool, pairing, canIssue, onCopy }) {
  const { t } = useTranslation();
  const skillUrl = useSkillUrl();
  const [issued, setIssued] = useState(null);
  const [busy, setBusy] = useState(false);
  const code = issued || pairing;

  const issue = async () => {
    if (busy || !window.confirm(t('apiKey.agent.newCodeConfirm'))) return;
    setBusy(true);
    try {
      const res = await API.post(`/api/token/${token.id}/pairing_code`);
      const { success, message, data } = res.data;
      if (success) setIssued({ code: data.pairing_code, expiresAt: data.pairing_expires_at });
      else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  const prompt = code
    ? [
        t('connectTool.agent.promptTool', { tool: tool.name || t('connectTool.other.name') }),
        buildAgentPrompt(t, { skillUrl, code: code.code, expiresAt: code.expiresAt })
      ].join('\n\n')
    : '';

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">{t('connectTool.agent.desc')}</p>
      {code ? (
        <>
          <CodeBlock
            code={prompt}
            language="text"
            copyLabel={t('apiKey.pairing.copyForAgent')}
            copiedLabel={t('token_index.commandCopiedShort')}
            onCopy={() => toast.success(t('connectTool.copied'))}
            onCopyError={() => toast.error(prompt)}
          />
          {issued && <PairingCodeReveal code={issued.code} expiresAt={issued.expiresAt} onCopy={onCopy} />}
        </>
      ) : canIssue ? (
        <Button type="button" variant="outline" size="sm" className="gap-1.5 rounded-lg" onClick={issue} disabled={busy}>
          {busy ? <Loader2 className="size-3.5 animate-spin" /> : <Bot className="size-3.5" />}
          {t('apiKey.agent.newCode')}
        </Button>
      ) : (
        <p className="text-sm text-muted-foreground">{t('connectTool.agent.cannotIssue')}</p>
      )}
      <p className="text-xs text-muted-foreground">{t('connectTool.agent.resetNote')}</p>
    </div>
  );
}

AgentTab.propTypes = {
  token: PropTypes.object.isRequired,
  tool: PropTypes.object.isRequired,
  pairing: PropTypes.object,
  canIssue: PropTypes.bool,
  onCopy: PropTypes.func.isRequired
};

export default function ToolConnectDialog({ open, onOpenChange, token, tokens, secrets, pairing, canIssueFor, serverAddress, onCopy }) {
  const { t, i18n } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo) || {};
  const anthropic = Boolean(siteInfo.ClaudeAPIEnabled);
  const base = resolveServerAddress(serverAddress ?? siteInfo.server_address);
  const tools = useMemo(() => availableTools({ anthropic }), [anthropic]);

  const [toolId, setToolId] = useState('');
  const [pickedId, setPickedId] = useState(null);
  const [pasted, setPasted] = useState('');
  const [models, setModels] = useState([]);
  const [model, setModel] = useState('');
  const [loading, setLoading] = useState(false);
  const [tab, setTab] = useState('');

  // Opened from a key row or a new key: that key. From the page header: a key picked here.
  const choices = useMemo(() => (tokens || []).filter((x) => x.status === 1), [tokens]);
  const current = token || choices.find((x) => x.id === pickedId) || choices[0] || null;
  const tool = findTool(toolId);

  const secret = current ? secrets?.[current.id] : undefined;
  const typedKey = normalizePastedKey(pasted);
  const key = secret || (typedKey && pastedKeyMatches(typedKey, current?.key_hint) ? typedKey : '');

  const handleOpenChange = (next) => {
    if (!next) {
      setPasted('');
      setToolId('');
      setPickedId(null);
      setTab('');
    }
    onOpenChange(next);
  };

  // The models this key may call (/api/token/:id/models, the same answer as /v1/models).
  const tokenId = current?.id;
  useEffect(() => {
    if (!open || !tokenId) return undefined;
    let alive = true;
    setLoading(true);
    setModels([]);
    API.get(`/api/token/${tokenId}/models`)
      .then((res) => {
        if (!alive) return;
        const ids = res.data?.success && Array.isArray(res.data.data) ? res.data.data.filter((m) => typeof m === 'string') : [];
        setModels(ids);
      })
      .catch(() => {})
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [open, tokenId]);

  // A new tool or model list resets the default model to the tool's preference.
  useEffect(() => {
    if (tool?.needsModel) setModel(pickModel(models, tool.prefer));
  }, [tool, models]);

  const guide = useMemo(() => (tool ? buildToolGuide(tool, { base, key, model, anthropic }) : null), [tool, base, key, model, anthropic]);
  const tabs = guide ? [...guide.methods.map((m) => m.id), ...(current ? ['ai'] : [])] : [];
  const activeTab = tabs.includes(tab) ? tab : tabs[0];
  const modelMissing = tool?.needsModel && !loading && models.length === 0;
  const CacheIcon = guide ? CACHE_ICON[guide.cache] : null;
  const docs = tool?.docs ? docsUrl(siteInfo.docs_link, i18n.language, tool.docs) : '';
  const keyOptions = useMemo(() => choices.map((x) => ({ value: String(x.id), label: `${x.name} (${x.key_hint || ''})` })), [choices]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader className="gap-1 px-6 py-4">
          <DialogTitle>{t('connectTool.title')}</DialogTitle>
          <DialogDescription>{t('connectTool.desc')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-5 px-6 py-4">
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3" role="group" aria-label={t('connectTool.pickTool')}>
            {tools.map((x) => (
              <ToolCard key={x.id} tool={x} selected={x.id === toolId} onSelect={setToolId} />
            ))}
          </div>

          {tool && !current && (
            <div className="flex flex-col items-center gap-1.5 rounded-md border border-border px-4 py-6 text-center">
              <Inbox className="size-6 text-muted-foreground/60" aria-hidden="true" />
              <p className="max-w-sm text-sm text-muted-foreground">{t('connectTool.noKey')}</p>
            </div>
          )}

          {tool && current && (
            <div className="space-y-4">
              {!token && (
                <div className="grid gap-1.5">
                  <span className="text-sm font-medium text-foreground">{t('connectTool.key')}</span>
                  <Combobox
                    value={String(current.id)}
                    onValueChange={(v) => {
                      setPickedId(Number(v));
                      setPasted('');
                    }}
                    options={keyOptions}
                    placeholder={t('connectTool.key')}
                  />
                </div>
              )}
              {secret ? (
                <div className="grid gap-1.5">
                  <span className="text-sm font-medium text-foreground">{t('token_index.openclawApiKey')}</span>
                  <code className="block truncate rounded-md border border-border bg-muted/40 px-3 py-2 font-mono text-xs">
                    {maskSecret(secret)}
                  </code>
                </div>
              ) : (
                <PasteKeyField id="connect-tool-key" value={pasted} onChange={setPasted} hint={current.key_hint} />
              )}

              {tool.needsModel && (
                <div className="grid gap-1.5">
                  <span className="text-sm font-medium text-foreground">{t('connectTool.model')}</span>
                  {loading ? (
                    <p className="py-2 text-sm text-muted-foreground">{t('token_index.testCommandLoadingModels')}</p>
                  ) : modelMissing ? (
                    <p className="rounded-md border border-border px-3 py-3 text-sm text-muted-foreground">
                      {t('token_index.openclawNoModel')}
                    </p>
                  ) : (
                    <Combobox
                      value={model}
                      onValueChange={setModel}
                      options={models.map((id) => ({ value: id, label: id }))}
                      placeholder={t('token_index.openclawModelPlaceholder')}
                      searchPlaceholder={t('token_index.openclawModelSearchPlaceholder')}
                      emptyText={t('token_index.openclawModelSearchEmpty')}
                    />
                  )}
                  {tool.id === 'goose' || tool.id === 'opencode' ? (
                    <p className="text-xs text-muted-foreground">{t(`connectTool.modelHint.${tool.id}`)}</p>
                  ) : null}
                </div>
              )}

              {!modelMissing && !loading && guide && (
                <Tabs value={activeTab} onValueChange={setTab}>
                  <TabsList
                    className={cn(
                      'grid h-auto w-full p-1',
                      tabs.length === 3 ? 'grid-cols-3' : tabs.length === 2 ? 'grid-cols-2' : 'grid-cols-1'
                    )}
                  >
                    {tabs.map((id) => (
                      <TabsTrigger key={id} value={id} className={activeTabClass}>
                        {t(`connectTool.method.${id}`)}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                  {guide.methods.map((m) => (
                    <TabsContent key={m.id} value={m.id} className="space-y-4">
                      {m.blocks.map((b, i) => (
                        <GuideBlock key={i} block={b} tool={tool} />
                      ))}
                    </TabsContent>
                  ))}
                  <TabsContent value="ai">
                    <AgentTab
                      key={current.id}
                      token={current}
                      tool={tool}
                      pairing={pairing}
                      canIssue={canIssueFor ? canIssueFor(current) : false}
                      onCopy={onCopy}
                    />
                  </TabsContent>
                </Tabs>
              )}

              {guide && (
                <div className="space-y-2 border-t border-border pt-3 text-xs text-muted-foreground">
                  <p className="flex items-start gap-1.5" data-testid="tool-cache">
                    <CacheIcon
                      className={cn('mt-0.5 size-3.5 shrink-0', guide.cache === 'verified' && 'text-success')}
                      aria-hidden="true"
                    />
                    <span>{t(`connectTool.cache.${tool.id}.${guide.cache}`, { defaultValue: t(`connectTool.cache.${guide.cache}`) })}</span>
                  </p>
                  {tool.verify && (
                    <p className="flex items-start gap-1.5">
                      <ListChecks className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                      <span>{t(`connectTool.verify.${tool.verify}`)}</span>
                    </p>
                  )}
                  <div className="flex flex-wrap gap-x-4 gap-y-1">
                    {tool.site && (
                      <a
                        href={tool.site}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-0.5 font-medium text-primary hover:underline"
                      >
                        {t('connectTool.install', { tool: tool.name })}
                        <ArrowUpRight className="size-3" aria-hidden="true" />
                      </a>
                    )}
                    {docs && (
                      <a
                        href={docs}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-0.5 font-medium text-primary hover:underline"
                      >
                        {t('connectTool.docs')}
                        <ArrowUpRight className="size-3" aria-hidden="true" />
                      </a>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

ToolConnectDialog.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  // The key to connect; when left out, the dialog offers the enabled keys of `tokens`.
  token: PropTypes.object,
  tokens: PropTypes.array,
  // Full keys known to this page (just created or reset), by key id.
  secrets: PropTypes.object,
  // The pairing code shown at creation, when the dialog opens right after it.
  pairing: PropTypes.shape({ code: PropTypes.string, expiresAt: PropTypes.number }),
  // Whether the viewer may issue a new pairing code for a key (the same rule as reset).
  canIssueFor: PropTypes.func,
  serverAddress: PropTypes.string,
  onCopy: PropTypes.func.isRequired
};
