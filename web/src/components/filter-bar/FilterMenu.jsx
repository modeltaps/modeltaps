import PropTypes from 'prop-types';
import { useMemo, useState } from 'react';
import { ChevronRight, Search } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { isEntryActive } from './utils';

// ==============================|| FILTER BAR — LEVEL 1 CATEGORY MENU ||============================== //
// 顶部「跳转到…」搜索 + 分类列表;已有筛选的分类右侧显示计数徽标。选中分类回调 onPick(field)。
// 悬停分类回调 onHover(field)(驱动右侧 flyout 展开/切换),离开回调 onHoverEnd();
// activeKey 为当前展开的分类(高亮)。specials 为附加入口(如 ID 查找),点击回调 onSpecial(special)。

export default function FilterMenu({ fields, state, specials = [], activeKey, onPick, onHover, onHoverEnd, onSpecial, t }) {
  const [q, setQ] = useState('');

  const visibleFields = useMemo(() => {
    const lower = q.trim().toLowerCase();
    if (!lower) return fields;
    return fields.filter((f) => t(f.labelKey).toLowerCase().includes(lower));
  }, [fields, q, t]);

  const visibleSpecials = useMemo(() => {
    const lower = q.trim().toLowerCase();
    if (!lower) return specials;
    return specials.filter((s) => t(s.labelKey).toLowerCase().includes(lower));
  }, [specials, q, t]);

  return (
    <div className="w-64">
      <div className="p-2">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={t('filterBar.jumpTo')}
            className="h-8 pl-8"
          />
        </div>
      </div>
      <div className="max-h-72 overflow-y-auto px-1 pb-1">
        {visibleFields.length === 0 && visibleSpecials.length === 0 ? (
          <div className="py-6 text-center text-sm text-muted-foreground">{t('filterBar.empty')}</div>
        ) : (
          <>
            {visibleFields.map((f) => {
              const active = isEntryActive(f, state[f.key]);
              const count = active ? (f.type === 'number' || f.type === 'text' ? 1 : state[f.key].values.length) : 0;
              return (
                <button
                  key={f.key}
                  type="button"
                  onClick={() => onPick(f)}
                  onMouseEnter={() => onHover?.(f)}
                  onMouseLeave={() => onHoverEnd?.()}
                  aria-haspopup="true"
                  aria-expanded={activeKey === f.key}
                  className={cn(
                    'flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none hover:bg-muted',
                    activeKey === f.key && 'bg-muted'
                  )}
                >
                  <span className="truncate">{t(f.labelKey)}</span>
                  <span className="flex items-center gap-1">
                    {count > 0 && (
                      <Badge variant="secondary" className="px-1.5">
                        {count}
                      </Badge>
                    )}
                    <ChevronRight className="size-4 text-muted-foreground" />
                  </span>
                </button>
              );
            })}
            {visibleSpecials.map((s) => (
              <button
                key={s.key}
                type="button"
                onClick={() => onSpecial(s)}
                onMouseEnter={() => onHoverEnd?.()}
                className="flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none hover:bg-muted"
              >
                <span className="flex items-center gap-2 truncate">
                  {s.icon && <s.icon className="size-4 text-muted-foreground" />}
                  <span className="truncate">{t(s.labelKey)}</span>
                </span>
                <ChevronRight className="size-4 text-muted-foreground" />
              </button>
            ))}
          </>
        )}
      </div>
    </div>
  );
}

FilterMenu.propTypes = {
  fields: PropTypes.array.isRequired,
  state: PropTypes.object.isRequired,
  specials: PropTypes.array,
  activeKey: PropTypes.string,
  onPick: PropTypes.func.isRequired,
  onHover: PropTypes.func,
  onHoverEnd: PropTypes.func,
  onSpecial: PropTypes.func,
  t: PropTypes.func.isRequired
};
