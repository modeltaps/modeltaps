import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { PANEL } from 'routes/paths';
import { marked } from 'marked';
import { Loader2, Sparkles } from 'lucide-react';

import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { ModelCombobox } from '@/components/ui/model-combobox';
import { catalogEntries, catalogEntry, settingLabel, settingSearchTiers } from '@/components/settings/catalog';
import { settingHref } from '@/components/settings/SettingHelp';
import { modelTraits } from '@/components/model/traits';
import { sanitizeHtml } from 'utils/sanitize';
import usePlaygroundKey from 'views/panel/ApiCatalog/playground/usePlaygroundKey';
import useCatalogModels from 'views/panel/ApiCatalog/useCatalogModels';
import { cardTitleKey, groupLabelKey } from 'views/panel/Setting/registry';
import { askSettingsAssistant, withConsoleMatches } from './settingsAssistant';
import { normalizeSynonyms, scoreTiers } from './commandScore';

// ==============================|| CHROME — SETTINGS ASSISTANT DIALOG ||============================== //
// The answer to an "Ask AI" question from the command palette (see settingsAssistant.js): the
// question (editable), the model that answers (chat models that call tools; the last pick is
// remembered), what the assistant looked up, and the answer with links to the settings. It only
// reads, and each question is billed to the admin like a playground request.

const MODEL_STORAGE_KEY = 'settingsAssistant.model';

const ANSWER_CLASS =
  'text-sm leading-6 [&_a]:text-info [&_a]:underline [&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:font-mono [&_code]:text-[12px] ' +
  '[&_ol]:list-decimal [&_ol]:pl-5 [&_p]:my-2 [&_ul]:list-disc [&_ul]:pl-5 [&_li]:my-0.5 [&_strong]:font-semibold';

// Batch models answer hours later; they are left out.
const BATCH = /(^|[-_.])batch($|[-_.])/i;
// The first default: a fast, cheap model that calls tools well, when the site has one.
const PREFERRED = [/claude.*haiku/i, /gpt-5.*mini/i, /gemini.*flash(?!-lite)/i, /claude.*sonnet/i, /deepseek-chat/i];

export function defaultAssistantModel(names) {
  for (const pattern of PREFERRED) {
    const hit = names.find((name) => pattern.test(name));
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
  const [question, setQuestion] = useState(initialQuestion);
  const [model, setModel] = useState(() => localStorage.getItem(MODEL_STORAGE_KEY) || '');
  const [state, setState] = useState({ status: 'idle', steps: [], answer: '', error: '' });
  const abortRef = useRef(null);
  const askedRef = useRef(false);

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

  const ask = useCallback(async () => {
    const q = question.trim();
    if (!q || !model) return;
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    localStorage.setItem(MODEL_STORAGE_KEY, model);
    setState({ status: 'running', steps: [], answer: '', error: '' });
    try {
      const { answer } = await askSettingsAssistant({
        question: q,
        model,
        language: i18n.language,
        getKey: ensureKey,
        describe,
        augment: (name, args, text) => withConsoleMatches(name, args, text, match),
        signal: controller.signal,
        onStep: (step) => setState((s) => ({ ...s, steps: [...s.steps, step] }))
      });
      setState((s) => ({ ...s, status: 'done', answer: answer || t('settingsAssistant.noAnswer') }));
    } catch (err) {
      if (controller.signal.aborted) return;
      setState((s) => ({ ...s, status: 'error', error: err?.message || String(err) }));
    }
  }, [question, model, i18n.language, ensureKey, describe, match, t]);

  // The question from the palette is asked as soon as a model is chosen.
  useEffect(() => {
    if (askedRef.current || !model || !initialQuestion.trim()) return;
    askedRef.current = true;
    ask();
  }, [model, initialQuestion, ask]);

  useEffect(() => () => abortRef.current?.abort(), []);

  const html = useMemo(() => (state.answer ? sanitizeHtml(marked.parse(state.answer)) : ''), [state.answer]);

  // Links to this console open in place; others in a new tab. A listener on the answer, as its
  // links come from the rendered markdown (a link's Enter key clicks it too).
  const answerRef = useRef(null);
  useEffect(() => {
    const el = answerRef.current;
    if (!el) return undefined;
    const onClick = (e) => {
      const link = e.target.closest?.('a[href]');
      if (!link) return;
      // Only a web or console address is followed; anything else the model wrote goes nowhere.
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
  }, [html, navigate, onClose]);

  const running = state.status === 'running';
  const noModels = !modelsLoading && names.length === 0;

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Sparkles className="size-4" />
            {t('settingsAssistant.title')}
          </DialogTitle>
          <DialogDescription>{t('settingsAssistant.description')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              ask();
            }}
          >
            <Textarea
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  ask();
                }
              }}
              rows={2}
              placeholder={t('settingsAssistant.placeholder')}
              aria-label={t('settingsAssistant.placeholder')}
            />
            <div className="flex flex-wrap items-center gap-2">
              <div className="w-64">
                <ModelCombobox
                  id="settings-assistant-model"
                  value={model}
                  onChange={setModel}
                  onSelect={setModel}
                  options={names}
                  allowCreate={false}
                  placeholder={t('settingsAssistant.model')}
                  disabled={running || noModels}
                />
              </div>
              <Button type="submit" size="sm" disabled={running || !question.trim() || !model}>
                {running && <Loader2 className="animate-spin" />}
                {t('settingsAssistant.ask')}
              </Button>
            </div>
            {noModels && <p className="text-xs text-warning">{t('settingsAssistant.noModels')}</p>}
          </form>

          {state.steps.length > 0 && (
            // While it runs, the latest look-ups; once answered, folded into one line.
            <details open={running || undefined} className="text-xs text-muted-foreground" data-testid="assistant-steps">
              <summary className="cursor-pointer select-none">{t('settingsAssistant.steps', { count: state.steps.length })}</summary>
              <ul className="mt-1 space-y-0.5">
                {(running ? state.steps.slice(-3) : state.steps).map((step, i) => (
                  <li key={i} className="truncate font-mono">
                    {step.name} {step.args}
                  </li>
                ))}
              </ul>
            </details>
          )}
          {running && state.steps.length === 0 && <p className="text-sm text-muted-foreground">{t('settingsAssistant.thinking')}</p>}
          {state.status === 'error' && (
            <p className="text-sm text-destructive">{t('settingsAssistant.failed', { message: state.error })}</p>
          )}
          {html && (
            // The answer is model text, rendered through the sanitizer.
            // eslint-disable-next-line react/no-danger
            <div ref={answerRef} className={ANSWER_CLASS} dangerouslySetInnerHTML={{ __html: html }} data-testid="assistant-answer" />
          )}
        </DialogBody>
        <DialogFooter>
          <p className="text-xs text-muted-foreground">{t('settingsAssistant.footer')}</p>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

SettingsAssistantDialog.propTypes = {
  question: PropTypes.string.isRequired,
  onClose: PropTypes.func.isRequired
};
