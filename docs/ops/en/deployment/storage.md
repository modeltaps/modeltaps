---
title: "Image Storage"
layout: doc
outline: deep
lastUpdated: true
---

# Image Storage

The image APIs of Cloudflare Workers AI and Stability AI, and Gemini's image output, return image data instead of a link. By default Modeltaps keeps these images on the gateway itself and returns a temporary link on the site's own address, with no third-party storage. External storage can be used instead. Turned off, those responses carry base64: the image API answers with `b64_json` instead of `url`, and Gemini replies embed `data:image/png;base64,...` inline, so clients that only read the image URL get no image.

Upstreams such as DALL·E and Flux return image links themselves. **Also store images and videos upstreams return as links** (on by default) fetches each image once through the channel's proxy and stores it, so clients get a link on this site that works where the upstream host is blocked or slow; if the fetch fails, the upstream link is passed on. Turned off, those links are passed on as they are.

Midjourney task queries answer with gateway image links too, taken over the same way as the Kling videos below (fetched on the first download); an image on the channel's own host (a Midjourney proxy you run) is passed on.

For a finished Kling video task, the video links returned when the task is queried become `<server address>/media/...` too. Videos are not fetched ahead: the first download of the link fetches the video from the upstream through the channel's proxy, passing it on while writing it to the data directory, and later downloads read that copy (with several nodes, each node fetches its own). The link lives for 1 hour from the first query, with the same download limit and speed as images, and a video may be up to 512 MB. Videos are always kept by this gateway, never uploaded to external storage.

## Default: this gateway

Images are kept on the gateway, with links like `<server address>/media/<random name>.png`:

- a link expires after 1 hour and the file is deleted (cleanup runs every 10 minutes);
- each link can be downloaded 5 times; from the 6th on it answers `410`, so links cannot be passed around as a public image host;
- each download runs at 2 MB/s, and a node serves at most 16 downloads at once, answering `429` beyond that.

Links use **Platform admin → Site → Server address**, which must be an address callers can reach. Without Redis the files are written to `data/media` in the data directory (change it with `media_dir`), which suits a single node. With Redis the images are kept in Redis and expire there, so any node can serve any link; several nodes need Redis.

## Set it in the console

Choose a storage and fill in its fields under **Platform admin → Gateway settings → Requests & protocols → Image storage**. Saving applies at once without a restart, and other nodes pick it up with the settings sync. Secrets are stored encrypted and shown masked; leaving one empty keeps it.

**Test upload** uploads a small image with the form as it stands, saved or not, and shows its link and a thumbnail. With a secret left empty, the saved secret is reused only for the saved endpoint, bucket and access key ID.

One storage is in use at a time. External storage returns its own links, which are not bound by the download and speed limits above and are not deleted by the gateway:

| Storage | Required | Notes |
|---|---|---|
| This gateway (default) | Nothing | See above |
| S3 compatible | Endpoint, bucket, access key ID, access key secret | AWS S3, Cloudflare R2, MinIO, Backblaze B2 and others |
| Alibaba Cloud OSS | Endpoint, bucket, access key ID, access key secret | Returns signed links valid for 1 hour, so the bucket can stay private |
| SM.MS | API token | Free image host; images are public |
| Imgur | Client ID | Free image host; images are public |

S3 signing needs a region. Cloudflare R2 uses `auto`, so leave it empty; standard AWS S3, Backblaze B2 and Wasabi endpoints (such as `https://s3.us-west-2.amazonaws.com` or `https://s3.us-west-004.backblazeb2.com`) are derived automatically; for other providers enter the region from their documentation. Callers get a plain link under the public URL (`endpoint/bucket` when it is empty), so the bucket must allow public reads. The Expires header days only set the object's Expires header and do not delete files; configure a lifecycle rule in the storage console for cleanup.

Alibaba Cloud OSS uploads and signed links use V4 signatures (new accounts and buckets have rejected V1 since 2025), which need the bucket's region. It can be left empty for a standard endpoint such as `oss-cn-beijing.aliyuncs.com`; custom domains and transfer acceleration endpoints need it.

### Example: Cloudflare R2

R2 charges nothing for downloads, which suits image hosting.

1. Create a bucket under R2 in the Cloudflare dashboard and note the account ID on the overview page.
2. Under the bucket's **Public access** settings, connect a custom domain. The `r2.dev` subdomain also works but is rate limited and meant for testing.
3. Under **Manage R2 API Tokens**, create a token with **Object Read & Write** limited to this bucket. Copy the Access Key ID and Secret Access Key it shows; they are shown only once.

Fill in the image storage like this:

| Field | Value |
|---|---|
| Storage | S3 compatible |
| Endpoint | `https://<account ID>.r2.cloudflarestorage.com` (for a bucket in the EU jurisdiction, `https://<account ID>.eu.r2.cloudflarestorage.com`) |
| Region | Empty; `auto` is used |
| Bucket | The bucket name |
| Access key ID | The token's Access Key ID |
| Access key secret | The token's Secret Access Key |
| Public URL | The custom domain (such as `https://img.example.com`) or `https://pub-xxxx.r2.dev`. The R2 endpoint is not publicly readable, so this field is required |
| Expires header (days) | `0` |

Click **Test upload**; a thumbnail means the setup works, then save. To clean up old images, add a delete rule under the bucket's **Object lifecycle rules**, for example after 30 days.

## Upgrading from the config file

Older versions set image storage in the `storage:` section of `config.yaml` (or the matching `STORAGE_*` environment variables). The first start after the upgrade moves it into the console; the `storage:` section is not read afterwards and can be removed. Older versions could set several stores and tried them in no fixed order; the move keeps one, preferring S3, then Alibaba Cloud OSS, SM.MS and Imgur.
