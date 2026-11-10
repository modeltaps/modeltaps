import { useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowRight, Check, Copy } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { LogoMark } from '@/components/chrome/Logo';
import { cn } from '@/lib/utils';
import { brandName } from 'utils/brand';

// ==============================|| PUBLIC HOME — HERO ||============================== //
// Headline and two actions on the left; on the right the one thing the page promises: a terminal
// that points a real coding tool at this very server. The base URLs are built from the address the
// page is served from, so the snippet can be copied and run as is. The brand drop stands in for the
// shell prompt (the page's one signature detail) and carries the single accent colour.

const TOOLS = [
  {
    id: 'claude',
    label: 'Claude Code',
    protocol: 'Anthropic',
    env: (origin) => [
      ['ANTHROPIC_BASE_URL', `${origin}/claude`],
      ['ANTHROPIC_AUTH_TOKEN', 'sk-YOUR_TOKEN']
    ],
    run: 'claude'
  },
  {
    id: 'codex',
    label: 'Codex CLI',
    protocol: 'OpenAI',
    env: (origin) => [
      ['OPENAI_BASE_URL', `${origin}/v1`],
      ['OPENAI_API_KEY', 'sk-YOUR_TOKEN']
    ],
    run: 'codex'
  },
  {
    id: 'gemini',
    label: 'Gemini CLI',
    protocol: 'Gemini',
    env: (origin) => [
      ['GOOGLE_GEMINI_BASE_URL', `${origin}/gemini`],
      ['GEMINI_API_KEY', 'sk-YOUR_TOKEN']
    ],
    run: 'gemini'
  }
];

// Static mock of the gateway's request log: three tools, three protocols, one exit. The row of the
// selected tool is lit. Literal technical strings and names, deliberately not translated.
const LOG = [
  { tool: 'claude', time: '14:02:11', path: '/claude/v1/messages', model: 'claude-sonnet-5', tokens: '3,412', member: 'alice' },
  { tool: 'codex', time: '14:02:09', path: '/v1/responses', model: 'gpt-5.5', tokens: '1,086', member: 'bob' },
  { tool: 'gemini', time: '14:02:04', path: '/gemini/v1beta', model: 'gemini-3.5-flash', tokens: '742', member: 'chen' }
];

const serverOrigin = () => (typeof window !== 'undefined' && window.location?.origin) || 'https://your-modeltaps';

