import { defineConfig } from 'vitepress'
import llmstxt from 'vitepress-plugin-llms'
import { localesConfig } from './locales.mts'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Modeltaps Docs",
  description:
    "Modeltaps is a unified multi-protocol AI gateway compatible with the OpenAI, Claude and Gemini APIs, with organizations, budgets and usage analytics.",
  head: [['link', { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }]],
  lastUpdated: true,
  // Operations docs are not published with the docs site yet
  srcExclude: ['ops/**'],
  vite: {
    plugins: [
      llmstxt({
        // The other languages translate the same pages; llms.txt only takes the English copy
        ignoreFiles: [
          'ops/**',
          'zh/**',
          'ja/**',
          'hk/**'
        ]
      })
    ]
  },
  markdown: {
    math: true,
    image: {
      lazyLoading: true
    }
  },
  // Four languages: English at the root, the others mirror the same tree under /zh/, /ja/, /hk/
  locales: localesConfig(),
  themeConfig: {
    // Search is configured site-wide; options.locales overrides its UI text per language (the
    // root uses the top-level translations)
    search: {
      provider: 'local',
      options: {
        translations: {
          button: { buttonText: 'Search', buttonAriaLabel: 'Search' },
          modal: {
            displayDetails: 'Display detailed list',
            resetButtonTitle: 'Reset search',
            backButtonTitle: 'Close search',
            noResultsText: 'No results for',
            footer: {
              selectText: 'to select',
              selectKeyAriaLabel: 'enter',
              navigateText: 'to navigate',
              navigateUpKeyAriaLabel: 'up arrow',
              navigateDownKeyAriaLabel: 'down arrow',
              closeText: 'to close',
              closeKeyAriaLabel: 'escape'
            }
          }
        },
        locales: {
          zh: {
            translations: {
              button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
              modal: {
                displayDetails: '显示详细列表',
                resetButtonTitle: '清除查询条件',
                backButtonTitle: '返回',
                noResultsText: '无相关结果',
                footer: {
                  selectText: '选择',
                  selectKeyAriaLabel: '回车',
                  navigateText: '切换',
                  navigateUpKeyAriaLabel: '上箭头',
                  navigateDownKeyAriaLabel: '下箭头',
                  closeText: '关闭',
                  closeKeyAriaLabel: 'esc'
                }
              }
            }
          },
          ja: {
            translations: {
              button: { buttonText: 'ドキュメントを検索', buttonAriaLabel: 'ドキュメントを検索' },
              modal: {
                displayDetails: '詳細を表示',
                resetButtonTitle: '検索条件をクリア',
                backButtonTitle: '戻る',
                noResultsText: '結果が見つかりません',
                footer: {
                  selectText: '選択',
                  selectKeyAriaLabel: 'Enter',
                  navigateText: '移動',
                  navigateUpKeyAriaLabel: '上矢印',
                  navigateDownKeyAriaLabel: '下矢印',
                  closeText: '閉じる',
                  closeKeyAriaLabel: 'esc'
                }
              }
            }
          },
          hk: {
            translations: {
              button: { buttonText: '搜尋文件', buttonAriaLabel: '搜尋文件' },
              modal: {
                displayDetails: '顯示詳細清單',
                resetButtonTitle: '清除查詢條件',
                backButtonTitle: '返回',
                noResultsText: '沒有相關結果',
                footer: {
                  selectText: '選擇',
                  selectKeyAriaLabel: 'Enter',
                  navigateText: '切換',
                  navigateUpKeyAriaLabel: '上箭頭',
                  navigateDownKeyAriaLabel: '下箭頭',
                  closeText: '關閉',
                  closeKeyAriaLabel: 'esc'
                }
              }
            }
          }
        }
      }
    },
    logo: { light: '/brand-mark.svg', dark: '/brand-mark-white.svg', alt: 'Modeltaps Logo' }
    // https://vitepress.dev/reference/default-theme-config
    // nav, sidebar, UI strings and footer come from each locale's themeConfig in locales.mts
  }
})
