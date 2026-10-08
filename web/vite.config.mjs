// https://github.com/vitejs/vite/discussions/3448
import path from 'path';
import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import jsconfigPaths from 'vite-jsconfig-paths';
import tailwindcss from '@tailwindcss/vite';

// ----------------------------------------------------------------------

// Release builds inject the git tag via `VITE_APP_VERSION=$(git describe --tags)`
// (see Makefile + .github workflows), matching the backend's config.Version.
// Pin it to an always-defined string so the version-mismatch notice never trips
// on `undefined` in dev (where it stays empty and the check is skipped).
const appVersion = process.env.VITE_APP_VERSION || '';

export default defineConfig(({ mode }) => {
  // Production images build without web/.env (gitignored, not committed), so the
  // index.html %VITE_APP_NAME% placeholder would otherwise leak as a literal.
  // Fall back to 'Modeltaps' only when neither .env nor the environment provides a
  // value — this keeps a local .env override intact.
  const env = loadEnv(mode, process.cwd(), 'VITE_');
  // dev/preview hosts: Cloudflare quick tunnels, mDNS / LAN names, Tailscale MagicDNS, plus any
  // bare machine names listed comma-separated in VITE_ALLOWED_HOSTS
  const devAllowedHosts = [
    '.trycloudflare.com',
    '.local',
    '.lan',
    '.ts.net',
    ...(env.VITE_ALLOWED_HOSTS || '')
      .split(',')
      .map((h) => h.trim())
      .filter(Boolean)
  ];
  if (!env.VITE_APP_NAME) {
    process.env.VITE_APP_NAME = 'Modeltaps';
  }

  // Dev/preview API proxy target. Override via VITE_PROXY_TARGET (e.g. in
  // web/.env.local) when the backend runs on a non-default port. Production
  // serves same-origin, so this is unused there.
  const proxyTarget = env.VITE_PROXY_TARGET || 'http://127.0.0.1:3000';

  // Dev/preview listening port. Override via VITE_DEV_PORT (e.g. in
  // web/.env.local). strictPort below makes Vite exit when it is taken instead
  // of silently hopping to another port.
  const devPort = Number(env.VITE_DEV_PORT) || 3010;

  return {
    plugins: [react(), jsconfigPaths(), tailwindcss()],
    // vitest picks this up via the shared vite config (run: pnpm test)
    test: {
      environment: 'node',
      setupFiles: ['src/test/setup-i18n.js']
    },
    define: {
      'import.meta.env.VITE_APP_VERSION': JSON.stringify(appVersion)
    },
    css: {
      preprocessorOptions: {
        scss: {
          // 使用现代 Sass API 解决 Legacy JS API 警告
          api: 'modern-compiler',
          // 静默弃用警告
          silenceDeprecations: ['legacy-js-api', 'import']
        }
      }
    },
    resolve: {
      alias: [
        {
          find: /^~(.+)/,
          replacement: path.join(process.cwd(), 'node_modules/$1')
        },
        {
          find: /^src(.+)/,
          replacement: path.join(process.cwd(), 'src/$1')
        },
        {
          find: /^@\/(.+)/,
          replacement: path.join(process.cwd(), 'src/$1')
        }
      ]
    },
    server: {
      // this ensures that the browser opens upon server start
      open: true,
      host: true,
      // fixed port (override with VITE_DEV_PORT); fail fast when it is in use
      port: devPort,
      strictPort: true,
      allowedHosts: devAllowedHosts,
      proxy: {
        '/api': {
          target: proxyTarget, // 设置代理的目标服务器
          changeOrigin: true
        },
        // 令牌测试弹窗按协议探测可用模型,需直连各协议中继端点
        '/gemini': { target: proxyTarget, changeOrigin: true },
        '/claude': { target: proxyTarget, changeOrigin: true },
        // 面板内置控制台直连 /v1/* 中继端点,与上面两条同理
        '/v1': { target: proxyTarget, changeOrigin: true }
      }
    },
    preview: {
      // this ensures that the browser opens upon preview start
      open: true,
      // fixed port (override with VITE_DEV_PORT); fail fast when it is in use
      port: devPort,
      strictPort: true,
      allowedHosts: devAllowedHosts,
      proxy: {
        '/api': {
          target: proxyTarget,
          changeOrigin: true
        },
        // 令牌测试弹窗按协议探测可用模型,需直连各协议中继端点
        '/gemini': { target: proxyTarget, changeOrigin: true },
        '/claude': { target: proxyTarget, changeOrigin: true },
        // 面板内置控制台直连 /v1/* 中继端点,与上面两条同理
        '/v1': { target: proxyTarget, changeOrigin: true }
      }
    },
    build: {
      // 生产构建不产出 sourcemap,避免线上暴露源码
      sourcemap: false,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (!id.includes('node_modules')) return;
            if (/node_modules\/(recharts|d3-[^/]+|victory-vendor|internmap)\//.test(id)) return 'vendor-charts';
            if (id.includes('node_modules/@tanstack/')) return 'vendor-table';
            if (id.includes('node_modules/@radix-ui/')) return 'vendor-ui';
            if (id.includes('node_modules/@iconify/') || id.includes('node_modules/lucide-react/')) return 'vendor-icons';
            if (/node_modules\/(marked|highlight\.js)\//.test(id)) return 'vendor-markdown';
          }
        }
      }
    }
  };
});
