---
title: "Adding VertexAI"
layout: doc
outline: deep
lastUpdated: true
---

# Adding VertexAI

1. Create a service credential.
   [Open this page](https://console.cloud.google.com/iam-admin/serviceaccounts), select your project, then click "Create service account".
   <img width="1099" alt="Create service account" src="/images/vertexai/step1-create-service-account.png">

2. Fill in the service account details and choose a name.
   <img width="604" alt="Service account details" src="/images/vertexai/step2-account-details.png">

3. Select roles: type `Vertex AI User` and `Service Account Token Creator` into the filter and select both roles.

<img width="582" alt="Filter roles" src="/images/vertexai/step3-role-filter.png">
<img width="593" alt="Select roles" src="/images/vertexai/step3-role-select.png">

4. Click "Continue", then click "Done".

5. Open the service account you just created, go to the "Keys" tab, choose "Create new key", select "JSON" and click "Create".
   <img width="598" alt="Keys tab" src="/images/vertexai/step5-keys-tab.png">
   <img width="454" alt="Create new key" src="/images/vertexai/step5-create-key.png">

<img width="386" alt="Select JSON key type" src="/images/vertexai/step5-key-type-json.png">

6. Download the generated JSON file, open it locally and copy its entire content into the channel's `Key` field.

7. Finally, enable the `IAM Service Account Credentials API` on [this page](https://console.cloud.google.com/apis/library/iamcredentials.googleapis.com).
