---
title: "添加 VertexAI"
layout: doc
outline: deep
lastUpdated: true
---

# 添加 VertexAI

1. 创建服务凭证
   [打开此页面](https://console.cloud.google.com/iam-admin/serviceaccounts)，选择对应项目，然后点击“创建服务帐号”。
   <img width="1099" alt="创建服务帐号" src="/images/vertexai/step1-create-service-account.png">

2. 填写服务账号详情，自定义一个名称。
   <img width="604" alt="服务账号详情" src="/images/vertexai/step2-account-details.png">

3. 选择角色：在过滤器中分别输入 `Vertex AI User` 和 `Service Account Token Creator`，并选择这两个角色。

<img width="582" alt="过滤角色" src="/images/vertexai/step3-role-filter.png">
<img width="593" alt="选择角色" src="/images/vertexai/step3-role-select.png">

4. 点击“继续”，然后点击“完成”。

5. 点击刚创建的服务账号，进入“密钥”页签，选择“创建新密钥”，选择“JSON”，然后点击“创建”。
   <img width="598" alt="密钥页签" src="/images/vertexai/step5-keys-tab.png">
   <img width="454" alt="创建新密钥" src="/images/vertexai/step5-create-key.png">

<img width="386" alt="选择 JSON 密钥类型" src="/images/vertexai/step5-key-type-json.png">

6. 创建成功后下载 JSON 文件，在本地打开并将其全部内容复制到渠道的 `Key` 中。

7. 最后在[此页面](https://console.cloud.google.com/apis/library/iamcredentials.googleapis.com)启用 `IAM Service Account Credentials API`。

## 渠道设置

- **其他**：填 `Region|ProjectID`，或 `Region1|Region2|Region3|ProjectID`（每次请求随机选一个区域）。`Region` 可以是 `global`、多区域 `us`、`eu`，或具体区域如 `us-east5`。
- **Claude 模型**：Claude 模型由 Vertex AI 的 Anthropic 发布方提供。请先在 Vertex AI Model Garden 中为该项目启用对应模型，并选择提供该模型的区域（见 Google 文档）。发往 Claude 兼容端点（`/claude/v1/messages`）的请求会原样转发，提示缓存字段、工具等都会完整到达 Vertex；OpenAI 兼容的聊天端点同样可用。客户端发送的 `anthropic-beta` 中只转发 Vertex 接受的值；需要固定该请求头时，在渠道的模型请求头中设置 `anthropic-beta`。
- **渠道 API 地址**：留空则使用 Google 官方地址。自定义地址（例如代理）必须包含 6 个 `%s` 占位符，依次为 host、project、region、publisher、model、action，例如 `https://proxy.example.com/%s/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s`。旧版 5 个占位符（没有 publisher）的地址仍可用于 Gemini 模型。其他地址会直接报错而不是被忽略，因此流量不会绕过你的代理。
