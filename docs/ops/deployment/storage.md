---
title: "图片存储"
layout: doc
outline: deep
lastUpdated: true
---

# 图片存储

Cloudflare Workers AI、Stability AI 的图片接口和 Gemini 的图片输出只返回图片数据，不返回链接。默认情况下，Modeltaps 把这些图片暂存在网关自己这里，返回本站地址下的临时链接，不需要任何第三方存储。也可以改用外部存储；关闭后这些响应以 base64 返回：图片接口给 `b64_json` 而不是 `url`，Gemini 的回复里直接嵌入 `data:image/png;base64,...`，只读取图片链接的客户端会拿不到图。

OpenAI、Azure 等自己返回图片链接的上游不经过图片存储。

## 默认：本网关

图片保存在网关上，链接形如 `<服务器地址>/media/<随机名>.png`：

- 1 小时后失效并自动删除（每 10 分钟清理一次）；
- 每个链接最多下载 5 次，第 6 次起返回 `410`，链接无法被当作公开图床转发；
- 单次下载限速 2 MB/s，每个节点同时最多 16 个下载，超出返回 `429`。

链接使用 **平台管理 → 站点 → 服务器地址**，必须填写调用方能访问的地址。没有 Redis 时文件写在数据目录的 `data/media`（可用 `media_dir` 修改），只适合单节点；启用 Redis 后图片存在 Redis 里并随过期时间自动删除，任一节点都能提供链接，多节点部署需要启用 Redis。

## 在后台配置

在 **平台管理 → 网关设置 → 请求与协议 → 图片存储** 里选择存储并填写字段，保存后立即生效，无需重启；多节点部署会随设置同步。密钥加密保存，页面只显示掩码，留空保存表示不修改。

**测试上传** 按表单当前内容上传一张小图并显示链接和缩略图，不必先保存。密钥留空时只对已保存的 endpoint、bucket 和 AccessKey ID 复用已保存的密钥。

同一时间只使用一个存储。外部存储返回的是存储自己的链接，不受上述下载次数和限速限制，也不会被网关删除：

| 存储 | 必填 | 说明 |
|---|---|---|
| 本网关（默认） | 无 | 见上文 |
| S3 兼容存储 | Endpoint、Bucket、AccessKey ID、AccessKey Secret | AWS S3、Cloudflare R2、MinIO、Backblaze B2 等 |
| 阿里云 OSS | Endpoint、Bucket、AccessKey ID、AccessKey Secret | 返回 1 小时内有效的签名链接，bucket 可以保持私有 |
| SM.MS | API Token | 免费图床，图片公开 |
| Imgur | Client ID | 免费图床，图片公开 |

S3 签名需要地域。Cloudflare R2 用 `auto`，留空即可；AWS S3、Backblaze B2、Wasabi 的标准 endpoint（如 `https://s3.us-west-2.amazonaws.com`、`https://s3.us-west-004.backblazeb2.com`）会自动推导；其他厂商请按其文档填写。返回给调用方的是公开访问地址下的普通链接（未填写时为 `endpoint/bucket`），Bucket 需要允许公开读取。Expires 头天数只写入对象的 Expires 头，不会删除文件，自动清理请在存储后台配置生命周期规则。

阿里云 OSS 上传和签名链接使用 V4 签名（2025 年起新账号、新 Bucket 已不接受 V1 签名），V4 签名需要 Bucket 所在地域。endpoint 是 `oss-cn-beijing.aliyuncs.com` 这类标准地址时可以不填地域；自定义域名或传输加速 endpoint 必须填写。

### 示例：Cloudflare R2

R2 不收下载流量费，适合做图床。

1. 在 Cloudflare 控制台的 R2 里创建 bucket，记下概览页上的账户 ID。
2. 在 bucket 设置的公开访问（Public access）里连接自定义域名；也可以开启 `r2.dev` 子域名，但它有限速，只适合测试。
3. 在 R2 的 API 令牌管理（Manage R2 API Tokens）里创建令牌，权限选对象读写（Object Read & Write），只授权这个 bucket。创建后显示的 Access Key ID 和 Secret Access Key 只出现一次，复制下来。

在图片存储里这样填写：

| 字段 | 填写 |
|---|---|
| 存储 | S3 兼容存储 |
| Endpoint | `https://<账户 ID>.r2.cloudflarestorage.com`（欧盟辖区的 bucket 用 `https://<账户 ID>.eu.r2.cloudflarestorage.com`） |
| 地域 | 留空，自动使用 `auto` |
| Bucket | bucket 名称 |
| AccessKey ID | 令牌的 Access Key ID |
| AccessKey Secret | 令牌的 Secret Access Key |
| 公开访问地址 | 自定义域名（如 `https://img.example.com`）或 `https://pub-xxxx.r2.dev`。R2 的 endpoint 不能公开访问，这一项必须填写 |
| Expires 头（天） | `0` |

点 **测试上传**，能看到缩略图即配置正确，再保存。要定期清理旧图，在 bucket 设置的对象生命周期规则（Object lifecycle rules）里添加删除规则，比如 30 天后删除。

## 从配置文件升级

旧版本在 `config.yaml` 的 `storage:` 段（或对应的 `STORAGE_*` 环境变量）里配置图床。升级后第一次启动时，这些设置会自动搬进后台，之后不再读取 `storage:` 段，可以删掉。旧版本可以同时配置多个图床并按不固定的顺序尝试；搬迁时只保留一个，优先级为 S3、阿里云 OSS、SM.MS、Imgur。
