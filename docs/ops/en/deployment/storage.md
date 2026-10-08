---
title: "Image Storage"
layout: doc
outline: deep
lastUpdated: true
---

# Image Storage

Some providers' image generation APIs do not return a URL. Once image storage is configured, Modeltaps uploads the image to the storage backend first and then returns the link.

When using `gemini` models that support image output, an incorrect image storage configuration causes image responses to fail. (The native Gemini API does not require this configuration.)

You can configure multiple storage backends; if an upload fails, the system automatically falls back to the next one.

```yaml
storage: # Storage settings (optional; mainly for image generation — some providers only return base64 images, and this lets Modeltaps return a URL instead)
  smms: # sm.ms image storage
    secret: "" # sm.ms API key
  imgur:
    client_id: "" # imgur client_id
  alioss: # Alibaba Cloud OSS object storage
    endpoint: "" # Endpoint (regional node), e.g. oss-cn-beijing.aliyuncs.com
    bucketName: "" # Bucket name, e.g. zerodeng-superai
    accessKeyId: "" # Alibaba Cloud access key ID, from the RAM section of the console
    accessKeySecret: "" # Alibaba Cloud access key secret, from the RAM section of the console
  s3: # AWS S3 protocol
    endpoint: "" # Endpoint (regional node), e.g. https://xxxxxx.r2.cloudflarestorage.com
    cdnurl: "" # Public access domain, e.g. https://pub-xxxxx.r2.dev; falls back to endpoint when unset
    bucketName: "" # Bucket name, e.g. zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```
