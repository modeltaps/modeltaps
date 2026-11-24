import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import {
  ArrowUpRight,
  Bot,
  Check,
  ChevronRight,
  CircleCheck,
  CircleDashed,
  Copy,
  Inbox,
  ListChecks,
  Loader2,
  Plug,
  Search,
  TriangleAlert
} from 'lucide-react';

import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody } from '@/components/ui/dialog';
import { Combobox } from '@/components/ui/combobox';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import CodeBlock from '@/components/ui/code-block';
import { toast } from '@/components/ui/sonner';
import BrandIcon from '@/components/brand/BrandIcon';
import { docsUrl } from '@/components/list-page/PageEmptyState';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { maskSecret } from 'utils/maskSecret';
import { resolveServerAddress } from 'utils/serverAddress';
import { PasteKeyField, normalizePastedKey, pastedKeyMatches, useCopied } from '../KeySecret';
import { PairingCodeReveal, buildAgentPrompt, useSkillUrl } from '../PairingCode';
import { CATEGORIES, POPULAR, availableTools, buildToolGuide, findTool, isPrivateAddress, pickModel, searchTools } from './tools';

// "Connect a tool" (AGENT-8, AGENT-9). A directory of tools (the popular ones, a search box and
// the full list by category); picking one shows the way to connect it that asks the least of the
// user, then the other ways, with the hand-written config file folded away at the end. Every
// value comes from ./tools.js with this site's address, the key and a default model filled in.
// Keys are stored as hashes only (SEC-47): the real key is in the values only when it is known
// (`secret`, right after this page created or reset it) or pasted; otherwise a placeholder.

const CACHE_ICON = { verified: CircleCheck, unverified: CircleDashed, depends: CircleDashed };
const APP_ICON = { ccswitch: 'ccswitch', cherry: 'cherrystudio' };
const APP_SITE = { ccswitch: 'https://ccswitch.io', cherry: 'https://cherry-ai.com' };

const toolName = (t, tool) => tool.name || t('connectTool.other.name');

// The back arrow sized to the text beside it (its strokes as tall as the glyphs).
function BackArrow() {
  return (
    <svg viewBox="0 0 8 16" className="h-[1em] w-[0.5em]" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M6.5 1.5 1.5 8l5 6.5" strokeLinecap="round" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

function ToolIcon({ tool, className }) {
  const { t } = useTranslation();
  return tool.icon ? (
    <BrandIcon icon={tool.icon} fallbackText={toolName(t, tool)} className={cn('size-5 shrink-0', className)} />
  ) : (
    <Plug className={cn('size-5 shrink-0 text-muted-foreground', className)} aria-hidden="true" />
  );
}

ToolIcon.propTypes = { tool: PropTypes.object.isRequired, className: PropTypes.string };

function ToolCard({ tool, onSelect }) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      onClick={() => onSelect(tool.id)}
      data-testid={`tool-${tool.id}`}
      className="flex min-w-0 items-center gap-2.5 rounded-lg border border-border bg-field px-3 py-2.5 text-left transition-colors hover:border-foreground/30"
    >
      <ToolIcon tool={tool} />
      <span className="min-w-0">
        <span className="block truncate text-sm font-medium text-foreground">{toolName(t, tool)}</span>
        <span className="block truncate text-xs text-muted-foreground">{t(`connectTool.method.${tool.primary}Short`)}</span>
      </span>
    </button>
  );
}

ToolCard.propTypes = { tool: PropTypes.object.isRequired, onSelect: PropTypes.func.isRequired };

function ToolGrid({ tools, onSelect }) {
  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
      {tools.map((x) => (
        <ToolCard key={x.id} tool={x} onSelect={onSelect} />
      ))}
    </div>
  );
}

ToolGrid.propTypes = { tools: PropTypes.array.isRequired, onSelect: PropTypes.func.isRequired };

