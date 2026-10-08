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
