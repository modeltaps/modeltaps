---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Modeltaps"
  text: "すべてのモデルを、ひとつのゲートウェイで。"
  tagline: OpenAI・Anthropic・Gemini の 3 つのプロトコルをひとつのアドレスで。ツールは base URL を変えるだけで、どのモデルも使えます。自分で運用し、キーとログは手元に。
  image:
    light: /hero-gateway-light.svg
    dark: /hero-gateway-dark.svg
    alt: ゲートウェイを示す図：左側のメンバーとアプリがそれぞれ鍵を持ち、破線で示された組織の境界に接続します。境界の中には Modeltaps ゲートウェイと予算メーターがあり、右側はクラウドプロバイダーと組織持ち込みの key という 2 つの経路に分かれます。
  actions:
    - theme: brand
      text: クイックスタート
      link: /ja/guide/quickstart
    - theme: alt
      text: 利用ガイド
      link: /ja/guide/api

features:
  - title: マルチプロトコル対応
    icon:
      light: /icons/protocols-light.svg
      dark: /icons/protocols-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: OpenAI・Claude・Gemini の 3 種類の互換 API を同時に提供します。既存のアプリケーションはコードを変更する必要はなく、接続先アドレスとキーを差し替えるだけで Modeltaps に切り替えられます。
    link: /ja/guide/api
  - title: 組織とマルチテナント
    icon:
      light: /icons/organizations-light.svg
      dark: /icons/organizations-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 組織・メンバー・ロールの階層的な管理に対応し、メンバーは組織のクォータの下で個別に記帳されます。チームや部門、複数の顧客が同一のデプロイを共用するシナリオに適しています。
  - title: 予算と利用量の管理
    icon:
      light: /icons/budget-light.svg
      dark: /icons/budget-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 組織とメンバーの 2 段階の予算上限、API キーのクォータと有効期限の制御を提供します。上限を超えると即座に遮断し、想定外の支出を防ぎます。
  - title: チャネルとプロバイダー管理
    icon:
      light: /icons/routing-light.svg
      dark: /icons/routing-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 複数プロバイダーのチャネルを一元的に編成し、グループ単位のルーティング、モデルマッピングとワイルドカード、失敗時のリトライと自動無効化に対応します。チャネルごとにプロキシを個別設定することもできます。
  - title: 柔軟な課金
    icon:
      light: /icons/billing-light.svg
      dark: /icons/billing-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: 従量倍率と回数単位の課金、拡張価格係数、価格の一括メンテナンスに対応します。課金基準が明確で、モデルやシナリオごとに細かく価格を設定できます。
    link: /ja/guide/billing
  - title: オブザーバビリティ
    icon:
      light: /icons/observability-light.svg
      dark: /icons/observability-dark.svg
      alt: ''
      width: 32
      height: 32
      wrap: true
    details: リクエストログ、呼び出しモダリティ、所要時間、消費明細を提供します。ダッシュボードと利用状況分析を組み合わせることで、コストの帰属分析やキャパシティプランニングが容易になります。
    link: /ja/guide/usage
---
