import { useCallback, useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { RotateCcw } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import PageActions from '@/components/chrome/PageActions';
import { showError } from 'utils/common';
import PolicyCallName from 'views/panel/RoutingPolicy/PolicyCallName';

// 托管: the platform's routing policies this context's keys can call by their bare name. Read-only;
// a policy the context overrides with its own policy of the same name is marked, since the bare name
// then reaches the own policy.
export default function ManagedPolicies({ api, filter }) {
  const { t } = useTranslation();
  const [policies, setPolicies] = useState(null);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const { success, message, data } = await api.listManaged();
      if (success) setPolicies(data || []);
      else {
        showError(message);
        setPolicies([]);
      }
    } catch (e) {
      setPolicies([]);
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="space-y-6">
      <PageActions>
        <Button variant="outline" size="sm" aria-label={t('userPage.refresh')} title={t('userPage.refresh')} onClick={refresh}>
          <RotateCcw className={loading ? 'size-4 animate-spin' : 'size-4'} />
        </Button>
      </PageActions>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        {filter}
        <p className="text-sm text-muted-foreground">{t('routingPolicy.managedIntro')}</p>
      </div>

      {policies === null ? (
        <div className="grid gap-4 md:grid-cols-2">
          <div className="h-36 animate-pulse rounded-xl bg-muted" />
          <div className="h-36 animate-pulse rounded-xl bg-muted" />
        </div>
      ) : policies.length === 0 ? (
        <Card className="py-10 text-center text-sm text-muted-foreground">{t('routingPolicy.managedEmpty')}</Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {policies.map((p) => (
            <Card key={p.name} className="group space-y-3 p-4">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <PolicyCallName name={p.name} hoverCopy />
                  {p.display_name && p.display_name !== p.name && <div className="text-sm">{p.display_name}</div>}
                </div>
                {p.overridden && (
                  <Badge variant="outline" className="font-normal" title={t('routingPolicy.managedOverriddenHint')}>
                    {t('routingPolicy.managedOverridden')}
                  </Badge>
                )}
              </div>
              {p.description && <p className="text-sm text-muted-foreground">{p.description}</p>}
              {/* Routes as one quiet line: label → target. */}
              {(p.routes || []).length > 0 && (
                <p className="text-sm">
                  {(p.routes || []).map((r, i) => (
                    <span key={r.label} title={r.description || undefined}>
                      {i > 0 && <span className="text-muted-foreground"> · </span>}
                      {r.label} <span className="text-muted-foreground">→</span> {r.target_model}
                    </span>
                  ))}
                </p>
              )}
              <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs text-muted-foreground">
                <dt>{t('routingPolicy.form.fallbackModel')}</dt>
                <dd className="truncate text-foreground">{p.fallback_model}</dd>
                <dt>{t('routingPolicy.form.decisionModel')}</dt>
                <dd className="truncate text-foreground">{p.decision_model}</dd>
                <dt>{t('routingPolicy.form.threshold')}</dt>
                <dd className="text-foreground">{Math.round(Number(p.threshold || 0) * 100)}%</dd>
              </dl>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

ManagedPolicies.propTypes = {
  api: PropTypes.object.isRequired,
  filter: PropTypes.node
};
