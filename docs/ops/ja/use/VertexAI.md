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

## チャネル設定

- **その他**：`Region|ProjectID`、または `Region1|Region2|Region3|ProjectID`（リクエストごとにリージョンをランダムに選択）を入力します。`Region` には `global`、マルチリージョンの `us`・`eu`、または `us-east5` のような個別のリージョンを指定できます。
- **Claude モデル**：Claude モデルは Vertex AI の Anthropic パブリッシャーが提供します。先に Vertex AI Model Garden でプロジェクトに対して該当モデルを有効にし、そのモデルを提供するリージョンを選んでください（Google のドキュメントを参照）。Claude 互換エンドポイント（`/claude/v1/messages`）へのリクエストはそのまま転送されるため、プロンプトキャッシュのフィールドやツールなどが変更されずに Vertex に届きます。OpenAI 互換のチャットエンドポイントも利用できます。クライアントが送る `anthropic-beta` のうち、Vertex が受け付ける値だけが転送されます。ヘッダーを固定したい場合は、チャネルのモデルヘッダーに `anthropic-beta` を設定してください。
- **API アドレス**：空欄にすると Google の公式アドレスを使います。カスタムアドレス（プロキシなど）には、host、project、region、publisher、model、action の順に `%s` プレースホルダーを 6 個含める必要があります。例：`https://proxy.example.com/%s/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s`。旧形式（publisher なしの 5 個）のアドレスは Gemini モデルに限り引き続き使えます。それ以外のアドレスは無視されずエラーになるため、トラフィックがプロキシを迂回することはありません。
