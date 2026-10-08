---
title: "Extra Ratios"
layout: doc
outline: deep
lastUpdated: true
---

# Extra Ratios

Pricing for multimodal models is comparatively complex, so Modeltaps provides extra ratios on top of the base price.

The following extra ratios are currently supported:

- input_audio_tokens: input audio tokens (input price)
- output_audio_tokens: output audio tokens (output price)
- cached_tokens: cached tokens (input price)
- cached_write_tokens: cache write tokens (input price)
- cached_read_tokens: cache read tokens (input price)
- reasoning_tokens: reasoning tokens (output price)
- input_text_tokens: input text tokens (input price)
- output_text_tokens: output text tokens (output price)
- input_image_tokens: input image tokens (input price)

## Billing method

Each extra ratio is applied as a multiplier on the base price shown in parentheses above.

For example, `gpt-image-1` charges 10/M for image input, 40/M for image output and 5/M for text input.

To price it, set the price of `gpt-image-1` to the image input/output price, then add `"input_text_tokens": 0.5` to the extra ratios.

With `input_text_tokens` set, the text input price becomes 10 \* 0.5 = 5/M.

## Format

The format is as follows:

```json
{
  "gpt-4o-audio-preview": {
    "input_audio_tokens": 40,
    "output_audio_tokens": 20
  },
  "gpt-4o-audio-preview-2024-10-01": {
    "input_audio_tokens": 40,
    "output_audio_tokens": 20
  },
  "gpt-4o-audio-preview-2024-12-17": {
    "input_audio_tokens": 16,
    "output_audio_tokens": 8
  },
  "gpt-4o-mini-audio-preview": {
    "input_audio_tokens": 67,
    "output_audio_tokens": 34
  },
  "gpt-4o-mini-audio-preview-2024-12-17": {
    "input_audio_tokens": 67,
    "output_audio_tokens": 34
  },
  "gpt-4o-realtime-preview": {
    "input_audio_tokens": 20,
    "output_audio_tokens": 10
  },
  "gpt-4o-realtime-preview-2024-10-01": {
    "input_audio_tokens": 20,
    "output_audio_tokens": 10
  },
  "gpt-4o-realtime-preview-2024-12-17": {
    "input_audio_tokens": 8,
    "output_audio_tokens": 4
  },
  "gpt-4o-mini-realtime-preview": {
    "input_audio_tokens": 17,
    "output_audio_tokens": 8.4
  },
  "gpt-4o-mini-realtime-preview-2024-12-17": {
    "input_audio_tokens": 17,
    "output_audio_tokens": 8.4
  },
  "gemini-2.5-flash-preview-04-17": {
    "reasoning_tokens": 5.833
  },
  "gpt-image-1": {
    "input_text_tokens": 0.5
  }
}
```
