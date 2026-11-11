import { useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Copy, ChevronRight } from 'lucide-react';

import { Segmented } from '@/components/objects';
import { resolveServerAddress } from 'utils/serverAddress';

// The base URLs clients send requests to, one per protocol the site serves. OpenAI is always on;
// Claude and Gemini only when their endpoints are enabled.
export function useApiAddresses() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const base = resolveServerAddress(siteInfo.server_address);
  return [
    { value: 'openai', label: t('token_index.openaiApi'), url: `${base}/v1` },
    ...(siteInfo.ClaudeAPIEnabled ? [{ value: 'claude', label: t('token_index.claudeApi'), url: `${base}/claude` }] : []),
    ...(siteInfo.GeminiAPIEnabled ? [{ value: 'gemini', label: t('token_index.geminiApi'), url: `${base}/gemini` }] : [])
  ];
}

// Where to send requests: a protocol switch (only with more than one protocol), the base URL with
// a copy button and a link to the API examples. Shown in the API Keys header's Connect popover
// and in a key's overview. A self-hosted address cannot be printed in the docs, hence here.
export default function ConnectInfo({ addresses, onCopy, className }) {
  const { t } = useTranslation();
  const [apiType, setApiType] = useState(addresses[0]?.value);
  const current = addresses.find((a) => a.value === apiType) || addresses[0];
  if (!current) return null;
  return (
    <div className={className}>
      {addresses.length > 1 && (
        <Segmented
          size="mini"
          label={t('apiKey.page.protocol')}
          value={current.value}
          onChange={setApiType}
          options={addresses.map(({ value, label }) => ({ value, label }))}
        />
      )}
      <div className="mt-2 flex min-w-0 items-center gap-1 rounded-md border border-border bg-muted/40 py-1 pl-2.5 pr-1">
        <code className="min-w-0 flex-1 select-all truncate font-mono text-[13px]">{current.url}</code>
        <button
          type="button"
          onClick={() => onCopy(current.url, t('token_index.apiAddress'))}
          className="flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
          aria-label={t('token_index.copy')}
          title={t('token_index.copy')}
        >
          <Copy className="size-3.5" />
        </button>
      </div>
      <Link
        to="/panel/api"
        className="mt-2 inline-flex items-center gap-0.5 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
      >
        {t('apiKey.page.examples')}
        <ChevronRight className="size-3.5" />
      </Link>
    </div>
  );
}

ConnectInfo.propTypes = {
  addresses: PropTypes.arrayOf(PropTypes.shape({ value: PropTypes.string, label: PropTypes.string, url: PropTypes.string })).isRequired,
  onCopy: PropTypes.func.isRequired,
  className: PropTypes.string
};
