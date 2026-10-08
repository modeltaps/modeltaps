import { watch } from 'vue'
import type { Router } from 'vitepress'

/**
 * Language routing on the first visit
 *
 * - Only applies to the site home page (/ or /index.html); deep links always open as requested
 * - Uses the language remembered in localStorage first, then matches the browser languages
 * - Every route change writes the current language back, so the language switcher's choice is
 *   what gets remembered
 * - Runs in the browser only; skipped during SSR
 */

const STORAGE_KEY = 'modeltaps-docs-locale'
const PREFIXES = ['zh', 'ja', 'hk'] as const

type LocaleKey = 'root' | (typeof PREFIXES)[number]

function readStored(): LocaleKey | null {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    if (value === 'root' || (PREFIXES as readonly string[]).includes(value ?? '')) {
      return value as LocaleKey
    }
  } catch {
    // localStorage is unavailable in private mode; treat it as nothing remembered
  }
  return null
}

function writeStored(key: LocaleKey): void {
  try {
    localStorage.setItem(STORAGE_KEY, key)
  } catch {
    // ignore write failures
  }
}

/** Matches the browser languages to a locale; anything else stays on English. */
function detectFromNavigator(): LocaleKey {
  const tags = navigator.languages?.length ? navigator.languages : [navigator.language]
  for (const raw of tags) {
    const tag = (raw || '').toLowerCase()
    if (tag.startsWith('zh')) {
      return /(^|-)(tw|hk|mo|hant)(-|$)/.test(tag) ? 'hk' : 'zh'
    }
    if (tag.startsWith('ja')) return 'ja'
    if (tag.startsWith('en')) return 'root'
  }
  return 'root'
}

/** Strips the base prefix to get the in-site path. */
function sitePath(path: string, base: string): string {
  const normalizedBase = base.endsWith('/') ? base.slice(0, -1) : base
  const stripped = normalizedBase && path.startsWith(normalizedBase)
    ? path.slice(normalizedBase.length)
    : path
  return stripped.startsWith('/') ? stripped : `/${stripped}`
}

function localeOf(path: string): LocaleKey {
  const segment = path.split('/')[1]
  return (PREFIXES as readonly string[]).includes(segment) ? (segment as LocaleKey) : 'root'
}

function isSiteHome(path: string): boolean {
  return path === '/' || path === '/index.html'
}

export function setupLocaleRedirect(router: Router, base = '/'): void {
  if (typeof window === 'undefined') return

  let redirecting = false
  if (isSiteHome(sitePath(router.route.path, base))) {
    const target = readStored() ?? detectFromNavigator()
    if (target !== 'root') {
      redirecting = true
      const prefix = base.endsWith('/') ? base : `${base}/`
      // A full page load rather than router.go: the redirect happens before mount, and switching
      // locale inside the SPA would not match the SSR DOM (the nav would stay in the old
      // language). replace also keeps the intermediate page out of history.
      location.replace(`${prefix}${target}/`)
    }
  }

  watch(
    () => router.route.path,
    (path) => {
      // Do not store the home page's root locale before the redirect, or it would overwrite
      // the detected language
      if (redirecting) {
        redirecting = false
        return
      }
      writeStored(localeOf(sitePath(path, base)))
    },
    { immediate: true }
  )
}
