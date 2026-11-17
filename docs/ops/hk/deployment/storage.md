---
title: "圖床設定"
layout: doc
outline: deep
lastUpdated: true
---

# 圖床設定

部分供應商的圖片生成介面不返回 url，因此在配置圖床後，系統會先將圖片上傳至圖床，再返回鏈接。

使用 `gemini` 支援圖像輸出的模型時，若圖床設定不正確，會導致圖片返回失敗（gemini 原生 API 介面無需配置）。

可以配置多個圖床，上傳失敗後系統會自動使用下一個圖床上傳。

```yaml
storage: # 儲存設定 (可選,主要用於圖片生成，有些供應商不提供url，只能返回base64圖片，設定後可以正常返回url格式的圖片生成)
  smms: # sm.ms 圖床設定
    secret: "" # sm.ms API 密鑰
  imgur:
    client_id: "" # imgur client_id
  alioss: # 阿里雲OSS對象存儲
    endpoint: "" # Endpoint（地域節點）,比如oss-cn-beijing.aliyuncs.com
    region: "" # 地域 ID，比如 cn-beijing；留空時從標準 endpoint（oss-<地域>.aliyuncs.com）推導，自訂網域或傳輸加速 endpoint 必須填寫
    bucketName: "" # Bucket名稱，比如zerodeng-superai
    accessKeyId: "" # 阿里授權KEY,在阿里雲後台用戶RAM控制部分獲取
    accessKeySecret: "" # 阿里授權SECRET,在阿里雲後台用戶RAM控制部分獲取
  s3: # AwsS3協議
    endpoint: "" # Endpoint（地域節點）,比如https://xxxxxx.r2.cloudflarestorage.com
    region: "" # 簽名地域，比如 us-east-1；留空時從 AWS、Backblaze B2、Wasabi 的 endpoint 推導，其他（如 Cloudflare R2）用 auto
    cdnurl: "" # 公共訪問域名，比如https://pub-xxxxx.r2.dev，不配置時使用 endpoint/bucketName（需要 Bucket 允許公開讀取）
    bucketName: "" # Bucket名稱，比如zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```

阿里雲 OSS 上傳和簽名連結使用 V4 簽名（2025 年起新帳戶、新 Bucket 已不接受 V1 簽名），V4 簽名需要 Bucket 所在地域。endpoint 是 `oss-cn-beijing.aliyuncs.com` 這類標準地址時可以不填 `region`；否則必須填寫，未填寫時啟動日誌會報錯並且不啟用阿里雲 OSS。

S3 簽名需要地域。Cloudflare R2 用 `auto`，留空即可；AWS S3、Backblaze B2、Wasabi 的標準 endpoint（如 `https://s3.us-west-2.amazonaws.com`、`https://s3.us-west-004.backblazeb2.com`）會自動推導；其他廠商請按其文檔填寫 `region`。返回給調用方的是 `cdnurl` 下的普通連結，Bucket 需要允許公開讀取；`expirationDays` 只寫入對象的 Expires 頭，不會刪除文件，自動清理請在存儲後台設定生命週期規則。
