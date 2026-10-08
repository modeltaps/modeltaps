import { describe, expect, it } from 'vitest';
import { MODALITY_OPTIONS } from './Modality';
import { MODALITY_COLORS } from '../views/panel/ModelInfo/modelInfoHelpers';
import zhCN from '../i18n/locales/zh_CN.json';
import zhHK from '../i18n/locales/zh_HK.json';
import jaJP from '../i18n/locales/ja_JP.json';
import enUS from '../i18n/locales/en_US.json';

const LOCALES = { zh_CN: zhCN, zh_HK: zhHK, ja_JP: jaJP, en_US: enUS };
const MODALITIES = Object.keys(MODALITY_OPTIONS);

describe('modality vocabulary', () => {
  it.each(Object.entries(LOCALES))('%s has a label and tab label for every modality', (_, locale) => {
    for (const m of MODALITIES) {
      expect(locale.modelpricePage.modality[m], `modelpricePage.modality.${m}`).toBeTruthy();
      expect(locale.modelpricePage.modalityTab[m], `modelpricePage.modalityTab.${m}`).toBeTruthy();
    }
  });

  it('every modality has a badge color', () => {
    for (const m of MODALITIES) {
      expect(MODALITY_COLORS[m], m).toBeTruthy();
    }
  });
});
