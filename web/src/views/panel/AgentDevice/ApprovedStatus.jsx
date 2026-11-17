import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { Check, Loader2, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { timestamp2string } from 'utils/common';

// ==============================|| AGENT DEVICE — AFTER APPROVAL ||============================== //
// Approving is half the handshake: the agent still has to come for its key. This follows the
// request until it does (or runs out), so the user knows the agent is connected before leaving.

export const STATUS_POLL_MS = 3000;

function Step({ state, title, children }) {
  return (
    <li className="relative flex gap-3 pb-5 last:pb-0">
      <span
        className={cn(
          'relative z-10 flex size-6 shrink-0 items-center justify-center rounded-full border bg-background',
          state === 'done' && 'border-success/40 bg-success/10 text-success',
          state === 'wait' && 'border-border text-muted-foreground',
          state === 'fail' && 'border-border text-muted-foreground'
        )}
      >
        {state === 'done' && <Check className="size-3.5" strokeWidth={2.25} />}
        {state === 'wait' && <Loader2 className="size-3.5 animate-spin" />}
        {state === 'fail' && <X className="size-3.5" />}
      </span>
      <div className="min-w-0 pt-0.5">
        <p className={cn('text-sm font-medium', state === 'wait' && 'text-muted-foreground')}>{title}</p>
        {children && <div className="mt-0.5 text-xs text-muted-foreground">{children}</div>}
      </div>
    </li>
  );
}

Step.propTypes = { state: PropTypes.oneOf(['done', 'wait', 'fail']).isRequired, title: PropTypes.node, children: PropTypes.node };

export default function ApprovedStatus({ userCode, keyName, keysPath }) {
  const { t } = useTranslation();
  const [status, setStatus] = useState({ status: 'approved' });

  useEffect(() => {
    if (status.status !== 'approved') return undefined;
    let alive = true;
    const check = () =>
      API.get(`/api/agent/device/${encodeURIComponent(userCode)}/status`)
        .then((res) => alive && res.data.success && setStatus(res.data.data))
        .catch(() => {});
    const id = setInterval(check, STATUS_POLL_MS);
    check();
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [userCode, status.status]);

  const claimed = status.status === 'claimed';
  const expired = status.status === 'expired';

  return (
    <div data-testid="device-done">
      <div className="mb-6 text-center">
        <h1 className="text-xl font-medium tracking-[-0.01em]">
          {t(claimed ? 'agentDevice.connectedTitle' : expired ? 'agentDevice.notClaimedTitle' : 'agentDevice.approvedTitle')}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {t(claimed ? 'agentDevice.closeHint' : expired ? 'agentDevice.notClaimedHint' : 'agentDevice.waitingHint')}
        </p>
      </div>

      <ol
        className="relative rounded-xl border border-border bg-card p-5 before:absolute before:bottom-8 before:left-8 before:top-8 before:w-px before:bg-border sm:p-6 sm:before:left-9"
        data-testid="device-status"
      >
        <Step state="done" title={t('agentDevice.stepApproved')}>
          {t('agentDevice.stepApprovedHint', { name: keyName })}
        </Step>
        <Step
          state={claimed ? 'done' : expired ? 'fail' : 'wait'}
          title={t(claimed ? 'agentDevice.stepClaimed' : expired ? 'agentDevice.stepNotClaimed' : 'agentDevice.stepWaiting')}
        >
          {claimed && status.claim && (
            <span className="font-mono">{[status.claim.ip, timestamp2string(status.claim.claimed_at)].filter(Boolean).join(' · ')}</span>
          )}
        </Step>
      </ol>

      <div className="mt-6 text-center">
        <Button asChild variant="outline">
          <Link to={keysPath}>{t('agentDevice.openKeys')}</Link>
        </Button>
      </div>
    </div>
  );
}

ApprovedStatus.propTypes = { userCode: PropTypes.string.isRequired, keyName: PropTypes.string, keysPath: PropTypes.string.isRequired };
