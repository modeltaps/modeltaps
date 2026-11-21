import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { PANEL } from 'routes/paths';
import { marked } from 'marked';
import { ArrowUp, Square } from 'lucide-react';

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Combobox } from '@/components/ui/combobox';
import { catalogEntries, catalogEntry, settingLabel, settingSearchTiers } from '@/components/settings/catalog';
import { settingHref } from '@/components/settings/SettingHelp';
import { modelTraits } from '@/components/model/traits';
import { sanitizeHtml } from 'utils/sanitize';
import usePlaygroundKey from 'views/panel/ApiCatalog/playground/usePlaygroundKey';
import useCatalogModels from 'views/panel/ApiCatalog/useCatalogModels';
import { cardTitleKey, groupLabelKey } from 'views/panel/Setting/registry';
import { askSettingsAssistant, newConversation, withConsoleMatches } from './settingsAssistant';
import AssistantProposal from './AssistantProposal';
import { normalizeSynonyms, scoreTiers } from './commandScore';

// ==============================|| CHROME — SETTINGS ASSISTANT DIALOG ||============================== //
// The settings assistant from the command palette, as a conversation (see settingsAssistant.js): the questions and
// answers above, the composer at the bottom with the model on the left (chat models that call
// tools; the last pick is remembered) and send on the right. Each answer says what the assistant
// looked up, links to the settings, and may carry a change plan the admin reviews and applies
// (AssistantProposal). Each question is billed to the admin like a playground request.

// v2: the model is remembered once the admin picks it; the old key held whatever was used last,
// mostly the earlier fast default.
const MODEL_STORAGE_KEY = 'settingsAssistant.model.v2';

// The model picker in the message box: just the model name, a rounded surface on hover or while
// the list is open, no field border.
const MODEL_TRIGGER =
  'h-8 w-auto max-w-64 cursor-pointer gap-1.5 rounded-full border-transparent bg-transparent px-2.5 text-[13px] text-muted-foreground transition-colors ' +
  'hover:bg-muted hover:text-foreground focus-visible:border-transparent focus-visible:bg-muted aria-expanded:border-transparent aria-expanded:bg-muted aria-expanded:text-foreground';

const ANSWER_CLASS =
  'text-sm leading-6 [&_a]:text-info [&_a]:underline [&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:font-mono [&_code]:text-[12px] ' +
  '[&_ol]:list-decimal [&_ol]:pl-5 [&_p]:my-2 [&_ul]:list-disc [&_ul]:pl-5 [&_li]:my-0.5 [&_strong]:font-semibold';

// Batch models answer hours later; they are left out.
const BATCH = /(^|[-_.])batch($|[-_.])/i;
// Models offered first, best first: the default is a strong model that reads several settings and
// reasons about a problem well (a fast one missed the setting made for the case and proposed
// switching protections off). Questions are rare, so quality is worth the price. Faster ones follow.
const PREFERRED = [
  /claude.*sonnet/i,
  /^gpt-5(?!.*(mini|nano|codex|chat|audio|realtime|search|image))/i,
  /gemini.*pro(?!.*image)/i,
  /claude.*haiku/i,
  /gpt-5.*mini/i,
  /gemini.*flash(?!-lite)/i,
  /deepseek-chat/i
];

// The newest model of a family: the highest version sorts last.
const newest = (names, pattern) =>
  names.filter((name) => pattern.test(name)).sort((a, b) => b.localeCompare(a, undefined, { numeric: true }))[0];

// The models offered first in the picker: the last one used, then the preferred ones on this site.
export function recommendedModels(names, last) {
  const picks = [];
  if (last && names.includes(last)) picks.push(last);
  // One model per family: a family the last model already belongs to adds no second one.
  for (const pattern of PREFERRED) {
    if (picks.some((name) => pattern.test(name))) continue;
    const hit = newest(names, pattern);
    if (hit) picks.push(hit);
  }
  return picks.slice(0, 5);
}

