---
title: "拡張価格設定"
layout: doc
outline: deep
lastUpdated: true
---

# 拡張価格設定

マルチモーダルモデルの価格設定は複雑になりがちなため、システムでは拡張価格設定の機能を提供しています。

現在サポートしている拡張価格設定は次のとおりです。

- input_audio_tokens：入力音声トークン（入力価格）
- output_audio_tokens：出力音声トークン（出力価格）
- cached_tokens：キャッシュトークン（入力価格）
- cached_write_tokens：キャッシュ書き込みトークン（入力価格）
- cached_read_tokens：キャッシュ読み取りトークン（入力価格）
- reasoning_tokens：推論トークン（出力価格）
- input_text_tokens：入力テキストトークン（入力価格）
- output_text_tokens：出力テキストトークン（出力価格）
- input_image_tokens：入力画像トークン（入力価格）

## 課金の計算方法

倍率で計算します。上記の括弧内に示した価格タイプが、その項目に紐づく基準価格です。

例：gpt-image-1 の画像入力価格が 10/M、画像出力価格が 40/M、テキスト入力価格が 5/M の場合を考えます。

価格設定では、`gpt-image-1` の価格として画像の入力 / 出力価格を設定し、拡張価格に `"input_text_tokens": 0.5` を指定するだけで済みます。

`input_text_tokens` を設定すると、テキスト入力価格は 10 \* 0.5 = 5/M となります。

## フォーマット

フォーマットは次のとおりです。

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