function TerminalCard() {
  const { t } = useTranslation();
  const [active, setActive] = useState(0);
  const [copied, setCopied] = useState(false);
  const tabRefs = useRef([]);
  const tool = TOOLS[active];
  const env = tool.env(serverOrigin());
  const script = [...env.map(([k, v]) => `export ${k}=${v}`), tool.run].join('\n');

  const select = (index) => {
    const next = (index + TOOLS.length) % TOOLS.length;
    setActive(next);
    setCopied(false);
    tabRefs.current[next]?.focus();
  };

  const onKeyDown = (event) => {
    const keys = { ArrowRight: active + 1, ArrowLeft: active - 1, Home: 0, End: TOOLS.length - 1 };
    if (!(event.key in keys)) return;
    event.preventDefault();
    select(keys[event.key]);
  };

  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(script);
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-[0_24px_48px_-24px_rgba(0,0,0,0.25)]">
      <div className="flex items-center justify-between gap-2 border-b border-border px-2">
        <div role="tablist" aria-label={t('home.hero.toolsAria')} className="flex min-w-0 overflow-x-auto">
          {TOOLS.map((item, index) => (
            <button
              key={item.id}
              ref={(node) => {
                tabRefs.current[index] = node;
              }}
              type="button"
              role="tab"
              id={`hero-tool-${item.id}`}
              aria-selected={index === active}
              aria-controls="hero-tool-panel"
              tabIndex={index === active ? 0 : -1}
              onClick={() => select(index)}
              onKeyDown={onKeyDown}
              className={cn(
                'relative h-11 shrink-0 whitespace-nowrap px-3 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring motion-reduce:transition-none',
                index === active ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {item.label}
              {index === active && <span aria-hidden="true" className="absolute inset-x-3 bottom-0 h-0.5 rounded-full bg-foreground" />}
            </button>
          ))}
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="h-8 shrink-0 gap-1.5 px-2.5 text-muted-foreground"
          onClick={onCopy}
          aria-label={copied ? t('home.hero.copied') : t('common.copy')}
        >
          {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
          <span className="hidden sm:inline">{copied ? t('home.hero.copied') : t('common.copy')}</span>
        </Button>
      </div>

      <div
        role="tabpanel"
        id="hero-tool-panel"
        aria-labelledby={`hero-tool-${tool.id}`}
        tabIndex={0}
        className="overflow-x-auto px-5 py-6 font-mono text-[13px] leading-7 focus-visible:outline-none"
      >
        <pre className="m-0">
          <code>
            {env.map(([key, value]) => (
              <span key={key} className="block whitespace-pre">
                <span className="text-muted-foreground">export </span>
                <span className="text-foreground">{key}</span>
                <span className="text-muted-foreground">=</span>
                <span className="text-foreground">{value}</span>
              </span>
            ))}
            <span className="mt-3 flex items-center gap-2.5">
              <LogoMark size={12} className="text-info" />
              <span className="text-foreground">{tool.run}</span>
              <span aria-hidden="true" className="h-4 w-2 bg-foreground/70" />
            </span>
          </code>
        </pre>
      </div>

      <div className="border-t border-border bg-muted/40 px-5 py-4">
        <div className="flex items-baseline justify-between gap-3 text-[12px] text-muted-foreground">
          <span>{t('home.hero.logTitle')}</span>
          <span>{t('home.hero.protocolNote', { protocol: tool.protocol })}</span>
        </div>
        <ul className="mt-3 grid gap-1 font-mono text-[12px] tabular-nums">
          {LOG.map((row) => (
            <li
              key={row.tool}
              className={cn(
                'grid grid-cols-[auto_1fr_auto] items-center gap-3 rounded-md px-2 py-1.5 transition-colors sm:grid-cols-[auto_4.5rem_1fr_auto_3rem] motion-reduce:transition-none',
                row.tool === tool.id ? 'bg-card text-foreground' : 'text-muted-foreground'
              )}
            >
              <span
                aria-hidden="true"
                className={cn('size-1.5 rounded-full', row.tool === tool.id ? 'bg-info' : 'bg-muted-foreground/40')}
              />
              <span className="hidden sm:block">{row.time}</span>
              <span className="truncate">
                {row.path} <span className="text-muted-foreground">· {row.model}</span>
              </span>
              <span className="text-right">{row.tokens}</span>
              <span className="hidden text-right sm:block">{row.member}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

export default function Hero() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const siteInfo = useSelector((state) => state.siteInfo);
  const account = useSelector((state) => state.account);

  // Primary action targets the API-key page. Signed-in users go straight there; visitors detour
  // through login and come back via ?redirect=.
  const handleGetKey = () => {
    navigate(account?.user ? '/panel/token' : `/login?redirect=${encodeURIComponent('/panel/token')}`);
  };

  return (
    <section className="border-b border-border">
      <div className="mx-auto grid min-h-[min(calc(100svh-3.5rem),760px)] max-w-6xl grid-cols-1 items-center gap-12 px-4 py-16 sm:px-6 lg:grid-cols-[1fr_1.2fr] lg:gap-16 lg:py-24">
        <div>
          <h1 className="max-w-[14ch] text-balance break-keep text-[2.5rem] font-semibold leading-[1.1] tracking-[-0.02em] sm:text-6xl">
            {t('home.hero.tagline')}
          </h1>
          <p className="mt-6 max-w-[34rem] text-pretty text-base leading-relaxed text-muted-foreground sm:text-lg">
            {t('home.hero.subline')}
          </p>

          <div className="mt-10 flex flex-col gap-3 sm:flex-row">
            <Button size="lg" className="h-12 px-6" onClick={handleGetKey}>
              {t('home.hero.ctaPrimary')}
              <ArrowRight />
            </Button>
            <Button asChild size="lg" variant="ghost" className="h-12 px-5 text-muted-foreground hover:text-foreground">
              <Link to="/price">{t('home.hero.ctaSecondary')}</Link>
            </Button>
          </div>

          <p className="sr-only">{brandName(siteInfo.system_name)}</p>
        </div>

        <TerminalCard />
      </div>
    </section>
  );
}