// The directory: popular tools, then every tool by category; a search narrows it to one list.
function Directory({ tools, onSelect }) {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const found = searchTools(tools, query);
  const popular = POPULAR.map((id) => tools.find((x) => x.id === id)).filter(Boolean);
  return (
    <div className="space-y-5">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('connectTool.search', { count: tools.length })}
          aria-label={t('connectTool.searchLabel')}
          className="pl-9"
        />
      </div>
      {query.trim() ? (
        found.length ? (
          <ToolGrid tools={found} onSelect={onSelect} />
        ) : (
          <div className="flex flex-col items-center gap-1.5 rounded-md border border-border px-4 py-6 text-center">
            <Inbox className="size-6 text-muted-foreground/60" aria-hidden="true" />
            <p className="max-w-sm text-sm text-muted-foreground">{t('connectTool.noMatch')}</p>
            <Button variant="link" size="sm" className="h-auto p-0" onClick={() => onSelect('other')}>
              {t('connectTool.useOther')}
            </Button>
          </div>
        )
      ) : (
        <>
          <section className="space-y-2">
            <h3 className="text-xs font-medium text-muted-foreground">{t('connectTool.popular')}</h3>
            <ToolGrid tools={popular} onSelect={onSelect} />
          </section>
          {CATEGORIES.map((category) => {
            const list = tools.filter((x) => x.category === category && !POPULAR.includes(x.id));
            if (!list.length) return null;
            return (
              <section key={category} className="space-y-2">
                <h3 className="text-xs font-medium text-muted-foreground">{t(`connectTool.category.${category}`)}</h3>
                <ToolGrid tools={list} onSelect={onSelect} />
              </section>
            );
          })}
        </>
      )}
    </div>
  );
}

Directory.propTypes = { tools: PropTypes.array.isRequired, onSelect: PropTypes.func.isRequired };

// One value to paste into the tool's own settings screen. A key shows masked and copies in full.
function ValueRow({ label, value, secret, missing, onCopy }) {
  const { t } = useTranslation();
  const [copied, markCopied] = useCopied();
  return (
    <div className="grid grid-cols-[7.5rem_1fr_auto] items-center gap-2">
      <span className="truncate font-mono text-xs text-muted-foreground" title={label}>
        {label}
      </span>
      <code
        className="min-w-0 truncate rounded-md border border-border bg-muted/40 px-2.5 py-1.5 font-mono text-[13px]"
        title={secret ? undefined : value}
      >
        {missing ? (
          <span className="font-sans text-muted-foreground">{t('connectTool.keyMissing')}</span>
        ) : secret ? (
          maskSecret(value)
        ) : (
          value
        )}
      </code>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-8 shrink-0 rounded-md text-muted-foreground hover:text-foreground"
        aria-label={`${t('token_index.copy')} ${label}`}
        disabled={missing}
        onClick={() => {
          onCopy(value, label);
          markCopied();
        }}
      >
        {copied ? <Check className="size-4 text-success" /> : <Copy className="size-4" />}
      </Button>
    </div>
  );
}

ValueRow.propTypes = {
  label: PropTypes.string,
  value: PropTypes.string,
  secret: PropTypes.bool,
  missing: PropTypes.bool,
  onCopy: PropTypes.func.isRequired
};

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

// Import buttons that open a local app's own confirmation dialog. They need the real key: the
// link carries it, so the hint suggests a key of its own for the import.
function DeeplinkMethod({ method, keyKnown, onCreateKey }) {
  const { t } = useTranslation();
  return (
    <div className="space-y-2.5">
      <div className="flex flex-wrap gap-2">
        {method.links.map((link) => (
          <Button key={link.app} asChild={keyKnown} disabled={!keyKnown} className="gap-2 rounded-lg" data-testid={`import-${link.app}`}>
            {keyKnown ? (
              <a href={link.url}>
                <BrandIcon icon={APP_ICON[link.app]} fallbackText={link.app} className="size-4" />
                {t(`connectTool.import.${link.app}`)}
              </a>
            ) : (
              <span>
                <BrandIcon icon={APP_ICON[link.app]} fallbackText={link.app} className="size-4" />
                {t(`connectTool.import.${link.app}`)}
              </span>
            )}
          </Button>
        ))}
      </div>
      {!keyKnown && (
        <p className="flex items-start gap-1.5 text-xs text-muted-foreground">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          <span>
            {t('connectTool.import.needKey')}{' '}
            {onCreateKey && (
              <Button variant="link" size="sm" className="h-auto p-0 text-xs" onClick={onCreateKey}>
                {t('connectTool.import.newKey')}
              </Button>
            )}
          </span>
        </p>
      )}
      {method.links.map((link) => (
        <p key={link.app} className="text-xs text-muted-foreground">
          {t(`connectTool.import.${link.app}Hint`)}{' '}
          <a href={APP_SITE[link.app]} target="_blank" rel="noreferrer" className="font-medium text-primary hover:underline">
            {t('connectTool.import.getApp')}
          </a>
        </p>
      ))}
      {method.links.some((link) => link.app === 'ccswitch') && (
        <p className="text-xs text-muted-foreground">{t('connectTool.import.ccswitchConflict')}</p>
      )}
      <p className="text-xs text-muted-foreground">{t('connectTool.import.keyInLink')}</p>
    </div>
  );
}

