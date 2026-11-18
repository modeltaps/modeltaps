---
title: "圖片儲存"
layout: doc
outline: deep
lastUpdated: true
---

# 圖片儲存

Cloudflare Workers AI、Stability AI 的圖片介面和 Gemini 的圖片輸出只返回圖片資料，不返回連結。預設情況下，Modeltaps 把這些圖片暫存在網關自己這裡，返回本站位址下的臨時連結，不需要任何第三方儲存。也可以改用外部儲存；關閉後這些回應以 base64 返回：圖片介面給 `b64_json` 而不是 `url`，Gemini 的回覆裡直接嵌入 `data:image/png;base64,...`，只讀取圖片連結的用戶端會拿不到圖。

DALL·E、Flux 等上游會直接返回圖片連結。**轉存上游返回的圖片連結**（預設開啟）會經渠道代理下載一次並存進圖片儲存，用戶端拿到本站連結，上游網域在內地打不開或很慢時也能用；下載失敗時原樣返回上游連結。關閉後這些連結原樣透傳。

## 預設：本網關

圖片保存在網關上，連結形如 `<伺服器地址>/media/<隨機名>.png`：

- 1 小時後失效並自動刪除（每 10 分鐘清理一次）；
- 每個連結最多下載 5 次，第 6 次起返回 `410`，連結無法被當作公開圖床轉發；
- 單次下載限速 2 MB/s，每個節點同時最多 16 個下載，超出返回 `429`。

連結使用 **平台管理 → 站點 → 伺服器地址**，必須填寫呼叫方能存取的位址。沒有 Redis 時檔案寫在數據目錄的 `data/media`（可用 `media_dir` 修改），只適合單節點；啟用 Redis 後圖片存在 Redis 裡並隨過期時間自動刪除，任一節點都能提供連結，多節點部署需要啟用 Redis。

## 在後台設定

在 **平台管理 → 閘道設定 → 請求與協定 → 圖片儲存** 裡選擇儲存並填寫欄位，儲存後立即生效，無需重啟；多節點部署會隨設定同步。密鑰加密保存，頁面只顯示遮罩，留空儲存表示不修改。

**測試上傳** 按表單目前內容上傳一張小圖並顯示連結和縮圖，不必先儲存。密鑰留空時只對已儲存的 endpoint、bucket 和 AccessKey ID 沿用已儲存的密鑰。

同一時間只使用一個儲存。外部儲存返回的是儲存自己的連結，不受上述下載次數和限速限制，也不會被網關刪除：

| 儲存 | 必填 | 說明 |
|---|---|---|
| 本網關（預設） | 無 | 見上文 |
| S3 相容儲存 | Endpoint、Bucket、AccessKey ID、AccessKey Secret | AWS S3、Cloudflare R2、MinIO、Backblaze B2 等 |
| 阿里雲 OSS | Endpoint、Bucket、AccessKey ID、AccessKey Secret | 返回 1 小時內有效的簽名連結，bucket 可以保持私有 |
| SM.MS | API Token | 免費圖床，圖片公開 |
| Imgur | Client ID | 免費圖床，圖片公開 |

S3 簽名需要地域。Cloudflare R2 用 `auto`，留空即可；AWS S3、Backblaze B2、Wasabi 的標準 endpoint（如 `https://s3.us-west-2.amazonaws.com`、`https://s3.us-west-004.backblazeb2.com`）會自動推導；其他廠商請按其文件填寫。返回給呼叫方的是公開存取位址下的普通連結（未填寫時為 `endpoint/bucket`），Bucket 需要允許公開讀取。Expires 標頭天數只寫入物件的 Expires 標頭，不會刪除檔案，自動清理請在儲存後台設定生命週期規則。

阿里雲 OSS 上傳和簽名連結使用 V4 簽名（2025 年起新帳戶、新 Bucket 已不接受 V1 簽名），V4 簽名需要 Bucket 所在地域。endpoint 是 `oss-cn-beijing.aliyuncs.com` 這類標準位址時可以不填地域；自訂網域或傳輸加速 endpoint 必須填寫。

### 示例：Cloudflare R2

R2 不收下載流量費，適合做圖床。

1. 在 Cloudflare 控制台的 R2 裏建立 bucket，記下概覽頁上的帳戶 ID。
2. 在 bucket 設定的公開存取（Public access）裏連接自訂網域；也可以開啟 `r2.dev` 子網域，但它有限速，只適合測試。
3. 在 R2 的 API 權杖管理（Manage R2 API Tokens）裏建立權杖，權限選物件讀寫（Object Read & Write），只授權這個 bucket。建立後顯示的 Access Key ID 和 Secret Access Key 只出現一次，複製下來。

在圖片儲存裏這樣填寫：

| 欄位 | 填寫 |
|---|---|
| 儲存 | S3 相容儲存 |
| Endpoint | `https://<帳戶 ID>.r2.cloudflarestorage.com`（歐盟轄區的 bucket 用 `https://<帳戶 ID>.eu.r2.cloudflarestorage.com`） |
| 地域 | 留空，自動使用 `auto` |
| Bucket | bucket 名稱 |
| AccessKey ID | 權杖的 Access Key ID |
| AccessKey Secret | 權杖的 Secret Access Key |
| 公開存取位址 | 自訂網域（如 `https://img.example.com`）或 `https://pub-xxxx.r2.dev`。R2 的 endpoint 不能公開存取，這一項必須填寫 |
| Expires 標頭（天） | `0` |

按 **測試上傳**，能看到縮圖即設定正確，再儲存。要定期清理舊圖，在 bucket 設定的物件生命週期規則（Object lifecycle rules）裏加入刪除規則，例如 30 天後刪除。

## 從設定檔升級

舊版本在 `config.yaml` 的 `storage:` 段（或對應的 `STORAGE_*` 環境變數）裡設定圖床。升級後第一次啟動時，這些設定會自動搬進後台，之後不再讀取 `storage:` 段，可以刪掉。舊版本可以同時設定多個圖床並按不固定的順序嘗試；搬遷時只保留一個，優先順序為 S3、阿里雲 OSS、SM.MS、Imgur。
