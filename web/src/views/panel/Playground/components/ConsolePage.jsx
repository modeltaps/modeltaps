import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| CONSOLE — PAGE SHELL ||============================== //
// 单列控制台页壳：左上标题 / 右上按钮区 / 主区（可滚动）/ 底部 composer。
// 主区与 composer 用同一个居中列宽（columnClassName），因此消息列与输入框左右边缘对齐。
// 标题与按钮区都不给时整块页眉不渲染——标题由外层页壳（MainLayout 的 PageHeader）提供的
// 页面用同一个组件，不必背一行空标题。只吃 props：不取数、不读路由、文案由上层传入。
// bodyRef 把主区这个滚动容器交给上层（如对话装配层的「跟着新消息走」），组件自己不管滚动。

export default function ConsolePage({ title, subtitle, actions, composer, columnClassName, className, bodyRef, children }) {
  const column = cn('mx-auto w-full max-w-[960px]', columnClassName);

  return (
    <div className={cn('flex h-full min-h-0 flex-col', className)}>
      {(title || actions) && (
        <header className="shrink-0 px-4 pt-4">
          <div className={cn(column, 'flex items-start gap-3')}>
            <div className="min-w-0 flex-1">
              {title && <h1 className="truncate text-xl font-semibold text-foreground">{title}</h1>}
              {subtitle && <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>}
            </div>
            {actions && <div className="flex shrink-0 items-center gap-1">{actions}</div>}
          </div>
        </header>
      )}

      <div ref={bodyRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
        <div className={cn(column, 'flex min-h-full flex-col')}>{children}</div>
      </div>

      {composer && (
        <div className="shrink-0 px-4 pb-4">
          <div className={column}>{composer}</div>
        </div>
      )}
    </div>
  );
}

ConsolePage.propTypes = {
  title: PropTypes.node,
  subtitle: PropTypes.node,
  actions: PropTypes.node,
  composer: PropTypes.node,
  columnClassName: PropTypes.string,
  className: PropTypes.string,
  bodyRef: PropTypes.object,
  children: PropTypes.node
};