DeeplinkMethod.propTypes = { method: PropTypes.object.isRequired, keyKnown: PropTypes.bool, onCreateKey: PropTypes.func };

// Numbered steps in the tool's own settings screen, then the values to paste.
function FieldsMethod({ method, tool, base, keyKnown, onCopy }) {
  const { t } = useTranslation();
  const steps = t(`connectTool.steps.${method.steps}`, { returnObjects: true, tool: tool.name });
  return (
    <div className="space-y-3">
      {Array.isArray(steps) && (
        <ol className="list-decimal space-y-1 pl-5 text-sm text-foreground">
          {steps.map((s, i) => (
            <li key={i}>{s}</li>
          ))}
        </ol>
      )}
      <div className="space-y-1.5">
        {method.fields.map((f) => (
          <ValueRow key={f.label} label={f.label} value={f.value} secret={f.secret} missing={f.secret && !keyKnown} onCopy={onCopy} />
        ))}
      </div>
      {tool.note === 'cursorPublic' && isPrivateAddress(base) && (
        <p className="flex items-start gap-1.5 text-xs text-amber-600 dark:text-amber-500">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          <span>{t('connectTool.note.cursorPublic')}</span>
        </p>
      )}
    </div>
  );
}

FieldsMethod.propTypes = {
  method: PropTypes.object.isRequired,
  keyKnown: PropTypes.bool,
  tool: PropTypes.object.isRequired,
  base: PropTypes.string,
  onCopy: PropTypes.func.isRequired
};

function CodeMethod({ method, tool }) {
  return (
    <div className="space-y-4">
      {method.blocks.map((b, i) => (
        <GuideBlock key={i} block={b} tool={tool} />
      ))}
    </div>
  );
}

CodeMethod.propTypes = { method: PropTypes.object.isRequired, tool: PropTypes.object.isRequired };

