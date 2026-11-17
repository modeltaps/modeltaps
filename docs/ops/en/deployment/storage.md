---
title: "Image Storage"
layout: doc
outline: deep
lastUpdated: true
---

# Image Storage

The image APIs of Cloudflare Workers AI and Stability AI, and Gemini's image output, return image data instead of a link. With image storage set, Modeltaps uploads the image first and returns its link to the caller. Without it those responses carry base64: the image API answers with `b64_json` instead of `url`, and Gemini replies embed `data:image/png;base64,...` inline. Clients that only read the image URL get no image.

Upstreams that return image links themselves, such as OpenAI and Azure, do not use image storage.

## Set it in the console

Choose a storage and fill in its fields under **Platform admin → Gateway settings → Requests & protocols → Image storage**. Saving applies at once without a restart, and other nodes pick it up with the settings sync. Secrets are stored encrypted and shown masked; leaving one empty keeps it.

**Test upload** uploads a small image with the form as it stands, saved or not, and shows its link and a thumbnail. With a secret left empty, the saved secret is reused only for the saved endpoint, bucket and access key ID.

One storage is in use at a time:

| Storage | Required | Notes |
|---|---|---|
| S3 compatible | Endpoint, bucket, access key ID, access key secret | AWS S3, Cloudflare R2, MinIO, Backblaze B2 and others |
| Alibaba Cloud OSS | Endpoint, bucket, access key ID, access key secret | Returns signed links valid for 1 hour, so the bucket can stay private |
| SM.MS | API token | Free image host; images are public |
| Imgur | Client ID | Free image host; images are public |

S3 signing needs a region. Cloudflare R2 uses `auto`, so leave it empty; standard AWS S3, Backblaze B2 and Wasabi endpoints (such as `https://s3.us-west-2.amazonaws.com` or `https://s3.us-west-004.backblazeb2.com`) are derived automatically; for other providers enter the region from their documentation. Callers get a plain link under the public URL (`endpoint/bucket` when it is empty), so the bucket must allow public reads. The Expires header days only set the object's Expires header and do not delete files; configure a lifecycle rule in the storage console for cleanup.

Alibaba Cloud OSS uploads and signed links use V4 signatures (new accounts and buckets have rejected V1 since 2025), which need the bucket's region. It can be left empty for a standard endpoint such as `oss-cn-beijing.aliyuncs.com`; custom domains and transfer acceleration endpoints need it.

## Upgrading from the config file

Older versions set image storage in the `storage:` section of `config.yaml` (or the matching `STORAGE_*` environment variables). The first start after the upgrade moves it into the console; the `storage:` section is not read afterwards and can be removed. Older versions could set several stores and tried them in no fixed order; the move keeps one, preferring S3, then Alibaba Cloud OSS, SM.MS and Imgur.
