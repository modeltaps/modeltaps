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
    region: "" # Region ID, e.g. cn-beijing; when empty it is derived from a standard endpoint (oss-<region>.aliyuncs.com). Required for custom domains and the acceleration endpoint
    bucketName: "" # Bucket name, e.g. zerodeng-superai
    accessKeyId: "" # Alibaba Cloud access key ID, from the RAM section of the console
    accessKeySecret: "" # Alibaba Cloud access key secret, from the RAM section of the console
  s3: # AWS S3 protocol
    endpoint: "" # Endpoint (regional node), e.g. https://xxxxxx.r2.cloudflarestorage.com
    region: "" # Signing region, e.g. us-east-1; when empty it is derived from AWS, Backblaze B2 and Wasabi endpoints, and is auto otherwise (Cloudflare R2)
    cdnurl: "" # Public access domain, e.g. https://pub-xxxxx.r2.dev; falls back to endpoint when unset
    bucketName: "" # Bucket name, e.g. zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```

Alibaba Cloud OSS uploads and signed links use V4 signatures (accounts and buckets created since 2025 no longer accept V1 signatures), and V4 needs the region of the bucket. You can leave `region` empty when the endpoint is a standard one such as `oss-cn-beijing.aliyuncs.com`; otherwise set it. Without it, startup logs an error and Alibaba Cloud OSS stays off.

S3 signatures need a region. Cloudflare R2 uses `auto`, so leave it empty there. Standard AWS S3, Backblaze B2 and Wasabi endpoints (such as `https://s3.us-west-2.amazonaws.com` or `https://s3.us-west-004.backblazeb2.com`) are derived automatically; for other providers set `region` as their documentation says. Callers get a plain link under `cdnurl`, so the bucket must allow public reads. `expirationDays` only sets the Expires header of the object and deletes nothing; configure a lifecycle rule at the provider to clean up old files.
