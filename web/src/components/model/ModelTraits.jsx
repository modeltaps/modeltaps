import { Braces, Brain, Ear, Eye, FileText, Film, Wrench } from 'lucide-react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { MODEL_KINDS, kindTone, modelTraits, traitOf } from './traits';

// ==============================|| MODEL KIND TAG & TRAIT MARKS ||============================== //
// The kind (one per model) is a tinted text tag; the traits (several) are tinted icon squares named
// on hover, always in the same order so they line up from row to row. Colors: see traits.js.

const TRAIT_ICONS = {
  vision: Eye,
  audio_input: Ear,
  video_input: Film,
  pdf_input: FileText,
  tool_call: Wrench,
  reasoning: Brain,
  structured_output: Braces
};

// The tone lives in --tone so one class list serves every hue.
const TINT = 'bg-[color-mix(in_oklab,var(--tone)_16%,transparent)] text-[var(--tone)]';

export const kindLabel = (t, kind) =>
  kind === 'chat' ? t('modelInfoPage.kindOptions.chat') : t(`channels.models.kind.${kind}`, { defaultValue: kind });

// Text in an 18px tag: the UI fonts' ascent is much larger than their descent, so centered glyphs
// sit about half a pixel low; 1px of bottom padding lifts them to the optical middle (measured).
export const TAG_TEXT = 'inline-flex h-[18px] shrink-0 items-center rounded-[5px] px-1.5 pb-px text-[11px] font-medium leading-none';

// `kind` "chat" gets a neutral tag where every row is tagged (the catalog's type column); lists that
// tag only the exceptions pass "" for chat models and get nothing.
export function ModelKindTag({ kind, className }) {
  const { t } = useTranslation();
  if (kind !== 'chat' && !MODEL_KINDS.includes(kind)) return null;
  return (
    <span style={{ '--tone': kindTone(kind) }} className={cn(TAG_TEXT, TINT, className)}>
      {kindLabel(t, kind)}
    </span>
  );
}

ModelKindTag.propTypes = { kind: PropTypes.string, className: PropTypes.string };

// The tinted square alone, for places that name the trait in text next to it (filter chips).
const GLYPH_SIZES = { md: ['size-[18px] rounded-[5px]', 'size-3'], sm: ['size-4 rounded-[4px]', 'size-[11px]'] };

export function TraitGlyph({ trait, size = 'md', className, ...rest }) {
  const def = traitOf(trait);
  if (!def) return null;
  const Icon = TRAIT_ICONS[trait];
  return (
    <span
      style={{ '--tone': def.tone }}
      className={cn('inline-flex shrink-0 items-center justify-center', GLYPH_SIZES[size][0], TINT, className)}
      {...rest}
    >
      <Icon className={GLYPH_SIZES[size][1]} strokeWidth={2} />
    </span>
  );
}

TraitGlyph.propTypes = { trait: PropTypes.string.isRequired, size: PropTypes.oneOf(['md', 'sm']), className: PropTypes.string };

export function TraitMark({ trait, size, className }) {
  const { t } = useTranslation();
  const def = traitOf(trait);
  if (!def) return null;
  const label = t(def.label);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <TraitGlyph trait={trait} size={size} role="img" aria-label={label} className={cn('cursor-default', className)} />
      </TooltipTrigger>
      <TooltipContent className="px-2 py-1 text-xs">{label}</TooltipContent>
    </Tooltip>
  );
}

TraitMark.propTypes = { trait: PropTypes.string.isRequired, size: PropTypes.oneOf(['md', 'sm']), className: PropTypes.string };

// `traits` when the caller already has them, else derived from kind + input modalities + capabilities.
export function ModelTraitMarks({ traits, kind, inputModalities, capabilities, size = 'md', className }) {
  const list = traits || modelTraits({ kind, inputModalities, capabilities });
  if (list.length === 0) return null;
  return (
    <TooltipProvider delayDuration={150}>
      <span className={cn('inline-flex shrink-0 items-center', size === 'sm' ? 'gap-0.5' : 'gap-1', className)}>
        {list.map((trait) => (
          <TraitMark key={trait} trait={trait} size={size} />
        ))}
      </span>
    </TooltipProvider>
  );
}

ModelTraitMarks.propTypes = {
  traits: PropTypes.arrayOf(PropTypes.string),
  kind: PropTypes.string,
  inputModalities: PropTypes.oneOfType([PropTypes.array, PropTypes.string]),
  capabilities: PropTypes.oneOfType([PropTypes.array, PropTypes.string]),
  size: PropTypes.oneOf(['md', 'sm']),
  className: PropTypes.string
};
