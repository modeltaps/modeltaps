import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Clock, Network, SquareTerminal } from 'lucide-react';

import BrandIcon from '@/components/brand/BrandIcon';
import UserAvatar from '@/components/chrome/avatar';
import { ContextMark } from '@/components/chrome/ContextSwitcher';
import { Dot } from '@/components/objects';
import { cn } from '@/lib/utils';

// ==============================|| AGENT DEVICE — REQUEST PARTS ||============================== //
// The pieces of the confirmation step: the agent and the account it connects to, side by side,
// and the facts of the request (client, network, time) to check before approving.

const TerminalMark = <SquareTerminal className="size-6 text-muted-foreground" strokeWidth={1.5} />;

// The agent's mark: its brand icon when recognized, a terminal glyph otherwise.
export function ClientMark({ client, className }) {
  return (
    <span className={cn('flex size-12 shrink-0 items-center justify-center rounded-xl border border-border bg-card', className)}>
      {client ? <BrandIcon brandKey={client.brandKey} domain={client.domain} fallback={TerminalMark} className="size-6" /> : TerminalMark}
    </span>
  );
}

ClientMark.propTypes = { client: PropTypes.object, className: PropTypes.string };

function OwnerMark({ user, org }) {
  if (org) return <ContextMark kind="org" org={org} className="size-12 rounded-xl" />;
  return <UserAvatar user={user} className="size-12" textClass="text-sm" />;
}

OwnerMark.propTypes = { user: PropTypes.object, org: PropTypes.object };

const Dots = () => (
  <span className="flex items-center gap-1" aria-hidden="true">
    {[0, 1, 2].map((i) => (
      <span key={i} className="size-1 rounded-full bg-foreground/20" />
    ))}
  </span>
);

// Agent ··· account: what approving connects.
export function Handshake({ client, user, org }) {
  return (
    <div className="mb-5 flex items-center justify-center gap-3" data-testid="device-handshake">
      <ClientMark client={client} />
      <Dots />
      <OwnerMark user={user} org={org} />
    </div>
  );
}

Handshake.propTypes = { client: PropTypes.object, user: PropTypes.object, org: PropTypes.object };

function Fact({ icon: Icon, children }) {
  return (
    <li className="flex min-w-0 items-start gap-2.5">
      <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" strokeWidth={1.5} aria-hidden="true" />
      <div className="min-w-0 flex-1">{children}</div>
    </li>
  );
}

Fact.propTypes = { icon: PropTypes.elementType.isRequired, children: PropTypes.node };

const clock = (s) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;

// Client, network and time of the request, as the agent sent them and the server saw them.
export function RequestFacts({ request, secondsLeft, now }) {
  const { t } = useTranslation();
  const minutesAgo = Math.max(0, Math.floor((now / 1000 - request.created_time) / 60));
  return (
    <ul className="space-y-2.5 text-sm" data-testid="device-facts">
      <Fact icon={SquareTerminal}>
        <span className="block truncate font-mono text-[13px]" title={request.request_user_agent}>
          {request.request_user_agent || t('agentDevice.noUserAgent')}
        </span>
      </Fact>
      <Fact icon={Network}>
        <span className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          <span className="font-mono text-[13px]">{request.request_ip || '—'}</span>
          {request.request_ip && (
            <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground" data-testid="device-network">
              <Dot tone={request.same_network ? 'live' : 'warn'} />
              {t(request.same_network ? 'agentDevice.sameNetwork' : 'agentDevice.otherNetwork')}
            </span>
          )}
        </span>
      </Fact>
      <Fact icon={Clock}>
        <span>
          {minutesAgo < 1 ? t('agentDevice.requestedJustNow') : t('agentDevice.requestedAgo', { count: minutesAgo })}
          <span className="text-muted-foreground">
            {' · '}
            {t('agentDevice.expiresIn')} <span className="font-mono tabular-nums">{clock(secondsLeft ?? 0)}</span>
          </span>
        </span>
      </Fact>
    </ul>
  );
}

RequestFacts.propTypes = { request: PropTypes.object.isRequired, secondsLeft: PropTypes.number, now: PropTypes.number.isRequired };
