---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Modeltaps"
  text: "Every model, every protocol. One gateway."
  tagline: OpenAI, Anthropic and Gemini APIs behind a single base URL. Point your tools at one address and use any model — self-hosted, with your own keys.
  image:
    light: /hero-gateway-light.svg
    dark: /hero-gateway-dark.svg
    alt: "Diagram of the gateway: members and apps on the left each hold a key and connect into the dashed organization boundary, which holds the Modeltaps gateway and a budget meter; two lanes on the right lead to cloud providers and organization-owned keys."
  actions:
    - theme: brand
      text: Get Started
      link: /guide/quickstart
    - theme: alt
      text: Guide
      link: /guide/api

features:
  - title: Multi-protocol compatibility
    icon:
      light: /icons/protocols-light.svg
      dark: /icons/protocols-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: Exposes OpenAI-, Claude- and Gemini-compatible APIs at the same time. Existing applications switch to Modeltaps by changing only the base URL and the API key — no code changes required.
    link: /guide/api
  - title: Organizations and multi-tenancy
    icon:
      light: /icons/organizations-light.svg
      dark: /icons/organizations-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: Hierarchical management of organizations, members and roles. Members are billed independently against the organization quota, which fits teams, departments and multiple customers sharing a single deployment.
  - title: Budget and usage control
    icon:
      light: /icons/budget-light.svg
      dark: /icons/budget-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: Budget caps at both the organization and member level, together with API key quota and expiry controls. Requests are blocked the moment a limit is exceeded, preventing unexpected spend.
  - title: Channel and provider management
    icon:
      light: /icons/routing-light.svg
      dark: /icons/routing-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: Orchestrate channels across providers with group-based routing, model mapping and wildcards, retry on failure and automatic disabling, plus a per-channel proxy.
  - title: Flexible billing
    icon:
      light: /icons/billing-light.svg
      dark: /icons/billing-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: Supports usage-based ratios and per-request pricing, extra ratios and bulk price maintenance. Billing rules are explicit and can be tuned per model and per scenario.
    link: /guide/billing
  - title: Observability
    icon:
      light: /icons/observability-light.svg
      dark: /icons/observability-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: Request logs, call modalities, latency and spend details, combined with dashboards and usage analytics, make cost attribution and capacity planning straightforward.
    link: /guide/usage
---
