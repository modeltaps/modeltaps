import { describe, expect, it } from 'vitest';
import { PANEL_TABS } from './panelTabs';

const modelsTab = PANEL_TABS.find((tab) => tab.key === 'models');
const t = (key) => key;

describe('models tab draft', () => {
  it('reads the switched-off models', () => {
    expect(modelsTab.make({ models: 'a,b', disabled_models: ['b'] })).toEqual({ models: 'a,b', disabled: 'b' });
    expect(modelsTab.make({ models: 'a' })).toEqual({ models: 'a', disabled: '' });
  });

  it('sends only switched-off models still listed, renamed by the mapping', () => {
    const channel = { model_mapping: JSON.stringify({ alias: 'up' }) };
    const patch = modelsTab.patch({ models: 'up,b,c', disabled: 'up,gone,c' }, channel, { t });
    expect(patch).toEqual({ models: 'alias,b,c', disabled_models: ['alias', 'c'] });
  });

  it('sends an empty list once every model is back on', () => {
    expect(modelsTab.patch({ models: 'a', disabled: '' }, {}, { t })).toEqual({ models: 'a', disabled_models: [] });
  });
});
