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

## Channel settings

- **Other**: `Region|ProjectID`, or `Region1|Region2|Region3|ProjectID` to pick one of several regions at random for each request. `Region` can be `global`, the multi-regions `us` and `eu`, or a specific region such as `us-east5`.
- **Claude models**: Claude models are served by the Anthropic publisher of Vertex AI. Enable each model for the project in Vertex AI Model Garden first, and pick a region that serves it (see Google's documentation). Requests to the Claude-compatible endpoint (`/claude/v1/messages`) are forwarded as sent, so prompt caching fields, tools and the like reach Vertex unchanged; the OpenAI-compatible chat endpoint works as well. Of the `anthropic-beta` values a client sends, only those Vertex accepts are forwarded; to pin the header yourself, set `anthropic-beta` in the channel's model headers.
- **API address**: leave it empty to use Google's endpoint. A custom address (for example a proxy) must contain six `%s` placeholders, in this order: host, project, region, publisher, model and action, for example `https://proxy.example.com/%s/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s`. An address in the older five-placeholder form (no publisher) keeps working for Gemini models only. Any other address is rejected with an error instead of being ignored, so traffic never bypasses your proxy.
