---
title: "扩展价格设置"
layout: doc
outline: deep
lastUpdated: true
---

# 扩展价格设置

多模态模型的价格设置较为复杂，因此系统提供了扩展价格设置功能。

目前支持以下扩展价格设置：

- input_audio_tokens：输入音频 Token (输入价格)
- output_audio_tokens：输出音频 Token (输出价格)
- cached_tokens：缓存 Token (输入价格)
- cached_write_tokens：缓存写入 Token (输入价格)
- cached_read_tokens：缓存读取 Token (输入价格)
- reasoning_tokens：推理 Token (输出价格)
- input_text_tokens：输入文字 Token (输入价格)
- output_text_tokens：输出文字 Token (输出价格)
- input_image_tokens：输入图片 Token (输入价格)

## 计费方法

按倍率计算，上方括号中标注的价格类型即为该项所绑定的基准价格。

例如：gpt-image-1 的图片输入价格为 10/M，图片输出价格为 40/M，文字输入价格为 5/M。

定价时只需将 `gpt-image-1` 的价格设置为图片的输入 / 输出价格，并在扩展价格中设置 `"input_text_tokens": 0.5`。

设置 `input_text_tokens` 后，文字输入价格即为 10 \* 0.5 = 5/M。

## 格式

格式如下：

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
