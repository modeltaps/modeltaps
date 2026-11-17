---
title: "图床配置"
layout: doc
outline: deep
lastUpdated: true
---

# 图床配置

部分供应商的图片生成接口不返回 url，因此在配置图床后，系统会先将图片上传至图床，再返回链接。

使用 `gemini` 支持图像输出的模型时，若图床设置不正确，会导致图片返回失败（gemini 原生 API 接口无需配置）。

可以配置多个图床，上传失败后系统会自动使用下一个图床上传。

```yaml
storage: # 存储设置 (可选,主要用于图片生成，有些供应商不提供url，只能返回base64图片，设置后可以正常返回url格式的图片生成)
  smms: # sm.ms 图床设置
    secret: "" # sm.ms API 密钥
  imgur:
    client_id: "" # imgur client_id
  alioss: # 阿里云OSS对象存储
    endpoint: "" # Endpoint（地域节点）,比如oss-cn-beijing.aliyuncs.com
    region: "" # 地域 ID，比如 cn-beijing；留空时从标准 endpoint（oss-<地域>.aliyuncs.com）推导，自定义域名或传输加速 endpoint 必须填写
    bucketName: "" # Bucket名称，比如zerodeng-superai
    accessKeyId: "" # 阿里授权KEY,在阿里云后台用户RAM控制部分获取
    accessKeySecret: "" # 阿里授权SECRET,在阿里云后台用户RAM控制部分获取
  s3: # AwsS3协议
    endpoint: "" # Endpoint（地域节点）,比如https://xxxxxx.r2.cloudflarestorage.com
    region: "" # 签名地域，比如 us-east-1；留空时从 AWS、Backblaze B2、Wasabi 的 endpoint 推导，其他（如 Cloudflare R2）用 auto
    cdnurl: "" # 公共访问域名，比如https://pub-xxxxx.r2.dev，如果不配置则使用endpoint
    bucketName: "" # Bucket名称，比如zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```

阿里云 OSS 上传和签名链接使用 V4 签名（2025 年起新账号、新 Bucket 已不接受 V1 签名），V4 签名需要 Bucket 所在地域。endpoint 是 `oss-cn-beijing.aliyuncs.com` 这类标准地址时可以不填 `region`；否则必须填写，未填写时启动日志会报错并且不启用阿里云 OSS。

S3 签名需要地域。Cloudflare R2 用 `auto`，留空即可；AWS S3、Backblaze B2、Wasabi 的标准 endpoint（如 `https://s3.us-west-2.amazonaws.com`、`https://s3.us-west-004.backblazeb2.com`）会自动推导；其他厂商请按其文档填写 `region`。返回给调用方的是 `cdnurl` 下的普通链接，Bucket 需要允许公开读取；`expirationDays` 只写入对象的 Expires 头，不会删除文件，自动清理请在存储后台配置生命周期规则。
