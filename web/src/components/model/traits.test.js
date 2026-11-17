import { describe, expect, it } from 'vitest';

import { kindTone, modelKindOf, modelTraits, traitOf } from './traits';

describe('modelTraits', () => {
  it('lists inputs then capabilities in display order', () => {
    expect(modelTraits({ kind: '', inputModalities: ['text', 'file', 'image'], capabilities: ['reasoning', 'tool_call', 'odd'] })).toEqual([
      'vision',
      'pdf_input',
      'tool_call',
      'reasoning'
    ]);
  });

  it('reads JSON array strings as the catalog stores them', () => {
    expect(modelTraits({ kind: 'chat', inputModalities: '["audio"]', capabilities: '["structured_output"]' })).toEqual([
      'audio_input',
      'structured_output'
    ]);
  });

  it('skips inputs for models that are not chat models', () => {
    expect(modelTraits({ kind: 'transcription', inputModalities: ['audio'], capabilities: [] })).toEqual([]);
  });
});

describe('modelKindOf', () => {
  it('follows the catalog rules: endpoints, then outputs, then the name', () => {
    expect(modelKindOf({ name: 'elevenlabs/scribe-v2', endpoints: ['audio.transcription'] })).toBe('transcription');
    expect(modelKindOf({ name: 'gpt-image-2', endpoints: ['chat', 'responses'], outputModalities: ['image'] })).toBe('image');
    expect(modelKindOf({ name: 'openai/text-embedding-3-small' })).toBe('embedding');
    expect(modelKindOf({ name: 'claude-fable-5-2', endpoints: ['chat'], outputModalities: ['text'] })).toBe('');
  });
});

describe('tones', () => {
  it('gives a kind and a trait about the same medium the same hue', () => {
    expect(kindTone('image')).toBe(traitOf('vision').tone);
    expect(kindTone('transcription')).toBe(traitOf('audio_input').tone);
    expect(kindTone('video')).toBe(traitOf('video_input').tone);
  });
});
