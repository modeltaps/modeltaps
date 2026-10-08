import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { capabilityEmptyKey } from './availability';

// ==============================|| API CATALOG — CAPABILITY CHIPS ||============================== //
// 能力 chip 行：能力名 + 可用模型数，0 个改为虚线描边并按原因给文案
// （原因判定走 availability 的 capabilityEmptyKey，与小节徽标、模态卡同一份）。
// 模态页页头与概览页的模态卡共用本组件，两处 chip 的样式与数字口径一致。

export default function CapabilityChips({ sections, loading, error, className }) {
  const { t } = useTranslation();

  if (!sections?.length) return null;

  return (
    <div className={cn('flex flex-wrap gap-1.5', className)}>
      {sections.map(({ capability, models }) => (
        <span
          key={capability.id}
          className={cn(
            'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium',
            !loading && models.length > 0 ? 'border-border text-foreground' : 'border-dashed border-border text-muted-foreground'
          )}
        >
          {t(capability.titleKey)}
          {/* 取数期间只占位：模型数还没到，写「暂无可用模型」会把加载中说成不支持。 */}
          {loading ? (
            <span className="inline-block h-3 w-6 animate-pulse rounded bg-muted" />
          ) : (
            <span className="tabular-nums text-muted-foreground">
              {/* 取数失败时这里的 0 不代表站点不支持，只代表我们不知道；planned 仍说「即将上线」。 */}
              {!error && models.length > 0 ? models.length : t(capabilityEmptyKey(capability, { error }))}
            </span>
          )}
        </span>
      ))}
    </div>
  );
}

CapabilityChips.propTypes = {
  sections: PropTypes.array,
  loading: PropTypes.bool,
  error: PropTypes.bool,
  className: PropTypes.string
};
