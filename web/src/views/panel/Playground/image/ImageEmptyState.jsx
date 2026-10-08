import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Camera, Mountain, Package, Palette, ScanFace, Shapes } from 'lucide-react';

import ExampleGrid from '../components/ExampleGrid';
import { aspectRatio, sizeTier } from './imageFields';

// ==============================|| PLAYGROUND — IMAGE EMPTY STATE ||============================== //
// 还没出图时的落地内容：2 行 × 3 列模板卡。每张卡除了提示词还带一份参数补丁（尺寸 / 张数 /
// 质量），点一下把提示词填进输入框、参数写进会话（当前模型不支持的取值由 resolveValues 回落）。
// 卡面文案全部走 i18n，本文件只登记图标与参数；卡底 tag 是参数摘要（比例 / 档位 / 张数），
// 与点选后参数 chip 上的写法一致。

export const IMAGE_EXAMPLES = [
  { id: 'poster', icon: Palette, values: { size: '1024x1536', n: 1 } },
  { id: 'product', icon: Package, values: { size: '1024x1024', n: 2 } },
  { id: 'portrait', icon: ScanFace, values: { size: '1024x1536', n: 1 } },
  { id: 'illustration', icon: Camera, values: { size: '1024x1024', n: 1 } },
  { id: 'icon', icon: Shapes, values: { size: '512x512', n: 4 } },
  { id: 'landscape', icon: Mountain, values: { size: '1536x1024', n: 1 } }
];

export const exampleTags = (values = {}) =>
  [aspectRatio(values.size), sizeTier(values.size), Number(values.n) > 0 ? `${Number(values.n)}x` : ''].filter(Boolean);

export default function ImageEmptyState({ onPick }) {
  const { t } = useTranslation();

  const items = IMAGE_EXAMPLES.map((example) => ({
    id: example.id,
    icon: <example.icon className="size-4" />,
    title: t(`playgroundConsole.image.examples.${example.id}.title`),
    description: t(`playgroundConsole.image.examples.${example.id}.desc`),
    tags: exampleTags(example.values),
    prompt: t(`playgroundConsole.image.examples.${example.id}.prompt`),
    values: example.values
  }));

  return (
    <ExampleGrid
      className="w-full"
      columns={3}
      items={items}
      onPick={onPick}
      title={t('playgroundConsole.image.empty.title')}
      subtitle={t('playgroundConsole.image.empty.subtitle')}
    />
  );
}

ImageEmptyState.propTypes = {
  onPick: PropTypes.func
};
