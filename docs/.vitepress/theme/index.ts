import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import './custom.css'
import { setupLocaleRedirect } from './locale-redirect'

export default {
  extends: DefaultTheme,
  enhanceApp({ router, siteData }) {
    setupLocaleRedirect(router, siteData.value.base)
  }
} satisfies Theme
