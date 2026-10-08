---
title: "價格更新"
layout: doc
outline: deep
lastUpdated: true
---

# 價格更新

Modeltaps 的模型價格統一存放在價格表中，由管理員在 `運營 --> 模型價格` 頁面維護。系統首次啟動時會用內建的預設價格表初始化，之後可選擇手動維護，或從價格服務批量同步。

## 手動維護

在 `運營 --> 模型價格` 頁面可以直接新增、編輯、批量修改和刪除模型價格。每條價格包含以下字段：

- **模型名稱**：需要與請求中的 `model` 一致，支援以 `*` 結尾的前綴匹配（例如 `gpt-4o*`）。
- **計費類型**：`tokens` 按 token 計費，`times` 按次計費。
- **模型歸屬**：即 `channel_type`，決定模型在「可用模型」頁面歸屬到哪個供應商。歸屬列表在 `運營 --> 模型歸屬` 維護，價格裡的歸屬 ID 必須與歸屬列表中的 ID 一致，否則頁面無法按供應商正確分組。
- **輸入 / 輸出倍率**：相對基礎額度的倍率。按次計費時兩者填相同值。
- **鎖定**：勾選後，批量同步的「覆蓋」模式不會修改該條價格，適合用於保護人工核定過的模型。
- **額外倍率**：快取讀寫、音訊等特殊 token 的倍率。
- **長上下文分檔**：輸入 token 超過閾值後，整次請求套用另一組輸入/輸出倍率。

## 從價格服務批量同步

點擊頁面上的 `更新價格`，填入一個返回價格表 JSON 的地址，拉取後會先列出「新增模型」與「價格變化」的對比，確認無誤再選擇同步方式：

- **僅新增**：只寫入系統中不存在的模型，已有價格不動。
- **覆蓋**：刪除並重建全部未鎖定的價格，使其與價格表完全一致。
- **僅更新**：只更新系統中已存在的模型，不新增。

::: tip 建議
生產環境建議先用「僅新增」，變更部分逐條人工核對後再改動，避免一次覆蓋打亂已經調好的倍率。
:::

同一個彈窗裡還提供了直接從模型目錄拉取直連渠道（OpenAI / Anthropic / Gemini 等）真實價格並換算為倍率的按鈕，拉取結果同樣進入上面的對比預覽，不會覆蓋已鎖定的價格。

### 價格表 JSON 格式

價格服務需要返回一個價格對象數組，或者一個帶 `data` 字段的對象（兩種都能解析）：

```json
[
  {
    "model": "gpt-4o",
    "type": "tokens",
    "channel_type": 1,
    "input": 1.25,
    "output": 5,
    "locked": false,
    "extra_ratios": {
      "cached_read": 0.1,
      "cached_write": 1.25
    },
    "long_context": {
      "threshold": 272000,
      "input_ratio": 2,
      "output_ratio": 1.5
    }
  },
  {
    "model": "mj_imagine",
    "type": "times",
    "channel_type": 34,
    "input": 50,
    "output": 50
  }
]
```

帶 `data` 包裝時：

```json
{
  "data": [{ "model": "gpt-4o", "type": "tokens", "channel_type": 1, "input": 1.25, "output": 5 }]
}
```

字段說明：

| 字段 | 類型 | 說明 |
| --- | --- | --- |
| `model` | string | 模型名稱，必填，可用 `*` 結尾做前綴匹配 |
| `type` | string | 計費類型，`tokens` 或 `times`，必填 |
| `channel_type` | int | 模型歸屬 ID，需與 `運營 --> 模型歸屬` 中的 ID 對應 |
| `input` | float | 輸入倍率，按次計費時即單次倍率 |
| `output` | float | 輸出倍率，按次計費時與 `input` 填相同值 |
| `locked` | bool | 是否鎖定，鎖定的價格不會被覆蓋模式修改 |
| `extra_ratios` | object | 額外倍率，鍵為倍率名（如 `cached_read`、`cached_write`），值為倍率 |
| `long_context` | object | 長上下文分檔，含 `threshold`、`input_ratio`、`output_ratio`；`threshold` 為 0 視為未啟用 |

## 指向自建價格服務

如需使用自行維護的價格表，可透過環境變數 `UPDATE_PRICE_SERVICE` 指定預設地址，價格頁的更新彈窗會自動填入該地址：

```bash
UPDATE_PRICE_SERVICE=https://prices.your-domain.com/prices.json
```

配合下面幾個環境變數還可以讓程式在啟動後週期性自動同步：

- `AUTO_PRICE_UPDATES`：是否開啟自動更新。
- `AUTO_PRICE_UPDATES_MODE`：自動更新模式，`add` / `overwrite` / `update` / `system`，預設 `system`（只用程式內建價格表初始化，不訪問價格服務）。
- `AUTO_PRICE_UPDATES_INTERVAL`：自動更新週期，單位分鐘。

各變數的完整說明見[環境變數](../deployment/env)。

::: warning 注意
自動更新會在無人確認的情況下改寫價格表。若倍率為人工核定，建議保持 `AUTO_PRICE_UPDATES_MODE=system`，並對重要模型勾選「鎖定」，僅在價格頁手動拉取、逐條核對後再同步。
:::
