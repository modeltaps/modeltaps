import { useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Bot, Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { InfoRow, InfoRows } from '@/components/objects';
import { API } from 'utils/api';
import { showError, timestamp2string } from 'utils/common';
import { resolveServerAddress } from 'utils/serverAddress';
import { CopyField } from './KeySecret';

// Handing an API key to an agent (AGENT-7). Every new key comes with a pairing code: the agent
// redeems it at POST /api/agent/pair and receives the key itself, so the key never goes through
// the chat. A code lasts ten minutes and works once; redeeming it gives the key a fresh secret, so
// the copy shown at creation stops working. The key's details issue new codes and show which agent
// claimed the key.

// Where agents read how to connect (AGENT-5).
export function useSkillUrl() {
  const siteInfo = useSelector((state) => state.siteInfo);
  return `${resolveServerAddress(siteInfo?.server_address)}/skill.md`;
}

// The pairing code with copy, how long it lasts, and what redeeming it does to the key.
export function PairingCodeReveal({ code, expiresAt, onCopy }) {
  const { t } = useTranslation();
  const skillUrl = useSkillUrl();
  const label = t('apiKey.pairing.label');
  return (
    <div className="space-y-1.5" data-testid="pairing-code">
      <CopyField label={label} value={code} onCopy={() => onCopy(code, label)} testId="pairing-code-value" />
      <p className="text-xs text-muted-foreground">
        {t('apiKey.pairing.hint', { url: skillUrl, time: expiresAt ? timestamp2string(expiresAt) : '' })}
      </p>
      <p className="text-xs text-muted-foreground">{t('apiKey.pairing.rotateNote')}</p>
    </div>
  );
}

PairingCodeReveal.propTypes = { code: PropTypes.string.isRequired, expiresAt: PropTypes.number, onCopy: PropTypes.func.isRequired };

const CLAIM_VIA = { pairing_code: 'apiKey.agent.viaPairingCode', device_flow: 'apiKey.agent.viaDeviceFlow' };

// The agent section of a key's overview: who claimed the key (time, IP, User-Agent), and a new
// pairing code for keys the viewer may hand over (`canIssue`, the same rule as reset).
export function AgentHandover({ token, canIssue, onCopy, onIssued }) {
  const { t } = useTranslation();
  const [issued, setIssued] = useState(null);
  const [busy, setBusy] = useState(false);
  const claim = token.agent_claim;

  const issue = async () => {
    if (busy || !window.confirm(t('apiKey.agent.newCodeConfirm'))) return;
    setBusy(true);
    try {
      const res = await API.post(`/api/token/${token.id}/pairing_code`);
      const { success, message, data } = res.data;
      if (success) {
        setIssued({ code: data.pairing_code, expiresAt: data.pairing_expires_at });
        onIssued?.();
      } else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  const pendingUntil = issued?.expiresAt || token.pairing_expires_at;
  return (
    <div className="space-y-3 py-2" data-testid="agent-handover">
      <InfoRows>
        <InfoRow label={t('apiKey.agent.claim')}>
          {claim ? (
            <span data-testid="agent-claim">{t('apiKey.agent.claimed', { via: t(CLAIM_VIA[claim.via] || CLAIM_VIA.pairing_code) })}</span>
          ) : (
            <span className="text-muted-foreground">{t('apiKey.agent.notClaimed')}</span>
          )}
        </InfoRow>
        {claim && (
          <>
            <InfoRow label={t('apiKey.agent.claimedAt')}>{timestamp2string(claim.claimed_at)}</InfoRow>
            {claim.client_name && <InfoRow label={t('apiKey.agent.client')}>{claim.client_name}</InfoRow>}
            <InfoRow label={t('apiKey.agent.ip')} mono>
              {claim.ip || '—'}
            </InfoRow>
            <InfoRow label={t('apiKey.agent.userAgent')} mono>
              {claim.user_agent || '—'}
            </InfoRow>
          </>
        )}
        {!issued && pendingUntil > 0 && (
          <InfoRow label={t('apiKey.pairing.label')}>{t('apiKey.agent.pendingUntil', { time: timestamp2string(pendingUntil) })}</InfoRow>
        )}
      </InfoRows>
      {issued && <PairingCodeReveal code={issued.code} expiresAt={issued.expiresAt} onCopy={onCopy} />}
      {canIssue && (
        <Button type="button" variant="outline" size="sm" className="gap-1.5 rounded-lg" onClick={issue} disabled={busy}>
          {busy ? <Loader2 className="size-3.5 animate-spin" /> : <Bot className="size-3.5" />}
          {t('apiKey.agent.newCode')}
        </Button>
      )}
    </div>
  );
}

AgentHandover.propTypes = {
  token: PropTypes.object.isRequired,
  canIssue: PropTypes.bool,
  onCopy: PropTypes.func.isRequired,
  onIssued: PropTypes.func
};
