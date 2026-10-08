// ==============================|| MIDJOURNEY — TYPE MAPS ||============================== //
// Ported from views/Midjourney/type/Type.js. action/code/status are upstream MJ
// data values; their display text resolves via i18n (midjourneyPage.*).

export const ACTION_TYPE = {
  IMAGINE: { labelKey: 'midjourneyPage.action.imagine', color: 'primary' },
  UPSCALE: { labelKey: 'midjourneyPage.action.upscale', color: 'orange' },
  VARIATION: { labelKey: 'midjourneyPage.action.variation', color: 'default' },
  HIGH_VARIATION: { labelKey: 'midjourneyPage.action.highVariation', color: 'default' },
  LOW_VARIATION: { labelKey: 'midjourneyPage.action.lowVariation', color: 'default' },
  PAN: { labelKey: 'midjourneyPage.action.pan', color: 'secondary' },
  DESCRIBE: { labelKey: 'midjourneyPage.action.describe', color: 'secondary' },
  BLEND: { labelKey: 'midjourneyPage.action.blend', color: 'secondary' },
  SHORTEN: { labelKey: 'midjourneyPage.action.shorten', color: 'secondary' },
  REROLL: { labelKey: 'midjourneyPage.action.reroll', color: 'secondary' },
  INPAINT: { labelKey: 'midjourneyPage.action.inpaint', color: 'secondary' },
  ZOOM: { labelKey: 'midjourneyPage.action.zoom', color: 'secondary' },
  CUSTOM_ZOOM: { labelKey: 'midjourneyPage.action.customZoom', color: 'secondary' },
  MODAL: { labelKey: 'midjourneyPage.action.modal', color: 'secondary' },
  SWAP_FACE: { labelKey: 'midjourneyPage.action.swapFace', color: 'secondary' },
  UPLOAD: { labelKey: 'midjourneyPage.action.upload', color: 'secondary' }
};

export const CODE_TYPE = {
  1: { labelKey: 'midjourneyPage.code.submitted', color: 'primary' },
  21: { labelKey: 'midjourneyPage.code.waiting', color: 'orange' },
  22: { labelKey: 'midjourneyPage.code.duplicated', color: 'default' },
  0: { labelKey: 'midjourneyPage.code.notSubmitted', color: 'default' }
};

export const STATUS_TYPE = {
  SUCCESS: { labelKey: 'midjourneyPage.status.success', color: 'success' },
  NOT_START: { labelKey: 'midjourneyPage.status.notStart', color: 'default' },
  SUBMITTED: { labelKey: 'midjourneyPage.status.submitted', color: 'secondary' },
  IN_PROGRESS: { labelKey: 'midjourneyPage.status.inProgress', color: 'primary' },
  FAILURE: { labelKey: 'midjourneyPage.status.failure', color: 'orange' },
  MODAL: { labelKey: 'midjourneyPage.status.modal', color: 'default' }
};

// finish_time/start_time (ms) → { str, seconds } consumed-time. Mirrors v1.
export function deriveRequestTime(item, t) {
  if (!(item.finish_time > 0)) return { seconds: 0, str: '' };
  const seconds = (item.finish_time - item.start_time) / 1000;
  return { seconds, str: `${seconds.toFixed(2)} ${t('midjourneyPage.seconds')}` };
}