export function defaultAssistantModel(names) {
  for (const pattern of PREFERRED) {
    const hit = newest(names, pattern);
    if (hit) return hit;
  }
  return names[0] || '';
}

function useToolModels() {
  const { models, loading } = useCatalogModels();
  const names = useMemo(
    () =>
      models
        .filter(
          (m) =>
            !m.kind &&
            !BATCH.test(m.id) &&
            modelTraits({ inputModalities: m.info.inputModalities, capabilities: m.info.capabilities }).includes('tool_call')
        )
        .map((m) => m.id),
    [models]
  );
  return { names, loading };
}

export default function SettingsAssistantDialog({ question: initialQuestion, onClose }) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { ensureKey } = usePlaygroundKey();
  const { names, loading: modelsLoading } = useToolModels();
  const [draft, setDraft] = useState('');
  const [model, setModel] = useState(() => localStorage.getItem(MODEL_STORAGE_KEY) || '');
  // Only a model the admin picks is remembered; otherwise the default follows the preferred list.
  const pickModel = (name) => {
    setModel(name);
    if (name) localStorage.setItem(MODEL_STORAGE_KEY, name);
  };
  // The conversation as shown: { role: 'user', text } and { role: 'assistant', status, steps, answer, error, proposal }.
  const [turns, setTurns] = useState([]);
  // The conversation as the model reads it (with its tool calls).
  const historyRef = useRef(null);
  const abortRef = useRef(null);
  const askedRef = useRef(false);
  const listRef = useRef(null);

  // The remembered model when it can still be used, else a preferred one.
  useEffect(() => {
    if (modelsLoading || names.length === 0) return;
    if (!names.includes(model)) setModel(defaultAssistantModel(names));
  }, [modelsLoading, names, model]);

  // The console's names of a setting, given to the model with each tool result.
  const describe = useCallback(
    (key) => {
      const entry = catalogEntry(key);
      if (!entry) return null;
      const location =
        entry.scope === 'site'
          ? `${t(groupLabelKey(entry.group))} › ${t(cardTitleKey(entry.card), { defaultValue: entry.card })}`
          : t(`settingsUi.catalog.objects.${entry.scope}`, { defaultValue: entry.scope });
      return { label: settingLabel(t, key), location };
    },
    [t]
  );

  // The console's own search for a query in the admin's words (the catalog search of the tools only
  // reads English): the best few settings, for withConsoleMatches.
  const match = useCallback(
    (query) => {
      const table = (lng) => i18n.getResource(lng, 'translation', 'command.synonyms');
      const synonyms = normalizeSynonyms(table('en_US'), table(i18n.language));
      return catalogEntries()
        .map((entry) => ({ entry, score: scoreTiers(query, settingSearchTiers(t, entry.key), synonyms) }))
        .filter((x) => x.score > 0)
        .sort((a, b) => b.score - a.score)
        .slice(0, 8)
        .map(({ entry }) => ({ key: entry.key, console_url: settingHref(entry.key) }));
    },
    [t, i18n]
  );

  const running = turns.at(-1)?.status === 'running';
  const updateLast = (patch) => setTurns((list) => list.map((turn, i) => (i === list.length - 1 ? { ...turn, ...patch(turn) } : turn)));

  const send = useCallback(
    async (text) => {
      const question = text.trim();
      if (!question || !model || running) return;
      const controller = new AbortController();
      abortRef.current = controller;
      setDraft('');
      setTurns((list) => [...list, { role: 'user', text: question }, { role: 'assistant', status: 'running', steps: [] }]);
      try {
        const result = await askSettingsAssistant({
          question,
          history: historyRef.current || newConversation(i18n.language),
          model,
          language: i18n.language,
          getKey: ensureKey,
          describe,
          augment: (name, args, toolText) => withConsoleMatches(name, args, toolText, match),
          signal: controller.signal,
          onStep: (step) => updateLast((turn) => ({ steps: [...turn.steps, step] }))
        });
        historyRef.current = result.history;
        updateLast(() => ({ status: 'done', answer: result.answer || t('settingsAssistant.noAnswer'), proposal: result.proposal }));
      } catch (err) {
        updateLast(() => (controller.signal.aborted ? { status: 'stopped' } : { status: 'error', error: err?.message || String(err) }));
      }
    },
    [model, running, i18n.language, ensureKey, describe, match, t]
  );

  // The question from the palette is asked as soon as a model is chosen.
  useEffect(() => {
    if (askedRef.current || !model || !initialQuestion.trim()) return;
    askedRef.current = true;
    send(initialQuestion);
  }, [model, initialQuestion, send]);

  useEffect(() => () => abortRef.current?.abort(), []);

  // Follow the conversation as it grows.
  useEffect(() => {
    const el = listRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [turns]);

  // Links to this console open in place; others in a new tab; anything else nowhere. A listener on
  // the list, as the links come from the rendered markdown (a link's Enter key clicks it too).
  useEffect(() => {
    const el = listRef.current;
    if (!el) return undefined;
    const onClick = (e) => {
      const link = e.target.closest?.('[data-answer] a[href]');
      if (!link) return;
      if (!/^(https?:\/\/|\/)/i.test(link.getAttribute('href') || '')) {
        e.preventDefault();
        return;
      }
      let url;
      try {
        url = new URL(link.getAttribute('href'), window.location.origin);
      } catch {
        return;
      }
      if (url.pathname !== PANEL && !url.pathname.startsWith(`${PANEL}/`)) {
        link.setAttribute('target', '_blank');
        link.setAttribute('rel', 'noopener noreferrer');
        return;
      }
      e.preventDefault();
      onClose();
      navigate(`${url.pathname}${url.search}${url.hash}`);
    };
    el.addEventListener('click', onClick);
    return () => el.removeEventListener('click', onClick);
  }, [navigate, onClose]);

  const noModels = !modelsLoading && names.length === 0;
  const exampleList = t('settingsAssistant.examples', { returnObjects: true });
  const examples = Array.isArray(exampleList) ? exampleList : [];
  const modelGroups = useMemo(() => {
    const recommended = recommendedModels(names, localStorage.getItem(MODEL_STORAGE_KEY));
    const option = (id) => ({ value: id, label: id });
    return [
      { key: 'recommended', heading: t('settingsAssistant.recommended'), options: recommended.map(option) },
      { key: 'all', heading: t('settingsAssistant.allModels'), options: names.filter((n) => !recommended.includes(n)).map(option) }
    ].filter((g) => g.options.length > 0);
  }, [names, t]);

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="flex h-[80vh] max-w-2xl flex-col gap-0 p-0">
        <DialogHeader className="border-b border-border px-5 py-4">
          <DialogTitle>{t('settingsAssistant.title')}</DialogTitle>
          <DialogDescription>{t('settingsAssistant.description')}</DialogDescription>
        </DialogHeader>

        <div ref={listRef} className="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-4" data-testid="assistant-conversation">
          {turns.length === 0 && (
            <div className="space-y-3 pt-6">
              <p className="text-sm text-muted-foreground">{t('settingsAssistant.empty')}</p>
              <div className="flex flex-wrap gap-2">
                {examples.map((example) => (
                  <button
                    key={example}
                    type="button"
                    disabled={!model}
                    onClick={() => send(example)}
                    className="cursor-pointer rounded-full border border-border px-3 py-1.5 text-left text-[13px] transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {example}
                  </button>
                ))}
              </div>
            </div>
          )}
          {turns.map((turn, i) =>
            turn.role === 'user' ? (
              <div key={i} className="flex justify-end">
                <p className="max-w-[85%] whitespace-pre-wrap rounded-2xl bg-muted px-3.5 py-2 text-sm">{turn.text}</p>
              </div>
            ) : (
              <AssistantTurn key={i} turn={turn} describe={describe} />
            )
          )}
        </div>

        <form
          className="space-y-2 border-t border-border px-5 py-3"
          onSubmit={(e) => {
            e.preventDefault();
            send(draft);
          }}
        >
          <div className="rounded-2xl border border-border bg-field px-3 pb-2 pt-2 focus-within:border-ring">
            <Textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  send(draft);
                }
              }}
              rows={2}
              placeholder={t(turns.length ? 'settingsAssistant.followUp' : 'settingsAssistant.placeholder')}
              aria-label={t('settingsAssistant.placeholder')}
              className="min-h-0 resize-none border-0 bg-transparent p-0 shadow-none focus-visible:border-0 focus-visible:ring-0"
            />
            <div className="mt-1 flex items-center justify-between gap-2">
              <div className="-ml-2.5 min-w-0">
                <Combobox
                  id="settings-assistant-model"
                  value={model}
                  onValueChange={pickModel}
                  groups={modelGroups}
                  placeholder={t('settingsAssistant.model')}
                  searchPlaceholder={t('settingsAssistant.searchModels')}
                  emptyText={t('settingsAssistant.noMatch')}
                  disabled={running || noModels}
                  className={MODEL_TRIGGER}
                  contentClassName="w-72"
                />
              </div>
              {running ? (
                <Button
                  type="button"
                  size="icon"
                  className="size-8 rounded-full"
                  onClick={() => abortRef.current?.abort()}
                  aria-label={t('settingsAssistant.stop')}
                >
                  <Square className="size-3.5" />
                </Button>
              ) : (
                <Button
                  type="submit"
                  size="icon"
                  className="size-8 rounded-full"
                  disabled={!draft.trim() || !model}
                  aria-label={t('settingsAssistant.ask')}
                >
                  <ArrowUp className="size-4" />
                </Button>
              )}
            </div>
          </div>
          <p className="text-[11px] text-muted-foreground">{noModels ? t('settingsAssistant.noModels') : t('settingsAssistant.footer')}</p>
        </form>
      </DialogContent>
    </Dialog>
  );
}

