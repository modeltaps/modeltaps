---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Modeltaps"
  text: "一個網關，接通所有模型"
  tagline: OpenAI、Anthropic、Gemini 三種協議共用同一個地址，工具只需改 base URL 便可使用任何模型。自行部署，key 與日誌都保留在自己手上。
  image:
    light: /hero-gateway-light.svg
    dark: /hero-gateway-dark.svg
    alt: 網關示意圖：左側多位成員與應用各持一把鑰匙，接入中間虛線標出的組織邊界；邊界內是 Modeltaps 網關與預算量表，右側分出雲端供應商與組織自帶 key 兩條通路。
  actions:
    - theme: brand
      text: 快速開始
      link: /hk/guide/quickstart
    - theme: alt
      text: 使用指南
      link: /hk/guide/api

features:
  - title: 多協議相容
    icon:
      light: /icons/protocols-light.svg
      dark: /icons/protocols-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 同時提供 OpenAI、Claude、Gemini 三套相容介面，既有應用無需改造代碼，僅替換接入地址與密鑰即可切換至 Modeltaps。
    link: /hk/guide/api
  - title: 組織與多租戶
    icon:
      light: /icons/organizations-light.svg
      dark: /icons/organizations-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 支援組織、成員與角色分層管理，成員在組織額度下獨立記賬，適用於團隊、部門與多客戶共用同一部署的場景。
  - title: 預算與用量管控
    icon:
      light: /icons/budget-light.svg
      dark: /icons/budget-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 提供組織與成員兩級預算上限、API Key 額度與有效期控制，超限即時攔截，避免非預期支出。
  - title: 渠道與供應商管理
    icon:
      light: /icons/routing-light.svg
      dark: /icons/routing-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 統一編排多家供應商渠道，支援分組路由、模型映射與通配符、失敗重試與自動禁用，並可為渠道單獨配置代理。
  - title: 靈活計費
    icon:
      light: /icons/billing-light.svg
      dark: /icons/billing-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 支援按量倍率與按次計費、擴展價格系數與價格批量維護，計費口徑清晰，可按模型與場景精細定價。
    link: /hk/guide/billing
  - title: 可觀測性
    icon:
      light: /icons/observability-light.svg
      dark: /icons/observability-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 提供請求日誌、調用模態、耗時與消費明細，配合儀表板與用量分析，便於成本歸因與容量規劃。
    link: /hk/guide/usage
---