// "Let the AI set it up": the agent reads skill.md and redeems a pairing code. A code is known
// right after creation (`pairing`); otherwise one can be issued here, which gives the key a new
// secret once the agent redeems it.
function AgentMethod({ token, tool, pairing, canIssue, onCopy }) {
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
        t('connectTool.agent.promptTool', { tool: toolName(t, tool) }),
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

AgentMethod.propTypes = {
  token: PropTypes.object.isRequired,
  tool: PropTypes.object.isRequired,
  pairing: PropTypes.object,
  canIssue: PropTypes.bool,
  onCopy: PropTypes.func.isRequired
};

// A folded section for the ways after the first.
function Fold({ title, children, testId }) {
  return (
    <details className="group rounded-lg border border-border" data-testid={testId}>
      <summary className="flex cursor-pointer list-none items-center gap-1.5 px-3 py-2.5 text-sm font-medium text-foreground [&::-webkit-details-marker]:hidden">
        <ChevronRight className="size-4 text-muted-foreground transition-transform group-open:rotate-90" aria-hidden="true" />
        {title}
      </summary>
      <div className="border-t border-border px-3 py-3">{children}</div>
    </details>
  );
}

Fold.propTypes = { title: PropTypes.node, children: PropTypes.node, testId: PropTypes.string };

export default function ToolConnectDialog({
  open,
  onOpenChange,
  token,
  tokens,
  secrets,
  pairing,
  canIssueFor,
  serverAddress,
  onCopy,
  onCreateKey
}) {
  const { t, i18n } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo) || {};
  const anthropic = Boolean(siteInfo.ClaudeAPIEnabled);
  const gemini = Boolean(siteInfo.GeminiAPIEnabled);
  const base = resolveServerAddress(serverAddress ?? siteInfo.server_address);
  const siteName = siteInfo.system_name || 'Modeltaps';
  const tools = useMemo(() => availableTools({ anthropic, gemini }), [anthropic, gemini]);

  const [toolId, setToolId] = useState('');
  const [pickedId, setPickedId] = useState(null);
  const [pasted, setPasted] = useState('');
  const [models, setModels] = useState([]);
  const [model, setModel] = useState('');
  const [loading, setLoading] = useState(false);

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

  const guide = useMemo(
    () => (tool ? buildToolGuide(tool, { base, key, model, anthropic, siteName }) : null),
    [tool, base, key, model, anthropic, siteName]
  );
  const modelMissing = tool?.needsModel && !loading && models.length === 0;
  const CacheIcon = guide ? CACHE_ICON[guide.cache] : null;
  const docs = tool?.docs ? docsUrl(siteInfo.docs_link, i18n.language, tool.docs) : '';
  const keyOptions = useMemo(() => choices.map((x) => ({ value: String(x.id), label: `${x.name} (${x.key_hint || ''})` })), [choices]);

  const renderMethod = (m) => {
    if (m.id === 'deeplink') return <DeeplinkMethod method={m} keyKnown={Boolean(key)} onCreateKey={onCreateKey} />;
    if (m.id === 'fields') return <FieldsMethod method={m} tool={tool} base={base} keyKnown={Boolean(key)} onCopy={onCopy} />;
    return <CodeMethod method={m} tool={tool} />;
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader className="gap-1 px-6 py-4">
          {tool ? (
            <>
              <button
                type="button"
                onClick={() => setToolId('')}
                className="inline-flex w-fit items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
                data-testid="tool-back"
              >
                <BackArrow />
                {t('connectTool.allTools')}
              </button>
              <DialogTitle className="flex items-center gap-2">
                <ToolIcon tool={tool} />
                {t('connectTool.titleFor', { tool: toolName(t, tool) })}
              </DialogTitle>
              <DialogDescription>{t(`connectTool.method.${guide.primary.id}Desc`)}</DialogDescription>
            </>
          ) : (
            <>
              <DialogTitle>{t('connectTool.title')}</DialogTitle>
              <DialogDescription>{t('connectTool.desc')}</DialogDescription>
            </>
          )}
        </DialogHeader>
        <DialogBody className="space-y-5 px-6 py-4">
          {!tool && <Directory tools={tools} onSelect={setToolId} />}

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
                <>
                  <section className="space-y-3 rounded-lg border border-primary/40 p-4" data-testid="tool-primary">
                    <h3 className="text-sm font-semibold text-foreground">{t(`connectTool.method.${guide.primary.id}`)}</h3>
                    {renderMethod(guide.primary)}
                  </section>
                  {(guide.others.length > 0 || current) && (
                    <div className="space-y-2">
                      <h3 className="text-xs font-medium text-muted-foreground">{t('connectTool.otherWays')}</h3>
                      {guide.others.map((m) => (
                        <Fold key={m.id} title={t(`connectTool.method.${m.id}`)} testId={`tool-way-${m.id}`}>
                          {renderMethod(m)}
                        </Fold>
                      ))}
                      <Fold title={t('connectTool.method.ai')} testId="tool-way-ai">
                        <AgentMethod
                          key={current.id}
                          token={current}
                          tool={tool}
                          pairing={pairing}
                          canIssue={canIssueFor ? canIssueFor(current) : false}
                          onCopy={onCopy}
                        />
                      </Fold>
                      {guide.manual && (
                        <Fold title={t('connectTool.method.file')} testId="tool-way-file">
                          {renderMethod(guide.manual)}
                        </Fold>
                      )}
                    </div>
                  )}
                </>
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
  onCopy: PropTypes.func.isRequired,
  // Opens the new key dialog (a key of its own for an import link).
  onCreateKey: PropTypes.func
};
