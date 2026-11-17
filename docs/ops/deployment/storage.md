---
title: "图片存储"
layout: doc
outline: deep
lastUpdated: true
---

# 图片存储

Cloudflare Workers AI、Stability AI 的图片接口和 Gemini 的图片输出只返回图片数据，不返回链接。配置图片存储后，Modeltaps 先把图片上传到存储，再把链接返回给调用方；未配置时这些响应以 base64 返回：图片接口给 `b64_json` 而不是 `url`，Gemini 的回复里直接嵌入 `data:image/png;base64,...`。只读取图片链接的客户端会拿不到图。

OpenAI、Azure 等自己返回图片链接的上游不经过图片存储。

## 在后台配置

在 **平台管理 → 网关设置 → 请求与协议 → 图片存储** 里选择存储并填写字段，保存后立即生效，无需重启；多节点部署会随设置同步。密钥加密保存，页面只显示掩码，留空保存表示不修改。

**测试上传** 按表单当前内容上传一张小图并显示链接和缩略图，不必先保存。密钥留空时只对已保存的 endpoint、bucket 和 AccessKey ID 复用已保存的密钥。

同一时间只使用一个存储：

| 存储 | 必填 | 说明 |
|---|---|---|
| S3 兼容存储 | Endpoint、Bucket、AccessKey ID、AccessKey Secret | AWS S3、Cloudflare R2、MinIO、Backblaze B2 等 |
| 阿里云 OSS | Endpoint、Bucket、AccessKey ID、AccessKey Secret | 返回 1 小时内有效的签名链接，bucket 可以保持私有 |
| SM.MS | API Token | 免费图床，图片公开 |
| Imgur | Client ID | 免费图床，图片公开 |

S3 签名需要地域。Cloudflare R2 用 `auto`，留空即可；AWS S3、Backblaze B2、Wasabi 的标准 endpoint（如 `https://s3.us-west-2.amazonaws.com`、`https://s3.us-west-004.backblazeb2.com`）会自动推导；其他厂商请按其文档填写。返回给调用方的是公开访问地址下的普通链接（未填写时为 `endpoint/bucket`），Bucket 需要允许公开读取。Expires 头天数只写入对象的 Expires 头，不会删除文件，自动清理请在存储后台配置生命周期规则。

阿里云 OSS 上传和签名链接使用 V4 签名（2025 年起新账号、新 Bucket 已不接受 V1 签名），V4 签名需要 Bucket 所在地域。endpoint 是 `oss-cn-beijing.aliyuncs.com` 这类标准地址时可以不填地域；自定义域名或传输加速 endpoint 必须填写。

## 从配置文件升级

旧版本在 `config.yaml` 的 `storage:` 段（或对应的 `STORAGE_*` 环境变量）里配置图床。升级后第一次启动时，这些设置会自动搬进后台，之后不再读取 `storage:` 段，可以删掉。旧版本可以同时配置多个图床并按不固定的顺序尝试；搬迁时只保留一个，优先级为 S3、阿里云 OSS、SM.MS、Imgur。
