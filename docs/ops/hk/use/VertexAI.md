---
title: "新增 VertexAI"
layout: doc
outline: deep
lastUpdated: true
---

# 新增 VertexAI

1. 建立服務憑證
   [開啟此頁面](https://console.cloud.google.com/iam-admin/serviceaccounts)，選擇對應項目，然後點擊“建立服務帳戶”。
   <img width="1099" alt="建立服務帳戶" src="/images/vertexai/step1-create-service-account.png">

2. 填寫服務帳戶詳情，自訂一個名稱。
   <img width="604" alt="服務帳戶詳情" src="/images/vertexai/step2-account-details.png">

3. 選擇角色：在過濾器中分別輸入 `Vertex AI User` 和 `Service Account Token Creator`，並選擇這兩個角色。

<img width="582" alt="過濾角色" src="/images/vertexai/step3-role-filter.png">
<img width="593" alt="選擇角色" src="/images/vertexai/step3-role-select.png">

4. 點擊“繼續”，然後點擊“完成”。

5. 點擊剛建立的服務帳戶，進入“密鑰”標籤頁，選擇“建立新密鑰”，選擇“JSON”，然後點擊“建立”。
   <img width="598" alt="密鑰標籤頁" src="/images/vertexai/step5-keys-tab.png">
   <img width="454" alt="建立新密鑰" src="/images/vertexai/step5-create-key.png">

<img width="386" alt="選擇 JSON 密鑰類型" src="/images/vertexai/step5-key-type-json.png">

6. 建立成功後下載 JSON 檔案，在本地開啟並將其全部內容複製到渠道的 `Key` 中。

7. 最後在[此頁面](https://console.cloud.google.com/apis/library/iamcredentials.googleapis.com)啟用 `IAM Service Account Credentials API`。

## 渠道設定

- **其他**：填 `Region|ProjectID`，或 `Region1|Region2|Region3|ProjectID`（每次請求隨機選一個區域）。`Region` 可以是 `global`、多區域 `us`、`eu`，或具體區域如 `us-east5`。
- **Claude 模型**：Claude 模型由 Vertex AI 的 Anthropic 發布方提供。請先在 Vertex AI Model Garden 中為該專案啟用對應模型，並選擇提供該模型的區域（見 Google 文件）。發往 Claude 相容端點（`/claude/v1/messages`）的請求會原樣轉發，提示快取欄位、工具等都會完整送達 Vertex；OpenAI 相容的聊天端點同樣可用。客戶端傳送的 `anthropic-beta` 中只轉發 Vertex 接受的值；需要固定該請求標頭時，在渠道的模型請求標頭中設定 `anthropic-beta`。
- **渠道 API 位址**：留空則使用 Google 官方位址。自訂位址（例如代理）必須包含 6 個 `%s` 佔位符，依次為 host、project、region、publisher、model、action，例如 `https://proxy.example.com/%s/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s`。舊版 5 個佔位符（沒有 publisher）的位址仍可用於 Gemini 模型。其他位址會直接報錯而不是被忽略，因此流量不會繞過你的代理。
