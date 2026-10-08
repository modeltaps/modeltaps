import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { FormField } from '@/components/ui/form-field';
import { configText } from 'i18n/configText';
import pluginConfig from './pluginConfig';

export default function PluginForm({ type, value, onChange }) {
  const { t } = useTranslation();
  const plugins = pluginConfig[String(type)];
  if (!plugins) return null;

  const plugin = value && typeof value === 'object' ? value : {};

  const setParam = (pluginId, paramId, v) => {
    onChange({ ...plugin, [pluginId]: { ...(plugin[pluginId] || {}), [paramId]: v } });
  };

  return (
    <div className="space-y-6">
      {Object.entries(plugins).map(([pluginId, cfg], idx) => (
        <div key={pluginId} className={`space-y-3 ${idx > 0 ? 'border-t border-foreground/10 pt-6' : ''}`}>
          <div className="space-y-1">
            <p className="text-sm font-semibold text-foreground">{configText(t, cfg.name)}</p>
            <p className="text-xs text-muted-foreground">{configText(t, cfg.description)}</p>
          </div>
          {Object.entries(cfg.params).map(([paramId, param]) =>
            param.type === 'bool' ? (
              <div key={paramId} className="flex items-center justify-between gap-3">
                <div className="space-y-2.5">
                  <span className="text-sm">{configText(t, param.name)}</span>
                  <p className="text-xs text-muted-foreground">{configText(t, param.description)}</p>
                </div>
                <Switch
                  checked={!!plugin[pluginId]?.[paramId]}
                  onCheckedChange={(c) => setParam(pluginId, paramId, c)}
                />
              </div>
            ) : param.type === 'select' ? (
              <FormField key={paramId} row label={configText(t, param.name)} help={configText(t, param.description)}>
                <Select
                  value={plugin[pluginId]?.[paramId] ?? param.default ?? ''}
                  onValueChange={(v) => setParam(pluginId, paramId, v)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(param.options || []).map((o) => (
                      <SelectItem key={o.value} value={o.value}>{configText(t, o.label)}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
            ) : (
              <FormField key={paramId} label={configText(t, param.name)} help={configText(t, param.description)}>
                <Input
                  value={plugin[pluginId]?.[paramId] ?? ''}
                  onChange={(e) => setParam(pluginId, paramId, e.target.value)}
                />
              </FormField>
            )
          )}
        </div>
      ))}
    </div>
  );
}

PluginForm.propTypes = {
  type: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  value: PropTypes.object,
  onChange: PropTypes.func
};
