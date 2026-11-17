---
title: "圖片儲存"
layout: doc
outline: deep
lastUpdated: true
---

# 圖片儲存

Cloudflare Workers AI、Stability AI 的圖片介面和 Gemini 的圖片輸出只返回圖片資料，不返回連結。設定圖片儲存後，Modeltaps 先把圖片上傳到儲存，再把連結返回給呼叫方；未設定時這些回應以 base64 返回：圖片介面給 `b64_json` 而不是 `url`，Gemini 的回覆裡直接嵌入 `data:image/png;base64,...`。只讀取圖片連結的用戶端會拿不到圖。

OpenAI、Azure 等自己返回圖片連結的上游不經過圖片儲存。

## 在後台設定

在 **平台管理 → 閘道設定 → 請求與協定 → 圖片儲存** 裡選擇儲存並填寫欄位，儲存後立即生效，無需重啟；多節點部署會隨設定同步。密鑰加密保存，頁面只顯示遮罩，留空儲存表示不修改。

**測試上傳** 按表單目前內容上傳一張小圖並顯示連結和縮圖，不必先儲存。密鑰留空時只對已儲存的 endpoint、bucket 和 AccessKey ID 沿用已儲存的密鑰。

同一時間只使用一個儲存：

| 儲存 | 必填 | 說明 |
|---|---|---|
| S3 相容儲存 | Endpoint、Bucket、AccessKey ID、AccessKey Secret | AWS S3、Cloudflare R2、MinIO、Backblaze B2 等 |
| 阿里雲 OSS | Endpoint、Bucket、AccessKey ID、AccessKey Secret | 返回 1 小時內有效的簽名連結，bucket 可以保持私有 |
| SM.MS | API Token | 免費圖床，圖片公開 |
| Imgur | Client ID | 免費圖床，圖片公開 |

S3 簽名需要地域。Cloudflare R2 用 `auto`，留空即可；AWS S3、Backblaze B2、Wasabi 的標準 endpoint（如 `https://s3.us-west-2.amazonaws.com`、`https://s3.us-west-004.backblazeb2.com`）會自動推導；其他廠商請按其文件填寫。返回給呼叫方的是公開存取位址下的普通連結（未填寫時為 `endpoint/bucket`），Bucket 需要允許公開讀取。Expires 標頭天數只寫入物件的 Expires 標頭，不會刪除檔案，自動清理請在儲存後台設定生命週期規則。

阿里雲 OSS 上傳和簽名連結使用 V4 簽名（2025 年起新帳戶、新 Bucket 已不接受 V1 簽名），V4 簽名需要 Bucket 所在地域。endpoint 是 `oss-cn-beijing.aliyuncs.com` 這類標準位址時可以不填地域；自訂網域或傳輸加速 endpoint 必須填寫。

## 從設定檔升級

舊版本在 `config.yaml` 的 `storage:` 段（或對應的 `STORAGE_*` 環境變數）裡設定圖床。升級後第一次啟動時，這些設定會自動搬進後台，之後不再讀取 `storage:` 段，可以刪掉。舊版本可以同時設定多個圖床並按不固定的順序嘗試；搬遷時只保留一個，優先順序為 S3、阿里雲 OSS、SM.MS、Imgur。
