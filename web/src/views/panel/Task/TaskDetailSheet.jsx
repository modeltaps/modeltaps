import PropTypes from 'prop-types';
import { useState } from 'react';
import { X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import CodeBlock from '@/components/ui/code-block';
import { cn } from '@/lib/utils';
import { timestamp2string } from 'utils/common';
import { isSunoMusic } from './taskHelpers';

// ==============================|| TASK — ROW DETAIL SHEET ||============================== //
// Lightweight right-side slide-over (mirrors Log/LogDetailSheet). Shows core task
// metadata plus the raw result payload; suno MUSIC tasks render their media inline.

function Row({ label, value }) {
  if (value === undefined || value === null || value === '') return null;
  return (
    <div className="flex items-start justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium text-foreground">{value}</span>
    </div>
  );
}
Row.propTypes = { label: PropTypes.string, value: PropTypes.node };

function SunoMusic({ items, t }) {
  const [selected, setSelected] = useState(items[0]);
  if (!selected) return null;
  return (
    <div className="space-y-3">
      <div className="space-y-1">
        {items.map((it) => (
          <button
            key={it.id}
            type="button"
            onClick={() => setSelected(it)}
            className={cn(
              'flex w-full items-center gap-3 rounded-md border border-border p-2 text-left',
              it.id === selected.id ? 'bg-muted' : 'hover:bg-muted/50'
            )}
          >
            {it.image_url && <img src={it.image_url} alt={it.title} className="size-12 flex-shrink-0 rounded object-cover" />}
            <span className="min-w-0 truncate text-sm font-medium">{it.title}</span>
          </button>
        ))}
      </div>
      {selected.audio_url && (
        <div>
          <p className="mb-1 text-xs text-muted-foreground">{t('suno.music', { defaultValue: 'Music' })}</p>
          <audio controls src={selected.audio_url} className="w-full">
            <track kind="captions" />
          </audio>
        </div>
      )}
      {selected.video_url && (
        <div>
          <p className="mb-1 text-xs text-muted-foreground">{t('suno.video', { defaultValue: 'Video' })}</p>
          <video controls src={selected.video_url} className="w-full">
            <track kind="captions" />
          </video>
        </div>
      )}
      {selected.metadata?.prompt && (
        <div>
          <p className="mb-1 text-xs text-muted-foreground">{t('suno.lyrics', { defaultValue: 'Lyrics' })}</p>
          <pre className="overflow-x-auto whitespace-pre-wrap rounded-md bg-muted/50 p-3 text-xs text-muted-foreground">
            {selected.metadata.prompt}
          </pre>
        </div>
      )}
    </div>
  );
}
SunoMusic.propTypes = { items: PropTypes.array, t: PropTypes.func };

export default function TaskDetailSheet({ item, t, onClose }) {
  if (!item) return null;

  const suno = isSunoMusic(item) && Array.isArray(item.data) && item.data.length > 0;

  return (
    <div className="fixed inset-0 z-[1200]">
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={onClose} aria-hidden="true" />
      <div className="absolute inset-y-0 right-0 flex w-full max-w-md flex-col border-l border-border bg-card shadow-xl">
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <h2 className="text-base font-semibold">{t('logPage.detailLabel')}</h2>
          <Button variant="ghost" size="icon" aria-label={t('common.close')} onClick={onClose}>
            <X className="size-5" />
          </Button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <section className="mb-4">
            <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{t('taskPage.title')}</h3>
            <Row label={t('taskPage.task')} value={item.task_id} />
            <Row label={t('taskPage.subTime')} value={timestamp2string(item.submit_time)} />
            <Row label={t('taskPage.finishTime')} value={timestamp2string(item.finish_time)} />
            <Row label={t('taskPage.channel')} value={item.channel_id} />
            <Row label={t('taskPage.user')} value={item.user_id} />
            <Row label={t('taskPage.platform')} value={item.platform} />
            <Row label={t('taskPage.type')} value={item.action} />
            <Row label={t('taskPage.progress')} value={`${item.progress ?? 0}%`} />
            <Row label={t('taskPage.fail')} value={item.fail_reason} />
          </section>

          {suno ? (
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t('suno.music', { defaultValue: 'Music' })}
              </h3>
              <SunoMusic items={item.data} t={t} />
            </section>
          ) : item.data ? (
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t('suno.response', { defaultValue: 'Response' })}
              </h3>
              <CodeBlock language="json" showCopy={false} code={JSON.stringify(item.data, null, 2)} />
            </section>
          ) : null}
        </div>
      </div>
    </div>
  );
}

TaskDetailSheet.propTypes = {
  item: PropTypes.object,
  t: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired
};
