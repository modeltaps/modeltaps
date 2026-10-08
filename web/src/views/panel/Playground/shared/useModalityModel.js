import { useEffect, useMemo } from 'react';

import { filterModels, matchesRule, pickPreferredModel } from 'views/panel/ApiCatalog/availability';
import { useConsole } from './ConsoleContext';

// ==============================|| PLAYGROUND — MODALITY MODEL ||============================== //
// 顶栏的模型是全控制台共用的，各模态只认按自己规则过滤出的那部分：
// 顶栏选的模型不在候选里时（刚从别的模态切过来）落到推荐项，并同步回顶栏，
// 否则顶栏徽标还停在别的模态的模型上。rule 可随子方向切换（语音页 tts / stt）；
// 传数组时取各规则的并集（图像页：images 模型 + 对话出图模型），保持目录顺序。

const filterByRules = (models, rule) =>
  Array.isArray(rule) ? filterModels(models).filter((m) => rule.some((r) => matchesRule(m, r))) : filterModels(models, rule);

// 纯函数部分：候选、当前模型，以及是否需要把当前模型同步回顶栏。
export function resolveModalityModel(models, rule, selectedModel) {
  const options = filterByRules(models, rule);
  const model = options.find((item) => item.id === selectedModel?.id) || pickPreferredModel(options) || null;
  return { options, model, sync: Boolean(model) && model.id !== selectedModel?.id };
}

export default function useModalityModel(rule) {
  const { models, selectedModel, setSelectedModel } = useConsole();
  const { options, model, sync } = useMemo(() => resolveModalityModel(models, rule, selectedModel), [models, rule, selectedModel]);

  useEffect(() => {
    if (sync) setSelectedModel(model);
  }, [sync, model, selectedModel, setSelectedModel]);

  return { options, model };
}
