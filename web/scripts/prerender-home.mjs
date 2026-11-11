// Prerenders the static home page served at `/` (see src/static-home and router/home.go).
//
// Runs after the client build: builds src/static-home/entry-server.jsx for Node with the same vite
// config, renders one page per language and writes build/home/<lang>.html. Each page is the built
// index.html (icons, stylesheet, social tags) with the app bundle taken out and the rendered markup
// put in, so the page shows its content without running the app. Values only the server knows
// (site name, origin, docs link) stay as tokens for the Go side to fill in.

import { mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { build } from 'vite';

const webDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const buildDir = path.join(webDir, 'build');
const ssrDir = path.join(webDir, 'node_modules', '.static-home');

const escapeHtml = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const htmlLang = (lng) => lng.replace('_', '-');

// Applies the stored or system theme before first paint (same localStorage key as hooks/useTheme).
const THEME_INIT = `<script>(function(){try{var s=localStorage.getItem('theme');var d=s==='dark'||(s!=='light'&&matchMedia('(prefers-color-scheme: dark)').matches);var r=document.documentElement;r.classList.toggle('dark',d);r.classList.toggle('light',!d)}catch(e){}})()</script>`;

// The page's own behaviour: theme toggle, closing the language menu on an outside click, and
// keeping the app's language preference in step with the page. The drip interaction is inlined
// separately from views/public/home/dripInteraction.js.
const PAGE_SCRIPT = `<script>(function(){
var root=document.documentElement,key='appLanguage';
document.querySelectorAll('[data-theme-toggle]').forEach(function(b){b.addEventListener('click',function(){var d=!root.classList.contains('dark');root.classList.toggle('dark',d);root.classList.toggle('light',!d);try{localStorage.setItem('theme',d?'dark':'light')}catch(e){}})});
document.addEventListener('click',function(e){document.querySelectorAll('details[open]').forEach(function(m){if(!m.contains(e.target))m.removeAttribute('open')})});
try{var q=new URLSearchParams(location.search).get('lang'),cur=root.getAttribute('data-lang');
if(q===cur){localStorage.setItem(key,cur)}
else if(!q&&document.cookie.indexOf(key+'=')<0){var saved=localStorage.getItem(key);if(saved&&saved!==cur){location.replace('/?lang='+encodeURIComponent(saved))}}}catch(e){}
})()</script>`;

function setMeta(html, attr, name, content) {
  const re = new RegExp(`(<meta ${attr}="${name}" content=")[^"]*(")`);
  if (!re.test(html)) throw new Error(`index.html has no <meta ${attr}="${name}">`);
  return html.replace(re, `$1${escapeHtml(content)}$2`);
}

async function main() {
  await build({
    root: webDir,
    logLevel: 'warn',
    build: {
      ssr: 'src/static-home/entry-server.jsx',
      outDir: ssrDir,
      emptyOutDir: true,
      copyPublicDir: false,
      rollupOptions: { output: { manualChunks: undefined, entryFileNames: 'entry-server.mjs' } }
    }
  });
  const { renderHome, LANGUAGES, STATIC_TOKENS, DRIP_SCRIPT } = await import(pathToFileURL(path.join(ssrDir, 'entry-server.mjs')).href);

  const template = await readFile(path.join(buildDir, 'index.html'), 'utf8');
  if (!template.includes('<div id="root"')) throw new Error('build/index.html has no #root element');

  const origin = STATIC_TOKENS.origin;
  const alternates = [
    ...LANGUAGES.map((l) => `<link rel="alternate" hreflang="${htmlLang(l)}" href="${origin}/?lang=${l}" />`),
    `<link rel="alternate" hreflang="x-default" href="${origin}/" />`
  ].join('\n    ');

  await mkdir(path.join(buildDir, 'home'), { recursive: true });
  for (const lng of LANGUAGES) {
    const { html: body, tagline, catchphrase } = renderHome(lng);
    const title = `${STATIC_TOKENS.brand} — ${tagline}`;
    const sep = /[.!?。！？]$/.test(tagline) ? ' ' : /^(zh|ja)/.test(lng) ? '。' : '. ';
    const description = `${tagline}${sep}${catchphrase}`;

    let page = template
      .replace(/<html lang="[^"]*"/, `<html lang="${htmlLang(lng)}" data-lang="${lng}"`)
      .replace(/<title>[^<]*<\/title>/, `<title>${escapeHtml(title)}</title>`)
      // no app bundle: drop the entry module, its preloads and the no-JS notice
      .replace(/\s*<script type="module"[^>]*><\/script>/g, '')
      .replace(/\s*<link rel="modulepreload"[^>]*>/g, '')
      .replace(/\s*<noscript>[\s\S]*?<\/noscript>/, '');
    page = setMeta(page, 'name', 'description', description);
    page = setMeta(page, 'property', 'og:title', title);
    page = setMeta(page, 'property', 'og:description', description);
    page = setMeta(page, 'name', 'twitter:title', title);
    page = setMeta(page, 'name', 'twitter:description', description);
    page = page.replace(
      '</head>',
      `  <link rel="canonical" href="${origin}/?lang=${lng}" />\n    ${alternates}\n    ${THEME_INIT}\n  </head>`
    );
    // a replacer function, so `$` sequences in the inlined sources are not read as patterns
    page = page.replace(
      /<div id="root"([^>]*)><\/div>/,
      (_, attrs) => `<div id="root"${attrs}>${body}</div>\n    ${PAGE_SCRIPT}\n    <script>${DRIP_SCRIPT}</script>`
    );
    if (/<\/script/i.test(DRIP_SCRIPT)) throw new Error('drip script must not contain a closing script tag');
    if (page.includes('type="module"')) throw new Error('static home still references the app bundle');

    await writeFile(path.join(buildDir, 'home', `${lng}.html`), page);
  }
  await rm(ssrDir, { recursive: true, force: true });
  console.log(`static home: wrote ${LANGUAGES.length} pages to build/home/`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
