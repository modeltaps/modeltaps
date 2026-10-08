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
