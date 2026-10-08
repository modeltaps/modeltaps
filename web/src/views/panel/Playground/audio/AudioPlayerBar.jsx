import { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { Download, Pause, Play } from 'lucide-react';

import { cn } from '@/lib/utils';
import { clock } from './audioFields';

// ==============================|| PLAYGROUND — AUDIO PLAYER BAR ||============================== //
// 合成结果的播放条：播放 / 暂停 + 波形进度 + 时间 + 下载。没有结果时是一条虚线占位，
// 时间显示 --:--，与 xAI Voice 页一致。波形是装饰：面板拿不到解码后的振幅，
// 用一组确定性条形（同一条形序列每次渲染都一样，SSR 与客户端不会打架）。
// 转写那边复用同一条：leading / trailing 插槽放录音与上传按钮，回放用户自己的音频时不给下载。
// 只吃 props：文案由上层传入，自己不引 i18n、不取数。

export const WAVE_BARS = Array.from({ length: 56 }, (_, index) => 0.3 + 0.7 * Math.abs(Math.sin(index * 1.7)));

export default function AudioPlayerBar({
  src = '',
  fileName = 'speech.mp3',
  pending = false,
  labels = {},
  leading = null,
  trailing = null,
  downloadable = true,
  playable = true
}) {
  const media = useRef(null);
  const [playing, setPlaying] = useState(false);
  const [time, setTime] = useState(0);
  const [duration, setDuration] = useState(0);

  // 换了一段新音频：进度与播放状态都回到起点（object URL 由会话层回收）。
  useEffect(() => {
    setPlaying(false);
    setTime(0);
    setDuration(0);
  }, [src]);

  const progress = duration > 0 ? Math.min(1, time / duration) : 0;
  // 放不了的格式（裸 PCM）：不挂 <audio>、播放与拖动置灰，只留下载与一句提示。
  const canPlay = Boolean(src) && playable;

  const toggle = () => {
    const element = media.current;
    if (!element) return;
    if (element.paused) element.play();
    else element.pause();
  };

  // 点波形跳转：按点击位置占整条的比例换算成秒。
  const seek = (event) => {
    const element = media.current;
    if (!element || !duration) return;
    const box = event.currentTarget.getBoundingClientRect();
    const ratio = Math.min(1, Math.max(0, (event.clientX - box.left) / box.width));
    element.currentTime = ratio * duration;
    setTime(element.currentTime);
  };

  return (
    <div
      className={cn(
        'flex items-center gap-3 rounded-2xl px-4 py-3',
        src ? 'border border-border bg-card' : 'border border-dashed border-border bg-muted/30'
      )}
    >
      {leading}

      <button
        type="button"
        disabled={!canPlay}
        onClick={toggle}
        aria-label={playing ? labels.pause : labels.play}
        title={playing ? labels.pause : labels.play}
        className="inline-flex size-9 shrink-0 items-center justify-center rounded-full bg-foreground text-background transition-opacity hover:opacity-90 disabled:opacity-30"
      >
        {playing ? <Pause className="size-4" /> : <Play className="size-4" />}
      </button>

      <button
        type="button"
        disabled={!canPlay}
        onClick={seek}
        aria-label={labels.seek}
        className="flex h-9 min-w-0 flex-1 items-center gap-[2px] disabled:cursor-default"
      >
        {WAVE_BARS.map((height, index) => (
          <span
            key={index}
            style={{ height: `${Math.round(height * 100)}%` }}
            className={cn(
              'w-full shrink rounded-full transition-colors',
              src && index / WAVE_BARS.length <= progress ? 'bg-foreground' : 'bg-muted-foreground/30'
            )}
          />
        ))}
      </button>

      {src && !playable ? (
        <span className="shrink-0 text-xs text-muted-foreground">{labels.unplayable}</span>
      ) : (
        <span className="shrink-0 font-mono text-xs tabular-nums text-muted-foreground">
          {src ? clock(time) : '--:--'} / {src && duration ? clock(duration) : '--:--'}
        </span>
      )}

      {src && downloadable && (
        <a
          href={src}
          download={fileName}
          aria-label={labels.download}
          title={labels.download}
          className="inline-flex size-8 shrink-0 items-center justify-center rounded-full border border-border text-muted-foreground transition-colors hover:bg-muted"
        >
          <Download className="size-4" />
        </a>
      )}

      {pending && !src && <span className="shrink-0 text-xs text-muted-foreground">{labels.pending}</span>}

      {trailing}

      {canPlay && (
        // 试听的是刚合成出来的音频，没有可挂的字幕轨。
        // eslint-disable-next-line jsx-a11y/media-has-caption
        <audio
          ref={media}
          src={src}
          className="hidden"
          onPlay={() => setPlaying(true)}
          onPause={() => setPlaying(false)}
          onEnded={() => setPlaying(false)}
          onTimeUpdate={(event) => setTime(event.currentTarget.currentTime)}
          onLoadedMetadata={(event) => setDuration(Number.isFinite(event.currentTarget.duration) ? event.currentTarget.duration : 0)}
        />
      )}
    </div>
  );
}

AudioPlayerBar.propTypes = {
  src: PropTypes.string,
  fileName: PropTypes.string,
  pending: PropTypes.bool,
  leading: PropTypes.node,
  trailing: PropTypes.node,
  downloadable: PropTypes.bool,
  playable: PropTypes.bool,
  labels: PropTypes.shape({
    play: PropTypes.string,
    pause: PropTypes.string,
    seek: PropTypes.string,
    download: PropTypes.string,
    pending: PropTypes.string,
    unplayable: PropTypes.string
  })
};
