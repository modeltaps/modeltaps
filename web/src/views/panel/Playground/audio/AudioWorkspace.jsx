import { useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import {
  AudioLines,
  Handshake,
  Headset,
  Leaf,
  Megaphone,
  Mic,
  Play,
  Podcast,
  SlidersHorizontal,
  Square,
  TriangleAlert,
  Upload
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import { AUDIO_ACCEPT, formatTimestamp } from 'views/panel/ApiCatalog/playground/audio';
import ChipPopover from '../components/ChipPopover';
import ConsolePage from '../components/ConsolePage';
import ModelChip from '../components/ModelChip';
import NoModelsCard from '../components/NoModelsCard';
import ParamsChip from '../components/ParamsChip';
import { ttsVoices } from '../shared/fieldSchema';
import { normalizeModel } from '../shared/modelIndex';
import AudioPlayerBar from './AudioPlayerBar';
import VoiceGallery from './VoiceGallery';
import { SPEECH_EXAMPLES, STT_PARAM_FIELDS, TTS_PARAM_FIELDS, speechSummary, transcribeSummary } from './audioFields';
import { canRecordAudio, isPlayableSpeechFormat, speechFileName } from './useAudioSession';

// ==============================|| PLAYGROUND — AUDIO WORKSPACE ||============================== //
// xAI Voice 页形态：H1 下一行分段 tab（文字转语音 / 语音转文字，URL 上的 ?tab= 由装配层管），
// tab 与页面标题同一左边线，内容居中一列。合成这边是「示例 chip → 波形播放条 → 大编辑卡」：
// 编辑卡顶部一行控件（模型 / 音色 / 参数 + 生成按钮），卡内就是 textarea，卡片撑满到主区底部；
// 页面没有贴底输入框、也没有设置栏。
// 转写这边同一列：同款播放条（录音 + 回放 + 上传，整条可拖放）+ 常驻结果卡（顶部控件行 + 结果 / 空态）。
// 没有可用模型时两个方向都把顶部说明换成 NoModelsCard，编辑区置灰；合成的控件行复述一遍原因，
// 转写的控件行不再复述（说明卡已写全原因与出口）。
// 只吃 props：会话、门禁与当前 tab 都由装配层（index.jsx）传入。

const sessionShape = PropTypes.object;

const CARD = 'space-y-3 rounded-2xl border border-border bg-card p-3 shadow-sm';

const SEG_CLASS = 'rounded-full px-3 py-1.5 text-sm font-medium transition-colors';

// 示例 chip 的图标与配色，按 SPEECH_EXAMPLES 的 id 取。
const EXAMPLE_ICONS = {
  support: { icon: Headset, className: 'text-sky-500' },
  sales: { icon: Handshake, className: 'text-emerald-500' },
  podcast: { icon: Podcast, className: 'text-violet-500' },
  announcement: { icon: Megaphone, className: 'text-amber-500' },
  meditation: { icon: Leaf, className: 'text-teal-500' }
};

function Segmented({ items, value, onChange, label }) {
  return (
    <div role="tablist" aria-label={label} className="flex flex-wrap items-center gap-1">
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          role="tab"
          aria-selected={item.id === value}
          onClick={() => onChange(item.id)}
          className={cn(SEG_CLASS, item.id === value ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground')}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}

Segmented.propTypes = {
  items: PropTypes.array.isRequired,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  label: PropTypes.string
};

function GateNotice({ gated, message, actionHref, actionLabel }) {
  if (!gated) return null;
  return (
    <span className="mr-auto inline-flex items-center gap-1.5 text-xs text-muted-foreground">
      <TriangleAlert className="size-3.5" />
      {message}
      {actionHref && (
        <Link to={actionHref} className="font-medium text-primary hover:underline">
          {actionLabel}
        </Link>
      )}
    </span>
  );
}

GateNotice.propTypes = { gated: PropTypes.bool, message: PropTypes.string, actionHref: PropTypes.string, actionLabel: PropTypes.string };

function RunButton({ running, disabled, label, stopLabel, onRun, onStop }) {
  return running ? (
    <Button size="sm" variant="outline" className="rounded-full" onClick={onStop}>
      <Square />
      {stopLabel}
    </Button>
  ) : (
    <Button size="sm" className="rounded-full" disabled={disabled} onClick={onRun}>
      <Play />
      {label}
    </Button>
  );
}

RunButton.propTypes = {
  running: PropTypes.bool,
  disabled: PropTypes.bool,
  label: PropTypes.string,
  stopLabel: PropTypes.string,
  onRun: PropTypes.func,
  onStop: PropTypes.func
};

function ErrorLine({ error }) {
  const { t } = useTranslation();
  if (!error) return null;
  return (
    <p className="text-xs text-destructive">
      {t(error.i18nKey)}
      {error.message ? ` · ${error.message}` : ''}
    </p>
  );
}

ErrorLine.propTypes = { error: PropTypes.object };

export default function AudioWorkspace({
  session,
  tab,
  onTabChange,
  gated = false,
  gateMessage,
  gateActionHref,
  gateActionLabel,
  noModels: speechNoModels,
  sttNoModels
}) {
  const { t } = useTranslation();
  const rows = useMemo(() => session.options.map(normalizeModel), [session.options]);

  const tts = tab === 'tts';
  // 转写方向有专门的说明（为什么没有转写模型、怎么补、用什么替代），缺省时退回通用文案。
  const noModels = tts ? speechNoModels : sttNoModels || speechNoModels;
  const running = session.status === 'running';
  const hasModels = session.options.length > 0;
  const runnable = !gated && !running && Boolean(session.model) && (tts ? session.input.trim() !== '' : Boolean(session.file));
  // 控件行那句话同一个位置：门禁优先，其次是「没有可用模型」（仅合成）。
  const gate = gated
    ? { gated: true, message: gateMessage, actionHref: gateActionHref, actionLabel: gateActionLabel }
    : {
        gated: tts && !hasModels,
        message: noModels?.description,
        actionHref: noModels?.actionHref,
        actionLabel: noModels?.actionLabel
      };

  // 模型 chip 两个方向同一颗：候选与当前选中都由会话按方向给（见 useAudioSession）。
  const modelChip = (
    <ModelChip
      items={rows}
      value={session.model?.id || ''}
      disabled={!hasModels}
      label={session.model?.id || t(hasModels ? 'playgroundConsole.model.pick' : 'playgroundConsole.model.none')}
      onChange={(id) => session.setSelectedModel(id)}
    />
  );

  const panelProps = { session, modelChip, gate, running, runnable, hasModels, noModels };

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-1">
      <Segmented
        label={t('playgroundConsole.modality.audio')}
        value={tab}
        onChange={onTabChange}
        items={[
          { id: 'tts', label: t('playgroundConsole.audio.tts') },
          { id: 'stt', label: t('playgroundConsole.audio.stt') }
        ]}
      />

      <ConsolePage className="min-h-0 flex-1" columnClassName="max-w-[46rem]">
        <div className="flex flex-1 flex-col gap-5">
          {tts ? <SpeechPanel {...panelProps} /> : <TranscribePanel {...panelProps} />}

          <ErrorLine error={session.error} />
        </div>
      </ConsolePage>
    </div>
  );
}

AudioWorkspace.propTypes = {
  session: sessionShape.isRequired,
  tab: PropTypes.oneOf(['tts', 'stt']).isRequired,
  onTabChange: PropTypes.func.isRequired,
  gated: PropTypes.bool,
  gateMessage: PropTypes.string,
  gateActionHref: PropTypes.string,
  gateActionLabel: PropTypes.string,
  noModels: PropTypes.shape({
    title: PropTypes.string,
    description: PropTypes.string,
    actionLabel: PropTypes.string,
    actionHref: PropTypes.string
  }),
  sttNoModels: PropTypes.shape({
    title: PropTypes.string,
    description: PropTypes.string,
    actionLabel: PropTypes.string,
    actionHref: PropTypes.string
  })
};

const noModelsShape = PropTypes.shape({
  title: PropTypes.string,
  description: PropTypes.string,
  actionLabel: PropTypes.string,
  actionHref: PropTypes.string
});

const panelShape = {
  session: sessionShape.isRequired,
  modelChip: PropTypes.node,
  gate: PropTypes.object,
  running: PropTypes.bool,
  runnable: PropTypes.bool,
  hasModels: PropTypes.bool,
  noModels: noModelsShape
};

// 合成：示例 chip 行 → 波形播放条 → 大编辑卡（控件行 + textarea）。
function SpeechPanel({ session, modelChip, gate, running, runnable, hasModels, noModels }) {
  const { t } = useTranslation();

  if (!hasModels) {
    return (
      <div className="flex flex-1 flex-col gap-4">
        <NoModelsCard
          title={noModels?.title}
          description={noModels?.description}
          actionLabel={noModels?.actionLabel}
          actionHref={noModels?.actionHref}
        />
        <SpeechEditor session={session} modelChip={modelChip} gate={gate} running={running} runnable={runnable} disabled />
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col gap-4">
      <div className="space-y-1 text-center">
        <p className="text-base font-medium text-foreground">{t('playgroundConsole.audio.pickExample')}</p>
        <p className="text-sm text-muted-foreground">{t('playgroundConsole.audio.customText')}</p>
      </div>

      <div className="flex flex-wrap justify-center gap-2">
        {SPEECH_EXAMPLES.map((id) => {
          const { icon: Icon, className } = EXAMPLE_ICONS[id] || {};
          return (
            <button
              key={id}
              type="button"
              disabled={running}
              onClick={() => session.setInput(t(`playgroundConsole.audio.examples.${id}.text`))}
              className="inline-flex h-9 items-center gap-2 rounded-full border border-border px-3.5 text-sm text-foreground transition-colors hover:bg-muted disabled:opacity-50"
            >
              {Icon && <Icon className={cn('size-4', className)} />}
              {t(`playgroundConsole.audio.examples.${id}.label`)}
            </button>
          );
        })}
      </div>

      <AudioPlayerBar
        src={session.ttsUrl}
        fileName={speechFileName(session.ttsFormat)}
        pending={running}
        playable={isPlayableSpeechFormat(session.ttsFormat)}
        labels={{
          play: t('playgroundConsole.audio.play'),
          pause: t('playgroundConsole.audio.pause'),
          seek: t('playgroundConsole.audio.seek'),
          download: t('playgroundConsole.audio.download', { format: session.ttsFormat }),
          pending: t('playgroundConsole.result.waiting'),
          unplayable: t('playgroundConsole.audio.unplayable', { format: session.ttsFormat.toUpperCase() })
        }}
      />

      <SpeechEditor session={session} modelChip={modelChip} gate={gate} running={running} runnable={runnable} />
    </div>
  );
}

SpeechPanel.propTypes = panelShape;

// 合成的大编辑卡：控件行 + textarea。没有可用模型时整卡置灰，控件行那句话由 GateNotice 给。
function SpeechEditor({ session, modelChip, gate, running, runnable, disabled = false }) {
  const { t } = useTranslation();
  const [voiceOpen, setVoiceOpen] = useState(false);
  const voiceChip = useRef(null);

  const focusVoiceChip = () => voiceChip.current?.querySelector('button')?.focus();

  const selectVoice = (voice) => {
    session.setValue('voice', voice);
    focusVoiceChip();
    setVoiceOpen(false);
  };

  return (
    <section className={cn(CARD, 'flex flex-1 flex-col')}>
      <div className="flex flex-wrap items-center gap-2">
        {modelChip}
        <div ref={voiceChip} className="contents">
          <ChipPopover
            side="bottom"
            open={voiceOpen}
            onOpenChange={setVoiceOpen}
            icon={<AudioLines className="size-3.5" />}
            label={session.values.voice}
            contentClassName="flex max-h-[360px] w-[288px] flex-col overflow-hidden p-2 max-sm:fixed max-sm:inset-x-4 max-sm:top-24 max-sm:mt-0 max-sm:w-auto"
          >
            <VoiceGallery
              voices={ttsVoices(session.model)}
              value={session.values.voice}
              onChange={selectVoice}
              onEscape={focusVoiceChip}
              className="min-h-0 flex-1"
            />
          </ChipPopover>
        </div>
        <ParamsChip
          side="bottom"
          schema={TTS_PARAM_FIELDS}
          values={session.values}
          model={session.model}
          onChange={session.setValue}
          icon={<SlidersHorizontal className="size-3.5" />}
          summary={speechSummary(session.values)}
        />
        <GateNotice {...gate} />
        <div className="ml-auto">
          <RunButton
            running={running}
            disabled={!runnable}
            label={t('playgroundConsole.audio.generate')}
            stopLabel={t('playgroundConsole.stop')}
            onRun={session.run}
            onStop={session.abort}
          />
        </div>
      </div>

      <label className="sr-only" htmlFor="playground-tts-input">
        {t('playgroundConsole.audio.input')}
      </label>
      <Textarea
        id="playground-tts-input"
        rows={10}
        value={session.input}
        disabled={running || disabled}
        placeholder={t('playgroundConsole.audio.inputPlaceholder')}
        onChange={(event) => session.setInput(event.target.value)}
        className="min-h-[12rem] flex-1 resize-none border-0 bg-transparent px-1 shadow-none focus-visible:ring-0"
      />
    </section>
  );
}

SpeechEditor.propTypes = { ...panelShape, disabled: PropTypes.bool };

// 选中的文件 / 录音换成可回放的 object URL；换文件或离开时回收。
function useFileUrl(file) {
  const [url, setUrl] = useState('');
  useEffect(() => {
    if (typeof Blob === 'undefined' || !(file instanceof Blob)) {
      setUrl('');
      return undefined;
    }
    const next = URL.createObjectURL(file);
    setUrl(next);
    return () => URL.revokeObjectURL(next);
  }, [file]);
  return url;
}

// 转写：与合成同款的播放条（左录音、中波形回放、右上传，整条可拖放）+ 常驻结果卡
// （顶部控件行：模型 / 参数 + 转写按钮；没有结果时卡内居中空态）。
// 录音只在浏览器支持 MediaRecorder 时出现（见 canRecordAudio）。
function TranscribePanel({ session, modelChip, gate, running, runnable, hasModels, noModels }) {
  const { t } = useTranslation();
  const [dragging, setDragging] = useState(false);
  const fileUrl = useFileUrl(session.file);
  const result = session.sttResult;
  const inputDisabled = running || !hasModels;

  const onDrop = (event) => {
    event.preventDefault();
    setDragging(false);
    if (inputDisabled) return;
    session.setFile(event.dataTransfer?.files?.[0] || null);
  };

  const recordButton = canRecordAudio() ? (
    <button
      type="button"
      disabled={inputDisabled}
      onClick={session.recording ? session.stopRecording : session.startRecording}
      aria-label={t(session.recording ? 'playgroundConsole.audio.stopRecord' : 'playgroundConsole.audio.startRecord')}
      title={t(session.recording ? 'playgroundConsole.audio.stopRecord' : 'playgroundConsole.audio.startRecord')}
      className={cn(
        'inline-flex size-9 shrink-0 items-center justify-center rounded-full border transition-colors disabled:opacity-30',
        session.recording
          ? 'animate-pulse border-destructive bg-destructive text-destructive-foreground'
          : 'border-border text-foreground hover:bg-muted'
      )}
    >
      {session.recording ? <Square className="size-3.5" /> : <Mic className="size-4" />}
    </button>
  ) : null;

  const uploadButton = (
    <label
      htmlFor="playground-stt-file"
      aria-disabled={inputDisabled}
      title={t('playgroundConsole.audio.uploadHint')}
      className={cn(
        'inline-flex h-8 shrink-0 cursor-pointer items-center gap-1.5 rounded-full border border-border px-3 text-sm text-foreground transition-colors hover:bg-muted max-sm:px-2',
        inputDisabled && 'pointer-events-none opacity-50'
      )}
    >
      <Upload className="size-4" />
      <span className="max-sm:sr-only">{t('playgroundConsole.audio.upload')}</span>
      <input
        id="playground-stt-file"
        className="sr-only"
        type="file"
        accept={AUDIO_ACCEPT}
        disabled={inputDisabled}
        onChange={(event) => session.setFile(event.target.files?.[0] || null)}
      />
    </label>
  );

  return (
    <div className="flex flex-1 flex-col gap-4">
      {hasModels ? (
        <div className="space-y-1 text-center">
          <p className="text-base font-medium text-foreground">{t('playgroundConsole.audio.sttTitle')}</p>
          <p className="text-sm text-muted-foreground">
            {t('playgroundConsole.audio.dropHint')} · {t('playgroundConsole.audio.uploadHint')}
          </p>
        </div>
      ) : (
        <NoModelsCard
          title={noModels?.title}
          description={noModels?.description}
          actionLabel={noModels?.actionLabel}
          actionHref={noModels?.actionHref}
        />
      )}

      <div className="space-y-1.5">
        <div
          onDragOver={(event) => {
            event.preventDefault();
            if (!inputDisabled) setDragging(true);
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
          className={cn('rounded-2xl transition-shadow', dragging && 'ring-2 ring-primary ring-offset-2 ring-offset-background')}
        >
          <AudioPlayerBar
            src={fileUrl}
            downloadable={false}
            leading={recordButton}
            trailing={uploadButton}
            labels={{
              play: t('playgroundConsole.audio.play'),
              pause: t('playgroundConsole.audio.pause'),
              seek: t('playgroundConsole.audio.seek')
            }}
          />
        </div>
        {session.file && <p className="truncate px-1 text-xs text-muted-foreground">{session.file.name}</p>}
        {session.fileError && <p className="px-1 text-xs text-destructive">{t(session.fileError)}</p>}
      </div>

      <section className={cn(CARD, 'flex flex-1 flex-col')}>
        <div className="flex flex-wrap items-center gap-2">
          {modelChip}
          <ParamsChip
            side="bottom"
            schema={STT_PARAM_FIELDS}
            values={session.values}
            model={session.model}
            onChange={session.setValue}
            icon={<SlidersHorizontal className="size-3.5" />}
            summary={transcribeSummary(session.values)}
          />
          <GateNotice {...gate} />
          <div className="ml-auto">
            <RunButton
              running={running}
              disabled={!runnable}
              label={t('playgroundConsole.audio.transcribe')}
              stopLabel={t('playgroundConsole.stop')}
              onRun={session.run}
              onStop={session.abort}
            />
          </div>
        </div>

        {result ? (
          <div className="space-y-2 px-1">
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className="font-medium text-foreground">{t('playgroundConsole.audio.transcript')}</span>
              {session.meta.map((fact) => (
                <span key={fact}>{fact}</span>
              ))}
            </div>
            <p className="whitespace-pre-wrap break-words text-sm">{result.text || t('playgroundConsole.result.empty')}</p>
            {result.segments.map((segment) => (
              <p key={segment.id} className="break-words text-xs text-muted-foreground">
                <span className="font-mono">
                  {formatTimestamp(segment.start)} – {formatTimestamp(segment.end)}
                </span>{' '}
                {segment.text}
              </p>
            ))}
          </div>
        ) : (
          <div className="flex min-h-[12rem] flex-1 items-center justify-center px-4 text-center text-sm text-muted-foreground">
            {t(running ? 'playgroundConsole.result.waiting' : 'playgroundConsole.audio.sttEmpty')}
          </div>
        )}
      </section>
    </div>
  );
}

TranscribePanel.propTypes = panelShape;
