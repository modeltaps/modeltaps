import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Play, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Textarea } from '@/components/ui/textarea';
import { FormField } from '@/components/ui/form-field';
import { ModelCombobox } from '@/components/ui/model-combobox';
import { showError } from 'utils/common';
import { tryRoutingPolicy } from './routingPolicyApi';

// 置信度不足或决策失败的原因，用 routingPolicy.reasons.* 文案。
const REASON_TONE = {
  classified: 'default',
  below_threshold: 'secondary',
  decision_error: 'destructive',
  empty_input: 'secondary',
  pinned: 'secondary'
};

// 「试一试」：粘贴一条首条用户消息，用当前表单里的策略（可以还没保存）跑一次决策，只调决策模型。
// 可以再加几个候选决策模型并列比较，看哪个在真实消息上更稳。
export default function TryItPanel({ policy, modelOptions = [] }) {
  const { t } = useTranslation();
  const [text, setText] = useState('');
  const [candidates, setCandidates] = useState([]);
  const [candidateInput, setCandidateInput] = useState('');
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState(null);

  const addCandidate = (value) => {
    const name = (value || '').trim();
    if (!name || candidates.includes(name) || name === policy.decision_model || candidates.length >= 3) return;
    setCandidates([...candidates, name]);
    setCandidateInput('');
  };

  const run = async () => {
    if (!text.trim()) return;
    setRunning(true);
    try {
      const decisionModels = [policy.decision_model, ...candidates].filter(Boolean);
      const { success, message, data } = await tryRoutingPolicy({ policy, text, decisionModels });
      if (success) setResults(data || []);
      else showError(message);
    } catch (e) {
      // surfaced globally
    } finally {
      setRunning(false);
    }
  };

  const canRun = Boolean(text.trim()) && Boolean(policy.decision_model) && !running;

  return (
    <div className="space-y-5">
      <p className="text-sm text-muted-foreground">{t('routingPolicy.tryIt.intro')}</p>
      <FormField id="rp-try-text" label={t('routingPolicy.tryIt.message')} hint={t('routingPolicy.tryIt.messageHint')}>
        <Textarea
          id="rp-try-text"
          rows={6}
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={t('routingPolicy.tryIt.messagePlaceholder')}
        />
      </FormField>
      <FormField
        id="rp-try-candidates"
        label={t('routingPolicy.tryIt.candidates')}
        hint={t('routingPolicy.tryIt.candidatesHint', { model: policy.decision_model || '—' })}
      >
        <div className="space-y-2">
          <ModelCombobox
            id="rp-try-candidates"
            value={candidateInput}
            onChange={setCandidateInput}
            onSelect={addCandidate}
            options={modelOptions.filter((m) => m !== policy.decision_model && !candidates.includes(m))}
            placeholder={t('routingPolicy.tryIt.candidatesPlaceholder')}
            createLabel={(input) => t('routingPolicy.form.useTyped', { value: input })}
            disabled={candidates.length >= 3}
          />
          {candidates.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {candidates.map((name) => (
                <Badge key={name} variant="outline" className="gap-1 pr-1">
                  {name}
                  <button
                    type="button"
                    aria-label={t('common.delete')}
                    className="rounded-full p-0.5 hover:bg-muted"
                    onClick={() => setCandidates(candidates.filter((c) => c !== name))}
                  >
                    <X className="size-3" />
                  </button>
                </Badge>
              ))}
            </div>
          )}
        </div>
      </FormField>
      <Button type="button" onClick={run} disabled={!canRun}>
        <Play className="mr-1 size-4" />
        {running ? t('routingPolicy.tryIt.running') : t('routingPolicy.tryIt.run')}
      </Button>

      {results && (
        <div className="space-y-2" data-testid="try-results">
          {results.map((r) => (
            <div key={r.decision_model} className="rounded-md border border-border p-3 text-sm">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium">{r.decision_model}</span>
                <Badge variant={REASON_TONE[r.reason] || 'secondary'}>
                  {t(`routingPolicy.reasons.${r.reason}`, { defaultValue: r.reason })}
                </Badge>
              </div>
              <div className="mt-2 grid gap-1 text-muted-foreground sm:grid-cols-2">
                <span>
                  {t('routingPolicy.tryIt.label')}: <span className="text-foreground">{r.label || '—'}</span>
                </span>
                <span>
                  {t('routingPolicy.tryIt.confidence')}:{' '}
                  <span className="text-foreground">
                    {r.reason === 'classified' || r.reason === 'below_threshold' ? `${Math.round(r.confidence * 100)}%` : '—'}
                  </span>
                </span>
                <span>
                  {t('routingPolicy.tryIt.target')}: <span className="text-foreground">{r.target_model}</span>
                </span>
                <span>
                  {t('routingPolicy.tryIt.latency')}: <span className="text-foreground">{r.decision_ms} ms</span>
                </span>
              </div>
              {r.error && <p className="mt-2 break-words text-xs text-destructive">{r.error}</p>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
