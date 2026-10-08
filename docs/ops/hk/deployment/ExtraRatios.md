---
title: "擴展價格設定"
layout: doc
outline: deep
lastUpdated: true
---

# 擴展價格設定

多模態模型的價格設定較為複雜，因此系統提供了擴展價格設定功能。

目前支援以下擴展價格設定：

- input_audio_tokens：輸入音訊 Token (輸入價格)
- output_audio_tokens：輸出音訊 Token (輸出價格)
- cached_tokens：快取 Token (輸入價格)
- cached_write_tokens：快取寫入 Token (輸入價格)
- cached_read_tokens：快取讀取 Token (輸入價格)
- reasoning_tokens：推理 Token (輸出價格)
- input_text_tokens：輸入文字 Token (輸入價格)
- output_text_tokens：輸出文字 Token (輸出價格)
- input_image_tokens：輸入圖片 Token (輸入價格)

## 計費方法

按倍率計算，上方括號中標註的價格類型即為該項所綁定的基準價格。

例如：gpt-image-1 的圖片輸入價格為 10/M，圖片輸出價格為 40/M，文字輸入價格為 5/M。

定價時只需將 `gpt-image-1` 的價格設定為圖片的輸入 / 輸出價格，並在擴展價格中設定 `"input_text_tokens": 0.5`。

設定 `input_text_tokens` 後，文字輸入價格即為 10 \* 0.5 = 5/M。

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
