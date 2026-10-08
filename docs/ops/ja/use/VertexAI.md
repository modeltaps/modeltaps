---
title: "VertexAI の追加"
layout: doc
outline: deep
lastUpdated: true
---

# VertexAI の追加

1. サービス認証情報を作成します。
   [このページを開き](https://console.cloud.google.com/iam-admin/serviceaccounts)、対象のプロジェクトを選択して「サービス アカウントを作成」をクリックします。
   <img width="1099" alt="サービス アカウントを作成" src="/images/vertexai/step1-create-service-account.png">

2. サービス アカウントの詳細を入力し、任意の名前を設定します。
   <img width="604" alt="サービス アカウントの詳細" src="/images/vertexai/step2-account-details.png">

3. ロールを選択します。フィルターに `Vertex AI User` と `Service Account Token Creator` をそれぞれ入力し、この 2 つのロールを選択します。

<img width="582" alt="ロールを絞り込む" src="/images/vertexai/step3-role-filter.png">
<img width="593" alt="ロールを選択する" src="/images/vertexai/step3-role-select.png">

4. 「続行」をクリックし、続いて「完了」をクリックします。

5. 作成したサービス アカウントをクリックし、「鍵」タブを開いて「新しい鍵を作成」を選択し、「JSON」を選んで「作成」をクリックします。
   <img width="598" alt="鍵タブ" src="/images/vertexai/step5-keys-tab.png">
   <img width="454" alt="新しい鍵を作成" src="/images/vertexai/step5-create-key.png">

<img width="386" alt="JSON の鍵タイプを選択" src="/images/vertexai/step5-key-type-json.png">

6. 作成に成功したら JSON ファイルをダウンロードし、ローカルで開いてその内容をすべてチャネルの `Key` にコピーします。

7. 最後に [このページ](https://console.cloud.google.com/apis/library/iamcredentials.googleapis.com) で `IAM Service Account Credentials API` を有効にします。
