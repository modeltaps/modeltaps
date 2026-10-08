---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Modeltaps"
  text: "一个网关，接入所有模型"
  tagline: OpenAI、Anthropic、Gemini 三种协议走同一个地址，工具只改 base URL 就能用任意模型。自己部署，key 和日志都在自己手里。
  image:
    light: /hero-gateway-light.svg
    dark: /hero-gateway-dark.svg
    alt: 网关示意图：左侧多位成员与应用各持一把钥匙，接入中间虚线标出的组织边界；边界内是 Modeltaps 网关与预算量表，右侧分出云端供应商与组织自带 key 两条通路。
  actions:
    - theme: brand
      text: 快速开始
      link: /zh/guide/quickstart
    - theme: alt
      text: 使用指南
      link: /zh/guide/api

features:
  - title: 多协议兼容
    icon:
      light: /icons/protocols-light.svg
      dark: /icons/protocols-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 同时提供 OpenAI、Claude、Gemini 三套兼容接口，既有应用无需改造代码，仅替换接入地址与密钥即可切换至 Modeltaps。
    link: /zh/guide/api
  - title: 组织与多租户
    icon:
      light: /icons/organizations-light.svg
      dark: /icons/organizations-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 支持组织、成员与角色分层管理，成员在组织额度下独立记账，适用于团队、部门与多客户共用同一部署的场景。
  - title: 预算与用量管控
    icon:
      light: /icons/budget-light.svg
      dark: /icons/budget-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 提供组织与成员两级预算上限、API Key 额度与有效期控制，超限即时拦截，避免非预期支出。
  - title: 渠道与供应商管理
    icon:
      light: /icons/routing-light.svg
      dark: /icons/routing-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 统一编排多家供应商渠道，支持分组路由、模型映射与通配符、失败重试与自动禁用，并可为渠道单独配置代理。
  - title: 灵活计费
    icon:
      light: /icons/billing-light.svg
      dark: /icons/billing-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 支持按量倍率与按次计费、扩展价格系数与价格批量维护，计费口径清晰，可按模型与场景精细定价。
    link: /zh/guide/billing
  - title: 可观测性
    icon:
      light: /icons/observability-light.svg
      dark: /icons/observability-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 提供请求日志、调用模态、耗时与消费明细，配合仪表盘与用量分析，便于成本归因与容量规划。
    link: /zh/guide/usage
---
