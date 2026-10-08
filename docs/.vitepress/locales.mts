import type { DefaultTheme } from 'vitepress'

/**
 * UI strings and section titles for each language.
 * Page paths match one to one across the four languages, so the nav/sidebar structure is shared
 * and only the text and the path prefix change.
 */
export interface LocaleText {
  label: string
  lang: string
  title: string
  description: string
  nav: { home: string; start: string; guide: string; faq: string }
  groups: {
    start: string
    api: string
  }
  pages: {
    quickstart: string
    api: string
    reasoning: string
    special: string
    tokens: string
    organization: string
    billing: string
    usage: string
    faq: string
  }
  ui: {
    outline: string
    prev: string
    next: string
    sidebarMenu: string
    returnToTop: string
    darkModeSwitch: string
    lightModeSwitchTitle: string
    darkModeSwitchTitle: string
    langMenu: string
    lastUpdated: string
    footerMessage: string
    notFoundTitle: string
    notFoundQuote: string
    notFoundLink: string
    notFoundLinkLabel: string
  }
}

export const localeTexts: Record<'root' | 'zh' | 'ja' | 'hk', LocaleText> = {
  root: {
    label: 'English',
    lang: 'en-US',
    title: 'Modeltaps Docs',
    description:
      'Modeltaps is a unified multi-protocol AI gateway compatible with the OpenAI, Claude and Gemini APIs, with organization multi-tenancy, budget control and usage analytics.',
    nav: { home: 'Home', start: 'Getting Started', guide: 'Guide', faq: 'FAQ' },
    groups: {
      start: 'Getting Started',
      api: 'API Access'
    },
    pages: {
      quickstart: 'Getting Started',
      api: 'Compatible APIs',
      reasoning: 'Reasoning',
      special: 'Advanced Usage',
      tokens: 'API Keys and Quota',
      organization: 'Organizations',
      billing: 'Billing and Pricing',
      usage: 'Usage and Logs',
      faq: 'FAQ'
    },
    ui: {
      outline: 'On this page',
      prev: 'Previous',
      next: 'Next',
      sidebarMenu: 'Menu',
      returnToTop: 'Return to top',
      darkModeSwitch: 'Appearance',
      lightModeSwitchTitle: 'Switch to light theme',
      darkModeSwitchTitle: 'Switch to dark theme',
      langMenu: 'Change language',
      lastUpdated: 'Last updated',
      footerMessage: 'Modeltaps Documentation',
      notFoundTitle: 'PAGE NOT FOUND',
      notFoundQuote: 'Check the link, or start over from the home page.',
      notFoundLink: 'Take me home',
      notFoundLinkLabel: 'go to home'
    }
  },
  zh: {
    label: '简体中文',
    lang: 'zh-Hans',
    title: 'Modeltaps 文档',
    description:
      'Modeltaps 是一款统一的多协议 AI 网关，兼容 OpenAI、Claude、Gemini 接口，提供组织多租户、预算管控与用量分析能力。',
    nav: { home: '首页', start: '快速开始', guide: '使用指南', faq: '常见问题' },
    groups: {
      start: '开始使用',
      api: 'API 接入'
    },
    pages: {
      quickstart: '快速开始',
      api: '兼容接口',
      reasoning: '推理设置',
      special: '特殊用法',
      tokens: 'API Key 与额度',
      organization: '组织',
      billing: '计费与价格',
      usage: '用量与日志',
      faq: '常见问题'
    },
    ui: {
      outline: '本页目录',
      prev: '上一页',
      next: '下一页',
      sidebarMenu: '菜单',
      returnToTop: '返回顶部',
      darkModeSwitch: '外观',
      lightModeSwitchTitle: '切换到浅色模式',
      darkModeSwitchTitle: '切换到深色模式',
      langMenu: '切换语言',
      lastUpdated: '最后更新于',
      footerMessage: 'Modeltaps 产品文档',
      notFoundTitle: '页面不存在',
      notFoundQuote: '请检查链接是否正确，或从首页重新开始浏览。',
      notFoundLink: '返回首页',
      notFoundLinkLabel: '返回首页'
    }
  },
  ja: {
    label: '日本語',
    lang: 'ja-JP',
    title: 'Modeltaps ドキュメント',
    description:
      'Modeltaps は OpenAI・Claude・Gemini の API に対応した統合型 AI ゲートウェイで、組織のマルチテナント、予算管理、利用状況分析を提供します。',
    nav: { home: 'ホーム', start: 'クイックスタート', guide: '利用ガイド', faq: 'よくある質問' },
    groups: {
      start: 'はじめに',
      api: 'API 接続'
    },
    pages: {
      quickstart: 'クイックスタート',
      api: '互換 API',
      reasoning: '推論設定',
      special: '特殊な呼び出し',
      tokens: 'API キーとクォータ',
      organization: '組織',
      billing: '料金と価格',
      usage: '利用状況とログ',
      faq: 'よくある質問'
    },
    ui: {
      outline: 'このページの内容',
      prev: '前のページ',
      next: '次のページ',
      sidebarMenu: 'メニュー',
      returnToTop: 'トップに戻る',
      darkModeSwitch: '外観',
      lightModeSwitchTitle: 'ライトテーマに切り替える',
      darkModeSwitchTitle: 'ダークテーマに切り替える',
      langMenu: '言語を変更',
      lastUpdated: '最終更新',
      footerMessage: 'Modeltaps 製品ドキュメント',
      notFoundTitle: 'ページが見つかりません',
      notFoundQuote: 'リンクを確認するか、ホームからやり直してください。',
      notFoundLink: 'ホームに戻る',
      notFoundLinkLabel: 'ホームに戻る'
    }
  },
  hk: {
    label: '繁體中文',
    lang: 'zh-Hant',
    title: 'Modeltaps 文件',
    description:
      'Modeltaps 是一款統一的多協議 AI 網關，相容 OpenAI、Claude、Gemini 介面，提供組織多租戶、預算管控與用量分析能力。',
    nav: { home: '首頁', start: '快速開始', guide: '使用指南', faq: '常見問題' },
    groups: {
      start: '開始使用',
      api: 'API 接入'
    },
    pages: {
      quickstart: '快速開始',
      api: '相容介面',
      reasoning: '推理設定',
      special: '特殊用法',
      tokens: 'API Key 與額度',
      organization: '組織',
      billing: '計費與價格',
      usage: '用量與日誌',
      faq: '常見問題'
    },
    ui: {
      outline: '本頁目錄',
      prev: '上一頁',
      next: '下一頁',
      sidebarMenu: '菜單',
      returnToTop: '回到頂部',
      darkModeSwitch: '外觀',
      lightModeSwitchTitle: '切換至淺色模式',
      darkModeSwitchTitle: '切換至深色模式',
      langMenu: '切換語言',
      lastUpdated: '最後更新於',
      footerMessage: 'Modeltaps 產品文件',
      notFoundTitle: '頁面不存在',
      notFoundQuote: '請檢查鏈接是否正確，或從首頁重新開始瀏覽。',
      notFoundLink: '返回首頁',
      notFoundLinkLabel: '返回首頁'
    }
  }
}

