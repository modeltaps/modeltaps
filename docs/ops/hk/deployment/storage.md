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
    bucketName: "" # Bucket名稱，比如zerodeng-superai
    accessKeyId: "" # 阿里授權KEY,在阿里雲後台用戶RAM控制部分獲取
    accessKeySecret: "" # 阿里授權SECRET,在阿里雲後台用戶RAM控制部分獲取
  s3: # AwsS3協議
    endpoint: "" # Endpoint（地域節點）,比如https://xxxxxx.r2.cloudflarestorage.com
    cdnurl: "" # 公共訪問域名，比如https://pub-xxxxx.r2.dev，如果不配置則使用endpoint
    bucketName: "" # Bucket名稱，比如zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```
