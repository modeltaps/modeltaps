// ==============================|| API CATALOG — REGISTRY ||============================== //
// 模态目录注册表:每个模态一个数据文件,新增模态只需在此登记。
// 路由参数 :modality 直接取这里的 key;未登记的值由 index.jsx 回落到 DEFAULT_MODALITY。

import chat from './chat';
import image from './image';
import speech from './speech';
import video from './video';

export const CATALOG = {
  chat,
  image,
  speech,
  video
};

export const DEFAULT_MODALITY = 'chat';

export const getCatalogEntry = (modality) => CATALOG[modality] || null;