SettingsAssistantDialog.propTypes = {
  question: PropTypes.string.isRequired,
  onClose: PropTypes.func.isRequired
};

// One answer: what was looked up (the latest while it runs, folded once answered), the answer and
// the change plan, if any.
function AssistantTurn({ turn, describe }) {
  const { t } = useTranslation();
  const running = turn.status === 'running';
  const html = useMemo(() => (turn.answer ? sanitizeHtml(marked.parse(turn.answer)) : ''), [turn.answer]);
  return (
    <div className="space-y-2">
      {turn.steps.length > 0 && (
        <details open={running || undefined} className="text-xs text-muted-foreground" data-testid="assistant-steps">
          <summary className="cursor-pointer select-none">{t('settingsAssistant.steps', { count: turn.steps.length })}</summary>
          <ul className="mt-1 space-y-0.5">
            {(running ? turn.steps.slice(-3) : turn.steps).map((step, i) => (
              <li key={i} className="truncate font-mono">
                {step.name} {step.args}
              </li>
            ))}
          </ul>
        </details>
      )}
      {running && turn.steps.length === 0 && <p className="text-sm text-muted-foreground">{t('settingsAssistant.thinking')}</p>}
      {turn.status === 'error' && <p className="text-sm text-destructive">{t('settingsAssistant.failed', { message: turn.error })}</p>}
      {turn.status === 'stopped' && <p className="text-sm text-muted-foreground">{t('settingsAssistant.stopped')}</p>}
      {html && (
        // The answer is model text, rendered through the sanitizer.
        // eslint-disable-next-line react/no-danger
        <div data-answer className={ANSWER_CLASS} dangerouslySetInnerHTML={{ __html: html }} data-testid="assistant-answer" />
      )}
      {turn.proposal?.length > 0 && <AssistantProposal changes={turn.proposal} describe={describe} />}
    </div>
  );
}

AssistantTurn.propTypes = { turn: PropTypes.object.isRequired, describe: PropTypes.func.isRequired };