/** Path prefix of a language; English is the root and has none. */
export type LocaleKey = keyof typeof localeTexts

function prefixOf(key: LocaleKey): string {
  return key === 'root' ? '' : `/${key}`
}

export function navOf(key: LocaleKey): DefaultTheme.NavItem[] {
  const t = localeTexts[key]
  const p = prefixOf(key)
  return [
    { text: t.nav.home, link: `${p}/` },
    { text: t.nav.start, link: `${p}/guide/quickstart` },
    { text: t.nav.guide, link: `${p}/guide/api` },
    { text: t.nav.faq, link: `${p}/guide/faq` }
  ]
}

export function sidebarOf(key: LocaleKey): DefaultTheme.SidebarItem[] {
  const { groups, pages } = localeTexts[key]
  const p = prefixOf(key)
  return [
    {
      text: groups.start,
      items: [{ text: pages.quickstart, link: `${p}/guide/quickstart` }]
    },
    {
      text: groups.api,
      items: [
        { text: pages.api, link: `${p}/guide/api` },
        { text: pages.reasoning, link: `${p}/guide/reasoning` },
        { text: pages.special, link: `${p}/guide/special` }
      ]
    },
    { text: pages.tokens, link: `${p}/guide/tokens` },
    { text: pages.organization, link: `${p}/guide/organization` },
    { text: pages.billing, link: `${p}/guide/billing` },
    { text: pages.usage, link: `${p}/guide/usage` },
    { text: pages.faq, link: `${p}/guide/faq` }
  ]
}

/** Per-language themeConfig: nav, sidebar and UI strings. */
export function themeConfigOf(key: LocaleKey): DefaultTheme.Config {
  const { ui } = localeTexts[key]
  return {
    nav: navOf(key),
    sidebar: sidebarOf(key),
    outline: { label: ui.outline },
    docFooter: { prev: ui.prev, next: ui.next },
    sidebarMenuLabel: ui.sidebarMenu,
    returnToTopLabel: ui.returnToTop,
    darkModeSwitchLabel: ui.darkModeSwitch,
    lightModeSwitchTitle: ui.lightModeSwitchTitle,
    darkModeSwitchTitle: ui.darkModeSwitchTitle,
    langMenuLabel: ui.langMenu,
    lastUpdated: { text: ui.lastUpdated },
    notFound: {
      title: ui.notFoundTitle,
      quote: ui.notFoundQuote,
      linkText: ui.notFoundLink,
      linkLabel: ui.notFoundLinkLabel
    },
    footer: { message: ui.footerMessage, copyright: '© 2026 Modeltaps' }
  }
}

/** Locale definitions: English at the root, the other languages under /zh/, /ja/ and /hk/. */
export function localesConfig() {
  const keys: LocaleKey[] = ['root', 'zh', 'ja', 'hk']
  return Object.fromEntries(
    keys.map((key) => {
      const t = localeTexts[key]
      return [
        key,
        {
          label: t.label,
          lang: t.lang,
          title: t.title,
          description: t.description,
          themeConfig: themeConfigOf(key)
        }
      ]
    })
  )
}
