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
    bucketName: "" # Bucket名称，比如zerodeng-superai
    accessKeyId: "" # 阿里授权KEY,在阿里云后台用户RAM控制部分获取
    accessKeySecret: "" # 阿里授权SECRET,在阿里云后台用户RAM控制部分获取
  s3: # AwsS3协议
    endpoint: "" # Endpoint（地域节点）,比如https://xxxxxx.r2.cloudflarestorage.com
    cdnurl: "" # 公共访问域名，比如https://pub-xxxxx.r2.dev，如果不配置则使用endpoint
    bucketName: "" # Bucket名称，比如zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```
