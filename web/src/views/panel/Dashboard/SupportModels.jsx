import { useEffect, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ChevronDown, ChevronUp } from 'lucide-react';

import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import BrandIcon from '@/components/brand/BrandIcon';
import { API } from 'utils/api';
import { copy, showError } from 'utils/common';

// ==============================|| DASHBOARD — SUPPORTED MODELS ||============================== //
// shadcn equivalent of the v1 SupportModels card. Same `/api/available_model`
// source, grouped by provider; click a model badge to copy its name.

export default function SupportModels() {
  const { t } = useTranslation();
  const [modelList, setModelList] = useState({});
  const [expanded, setExpanded] = useState(false);
  const ownedby = useSelector((state) => state.siteInfo?.ownedby);

  const fetchModels = async () => {
    try {
      const res = await API.get('/api/available_model');
      const { data, success } = res.data;
      if (!success) return;

      const modelGroup = Object.entries(data).reduce((acc, [modelId, modelInfo]) => {
        const { owned_by } = modelInfo;
        if (!acc[owned_by]) acc[owned_by] = [];
        acc[owned_by].push(modelId);
        return acc;
      }, {});

      Object.values(modelGroup).forEach((models) => models.sort());

      const sorted = Object.keys(modelGroup)
        .sort((a, b) => {
          const ownerA = ownedby?.find((item) => item.name === a);
          const ownerB = ownedby?.find((item) => item.name === b);
          return (ownerA?.id || 0) - (ownerB?.id || 0);
        })
        .reduce((acc, key) => {
          acc[key] = modelGroup[key];
          return acc;
        }, {});

      setModelList(sorted);
    } catch (error) {
      showError(error.message);
    }
  };

  useEffect(() => {
    fetchModels();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const getIconByName = (name) => ownedby?.find((item) => item.name === name)?.icon;
  const entries = Object.entries(modelList);
  const preview = entries.slice(0, 1);

  return (
    <Card>
      <CardContent className="p-4">
        <div className="relative">
          <div className={`flex items-center gap-4 pr-10 ${expanded ? 'mb-4' : ''}`}>
            <span className="shrink-0 whitespace-nowrap text-sm text-muted-foreground">{t('dashboard_index.model_price')}:</span>

            {!expanded && (
              <div
                className="flex flex-1 gap-2 overflow-x-auto [&::-webkit-scrollbar]:hidden"
                style={{
                  scrollbarWidth: 'none',
                  maskImage: 'linear-gradient(to right, black 90%, transparent 100%)',
                  WebkitMaskImage: 'linear-gradient(to right, black 90%, transparent 100%)'
                }}
              >
                {preview.map(([provider, models]) => (
                  <div key={provider} className="flex items-center gap-2">
                    <span className="whitespace-nowrap text-sm font-semibold text-muted-foreground">{provider}:</span>
                    {models.map((model) => (
                      <Badge
                        key={model}
                        onClick={() => copy(model, t('dashboard_index.model_name'))}
                        className="cursor-pointer whitespace-nowrap border-transparent bg-muted text-foreground hover:bg-muted/70"
                      >
                        {model}
                      </Badge>
                    ))}
                  </div>
                ))}
                {entries.length === 0 && (
                  <span className="whitespace-nowrap text-sm text-muted-foreground">{t('dashboard_index.no_data')}</span>
                )}
              </div>
            )}
          </div>

          <div className="absolute right-0 top-[-2px] pl-1" style={{ background: 'linear-gradient(to right, transparent, var(--card) 20%)' }}>
            <Button
              variant="ghost"
              size="icon"
              className="size-7 text-muted-foreground hover:text-foreground"
              onClick={() => setExpanded((v) => !v)}
            >
              {expanded ? <ChevronUp className="size-5" /> : <ChevronDown className="size-5" />}
            </Button>
          </div>
        </div>

        {expanded && (
          <div className="flex flex-col gap-4">
            {entries.map(([provider, models]) => (
              <div key={provider}>
                <div className="mb-2 flex items-center gap-2">
                  <BrandIcon icon={getIconByName(provider)} ownedBy={provider} fallbackText={provider} className="size-4" />
                  <span className="text-sm font-semibold text-muted-foreground">{provider}</span>
                </div>
                <div className="grid grid-cols-2 gap-2 pl-1 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                  {models.map((model) => (
                    <Badge
                      key={model}
                      title={model}
                      onClick={() => copy(model, t('dashboard_index.model_name'))}
                      className="w-full cursor-pointer justify-start border-transparent bg-muted text-foreground hover:bg-muted/70"
                    >
                      <span className="truncate">{model}</span>
                    </Badge>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
