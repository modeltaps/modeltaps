---
title: "開啟按線路路由"
layout: doc
outline: deep
lastUpdated: true
---

# 開啟按線路路由

線路把每個公開模型按檔位分給承接它的渠道，帶暫停規則和故障切換。新裝的實例預設開啟。從舊版本升級上來的實例保持原樣：`ModelSupplyEnabled` 開啟之前，請求仍按渠道的模型列表和模型映射路由，和以前完全一樣，線路相關頁面只展示「開啟後會怎樣」，供應商頁也不能排渠道的呼叫順序。

先用自己的數據核對遷移會改甚麼，再開啟。下面每一步都用 root 賬號操作：登入網站後，在瀏覽器開發者工具的主控台裏發請求，用的就是目前的登入工作階段。

```js
// 後面幾步共用的小函數。
const api = (method, path, body) =>
  fetch(path, { method, headers: { 'Content-Type': 'application/json' }, body: body && JSON.stringify(body) }).then((r) => r.json());
```

## 1. 備份數據庫

先備一份能還原的數據（SQLite：停掉閘道後複製數據庫檔案；MySQL 或 PostgreSQL：匯出一份 dump）。遷移會寫入線路、目錄模型和價格。

## 2. 試運行

```js
const report = (await api('POST', '/api/model_supply/migrate', { dry_run: true })).data;
```

試運行不寫任何數據。繼續之前先看報告：

| 欄位 | 要核對甚麼 |
| --- | --- |
| `supplies_planned` | 會建多少條線路，每個渠道的每個模型一條。 |
| `new_models`、`new_aliases`、`status_changes` | 會新增或發佈的目錄模型。渠道列出的名字一旦發佈，客戶端就能看到。 |
| `billing_changes`、`price_copies`、`price_conflicts`、`plus_mappings` | 以後按公開模型而不是上游名計價的請求。價格有衝突的先改好。 |
| `unresolved_wildcards`、`skipped_entries`、`duplicate_entries` | 不會變成線路的渠道條目。到渠道上改，或者確認可以捨棄。 |
| `policy_gaps` | 目標模型將沒有線路的路由策略。 |
| `routing_differences` | 按用戶分組列出存取方式會變的模型，每一條都應該在預期之內。 |

每改一處就再試運行一次，直到報告裏只剩你想要的改動。

## 3. 遷移並開啟

在「平台管理 → 線路分配 → 線路路由與排序依據」裏開啟「按線路路由」。儲存之前，開關下方會顯示要新建多少條線路，以及切換後有多少個模型沒有可用線路。儲存時先遷移（只補還沒有的線路），再開啟。用接口也一樣：

```js
await api('PUT', '/api/option/', { key: 'ModelSupplyEnabled', value: 'true' });
```

遷移的結果應當和最後一次試運行一致。

## 4. 核對

- 用幾個真實的 API key 呼叫 `GET /v1/models`，除了報告裏列出的差異，清單應和開啟前一樣。
- 在模型的「線路」頁籤裏，請求預覽會顯示下一個請求發往哪個渠道。優先用預覽，少發測試請求：用無效金鑰發出的請求會觸發預設的憑據規則，把渠道暫停。
- 開啟後的第一個小時留意供應商頁和日誌。

## 關回去

在同一個開關上關掉並儲存，或者：

```js
await api('PUT', '/api/option/', { key: 'ModelSupplyEnabled', value: 'false' });
```

路由立刻回到渠道的模型列表。線路會保留，再次開啟不用重新遷移；這期間渠道有改動的話，還是先試運行一次。
