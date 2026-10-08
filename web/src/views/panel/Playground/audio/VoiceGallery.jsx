import { useEffect, useId, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, Search } from 'lucide-react';

import { cn } from '@/lib/utils';
import { TTS_VOICES, groupVoicesByLanguage } from '../shared/fieldSchema';

// ==============================|| PLAYGROUND — VOICE GALLERY ||============================== //
// 音色单列列表（xAI 音色浮层形态）：顶部搜索框按名称过滤，每行「名称 + 性别 · 用途」小字，
// 当前选中行右侧打勾；选中即生效，没有「下一步」。音色随当前模型（voices），按语言分组，
// 语言名用 Intl 按界面语言本地化，语言为空的归「其他」组；全部音色都没有语言时不画组标题。
// 编辑卡上的音色 chip 把它当浮层内容用，所以外层容器的排版由 className 交给使用方。
// 静态音色表暂无试听音频（previewUrl 恒为 null），所以不画试听按钮。
// 键盘：焦点在搜索框时 ↑/↓ 移动高亮（到头停住），Enter 选中高亮项；鼠标悬停同步高亮；Esc 交给 onEscape。
// 搜索框固定在顶部，只有列表滚动：使用方给外层定高（flex 列），列表自己 overflow。
// 已选项：弱底色 + 名称加粗；高亮：更深的底色。两者可叠加。

export function filterVoices(query, voices = TTS_VOICES) {
  const keyword = query.trim().toLowerCase();
  return keyword ? voices.filter((voice) => voice.id.toLowerCase().includes(keyword)) : voices;
}

// 打开时高亮当前音色，它不在列表里就高亮第一项；列表为空时没有高亮。
export function initialActiveIndex(voices, value) {
  if (!voices.length) return -1;
  return Math.max(0, voices.findIndex((voice) => voice.id === value));
}

// 语言码 → 界面语言下的语言名；Intl 不认识时原样显示语言码。
export function languageLabel(code, lng) {
  try {
    const locale = String(lng || 'en').replace('_', '-');
    return new Intl.DisplayNames([locale], { type: 'language' }).of(code) || code;
  } catch {
    return code;
  }
}

// 与 ModelPicker 的循环不同，这里到头停住。
export function clampActiveIndex(current, delta, length) {
  if (!length) return -1;
  return Math.min(length - 1, Math.max(0, current + delta));
}

export default function VoiceGallery({ voices: source = TTS_VOICES, value, onChange, onEscape, className }) {
  const { t, i18n } = useTranslation();
  const [query, setQuery] = useState('');
  const groups = groupVoicesByLanguage(filterVoices(query, source));
  const voices = groups.flatMap((group) => group.voices);
  const showHeaders = groups.some((group) => group.language);
  const groupId = (group) => `${idPrefix}-lang-${group.language || 'other'}`;
  const [active, setActive] = useState(() => initialActiveIndex(voices, value));
  const options = useRef([]);
  const idPrefix = useId();
  const optionId = (index) => `${idPrefix}-voice-${index}`;

  useEffect(() => {
    options.current[active]?.scrollIntoView?.({ block: 'nearest' });
  }, [active]);

  const onKeyDown = (event) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      setActive((current) => clampActiveIndex(current, event.key === 'ArrowDown' ? 1 : -1, voices.length));
      return;
    }
    if (event.key === 'Enter' && voices[active]) {
      event.preventDefault();
      onChange(voices[active].id);
    }
  };

  return (
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions -- 只截获 Esc 冒泡，交互元素是内部的输入框与选项
    <div
      className={cn('flex flex-col gap-1', className)}
      onKeyDown={(event) => {
        if (event.key === 'Escape') onEscape?.();
      }}
    >
      <label className="flex shrink-0 items-center gap-2 border-b border-border px-2 pb-2">
        <Search className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="sr-only">{t('playgroundConsole.audio.searchVoices')}</span>
        <input
          type="search"
          autoFocus
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setActive(filterVoices(event.target.value, source).length ? 0 : -1);
          }}
          onKeyDown={onKeyDown}
          role="combobox"
          aria-expanded
          aria-controls={`${idPrefix}-voices`}
          aria-autocomplete="list"
          aria-activedescendant={voices[active] ? optionId(active) : undefined}
          placeholder={t('playgroundConsole.audio.searchVoices')}
          className="w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
        />
      </label>

      <div id={`${idPrefix}-voices`} role="listbox" aria-label={t('playgroundConsole.fields.voice')}
        className="flex min-h-0 flex-1 flex-col overflow-y-auto py-1"
      >
        {groups.map((group) => (
          <div
            key={group.language || 'other'}
            role="group"
            aria-labelledby={showHeaders ? groupId(group) : undefined}
            className="flex flex-col"
          >
            {showHeaders && (
              <div id={groupId(group)} className="px-2 pb-1 pt-2 text-xs font-medium text-muted-foreground">
                {group.language ? languageLabel(group.language, i18n.language) : t('playgroundConsole.audio.otherLanguage')}
              </div>
            )}
            {group.voices.map((voice) => {
              const index = voices.indexOf(voice);
              const selected = voice.id === value;
              const facts = [
                voice.gender && t(`playgroundConsole.audio.gender.${voice.gender}`),
                ...(voice.tags || []).map((item) => t(`playgroundConsole.audio.tags.${item}`))
              ].filter(Boolean);
              return (
                <button
                  key={voice.id}
                  ref={(node) => {
                    options.current[index] = node;
                  }}
                  id={optionId(index)}
                  type="button"
                  role="option"
                  aria-selected={selected}
                  tabIndex={-1}
                  onClick={() => onChange(voice.id)}
                  onMouseEnter={() => setActive(index)}
                  className={cn(
                    'flex items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors',
                    index === active ? 'bg-accent' : selected && 'bg-muted/60'
                  )}
                >
                  <span className="min-w-0 flex-1">
                    <span className={cn('block truncate text-sm text-foreground', selected ? 'font-semibold' : 'font-medium')}>
                      {voice.id}
                    </span>
                    {facts.length > 0 && <span className="block truncate text-xs text-muted-foreground">{facts.join(' · ')}</span>}
                  </span>
                  {selected && <Check className="size-4 shrink-0 text-foreground" />}
                </button>
              );
            })}
          </div>
        ))}
        {voices.length === 0 && (
          <p className="px-2 py-3 text-center text-xs text-muted-foreground">{t('playgroundConsole.audio.noVoiceMatch')}</p>
        )}
      </div>
    </div>
  );
}

VoiceGallery.propTypes = {
  voices: PropTypes.arrayOf(
    PropTypes.shape({ id: PropTypes.string.isRequired, language: PropTypes.string, gender: PropTypes.string, tags: PropTypes.array })
  ),
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  onEscape: PropTypes.func,
  className: PropTypes.string
};
