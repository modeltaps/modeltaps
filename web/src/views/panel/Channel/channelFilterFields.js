import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';

// ==============================|| CHANNEL — FILTER FIELD DEFINITIONS ||============================== //
// 驱动可复用 FilterBar 的字段定义(App 专属配置,非组件本身)。后端 model.SearchChannelsParams
// 均为单值标量,故不用 Log 的多选数组:
//   文本(LIKE / 精确):models / test_model / key(精确) / other / base_url(name 由工具栏搜索框承担)
//   单选枚举(标量):  type / status / filter_tag(固定项)、group / tag(动态项)
// text / single enum 的 paramInclude 即字段键,与后端 form tag 对齐,刷新恢复与请求共用一套映射。

const textField = (key, labelKey, extra = {}) => ({ key, labelKey, type: 'text', ...extra });

const singleEnum = (key, labelKey, options) => ({
  key,
  labelKey,
  type: 'enum',
  single: true,
  supportsExclude: false,
  paramInclude: key,
  options
});

// 渠道类型选项来自 CHANNEL_OPTIONS(value=数字类型,text=展示名)。
const typeOptions = Object.values(CHANNEL_OPTIONS).map((o) => ({ value: String(o.value), label: o.text }));

const statusOptions = [
  { value: '1', labelKey: 'channel_index.enabled' },
  { value: '2', labelKey: 'channel_index.disabled' },
  { value: '3', labelKey: 'channel_index.speedTestDisabled' }
];

// filter_tag:1=仅显示带标签、2=仅显示标签代表行(0=全部,以 chip 缺省表示)。
const filterTagOptions = [
  { value: '1', labelKey: 'channel_index.filterTags' },
  { value: '2', labelKey: 'channel_index.onlyTags' }
];

export function buildChannelFilterFields({ groupOptions = [], tags = [] }) {
  return [
    textField('models', 'channel_index.modelName'),
    textField('test_model', 'channel_index.testModel'),
    textField('key', 'channel_index.channelKey', { opLabelKey: 'filterBar.opIsOne' }),
    textField('other', 'channel_index.otherParameters'),
    textField('base_url', 'channel_index.channelApiAddress'),
    singleEnum('type', 'channel_index.channelType', typeOptions),
    singleEnum('status', 'channel_index.status', statusOptions),
    singleEnum(
      'group',
      'channel_index.group',
      (groupOptions || []).map((g) => ({ value: g, label: g }))
    ),
    singleEnum('filter_tag', 'channel_index.filterTags', filterTagOptions),
    singleEnum(
      'tag',
      'channel_row.tag',
      (tags || []).map((o) => ({ value: o.tag, label: o.tag }))
    )
  ];
}

// URL 同步时先清空这些键再按当前 state 重写(切换后不残留)。
export const CHANNEL_FILTER_PARAM_KEYS = [
  'models',
  'test_model',
  'key',
  'other',
  'base_url',
  'type',
  'status',
  'group',
  'filter_tag',
  'tag'
];
