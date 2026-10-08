import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ==============================|| PUBLIC HOME — NATIVE PROTOCOLS ||============================== //
// Three protocol-compatible endpoints behind one address: tabbed curl samples for the
// OpenAI / Anthropic / Gemini surfaces plus the tools that can point at them directly.
// Product tokens only (no brand colour); request samples and tool names are literal
// technical strings, so they stay untranslated. No animation beyond token transitions.

const PROTOCOLS = [
  {
    id: 'openai',
    label: 'OpenAI',
    endpoint: 'POST /v1/chat/completions',
    code: `curl https://<your-modeltaps>/v1/chat/completions \\
  -H "Authorization: Bearer sk-YOUR_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-5.5",
    "messages": [{ "role": "user", "content": "hi~" }]
  }'`
  },
  {
    id: 'anthropic',
    label: 'Anthropic',
    endpoint: 'POST /claude/v1/messages',
    code: `curl https://<your-modeltaps>/claude/v1/messages \\
  -H "x-api-key: sk-YOUR_TOKEN" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "claude-sonnet-5",
    "max_tokens": 1024,
    "messages": [{ "role": "user", "content": "hi~" }]
  }'`
  },
  {
    id: 'gemini',
    label: 'Gemini',
    endpoint: 'POST /gemini/v1beta/models/{model}:generateContent',
    code: `curl https://<your-modeltaps>/gemini/v1beta/models/gemini-3.5-flash:generateContent \\
  -H "x-goog-api-key: sk-YOUR_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "contents": [{ "role": "user", "parts": [{ "text": "hi~" }] }]
  }'`
  }
];

const TOOLS = ['Claude Code', 'Codex CLI', 'Cursor', 'Continue', 'Open WebUI'];

export default function ProtocolsSection() {
  const { t } = useTranslation();
  const [active, setActive] = useState(0);
  const tabRefs = useRef([]);

  const select = (index) => {
    const next = (index + PROTOCOLS.length) % PROTOCOLS.length;
    setActive(next);
    tabRefs.current[next]?.focus();
  };

  const onKeyDown = (event) => {
    const keys = { ArrowRight: active + 1, ArrowLeft: active - 1, Home: 0, End: PROTOCOLS.length - 1 };
    if (!(event.key in keys)) return;
    event.preventDefault();
    select(keys[event.key]);
  };

  const current = PROTOCOLS[active];

  return (
    <section className="border-b border-border">
      <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 sm:py-24">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-balance text-3xl font-semibold leading-snug sm:text-4xl">{t('home.protocols.title')}</h2>
          <p className="mt-4 text-pretty text-base leading-relaxed text-muted-foreground">{t('home.protocols.subtitle')}</p>
        </div>

        <div className="mt-14 overflow-hidden rounded-xl border border-border bg-card">
          <div role="tablist" aria-label={t('home.protocols.title')} className="flex flex-wrap gap-1 border-b border-border p-2">
            {PROTOCOLS.map((protocol, index) => (
              <button
                key={protocol.id}
                ref={(node) => {
                  tabRefs.current[index] = node;
                }}
                type="button"
                role="tab"
                id={`protocol-tab-${protocol.id}`}
                aria-selected={index === active}
                aria-controls={`protocol-panel-${protocol.id}`}
                tabIndex={index === active ? 0 : -1}
                onClick={() => setActive(index)}
                onKeyDown={onKeyDown}
                className={`rounded-lg px-4 py-2 text-sm font-semibold transition-colors motion-reduce:transition-none ${
                  index === active ? 'bg-foreground text-background' : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                }`}
              >
                {protocol.label}
              </button>
            ))}
          </div>

          <div
            role="tabpanel"
            id={`protocol-panel-${current.id}`}
            aria-labelledby={`protocol-tab-${current.id}`}
            tabIndex={0}
            className="p-4 sm:p-6"
          >
            <p className="font-mono text-[12.5px] font-semibold text-muted-foreground">{current.endpoint}</p>
            <pre className="mt-3 overflow-x-auto rounded-lg bg-muted p-4 font-mono text-[12.5px] leading-relaxed text-foreground">
              <code>{current.code}</code>
            </pre>
          </div>
        </div>

        <div className="mt-8 flex flex-wrap items-center justify-center gap-2.5">
          <span className="text-sm text-muted-foreground">{t('home.protocols.toolsLabel')}</span>
          {TOOLS.map((tool) => (
            <span key={tool} className="rounded-full border border-border px-3.5 py-1.5 text-[13px] font-medium text-foreground">
              {tool}
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}
