import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { PackageOpen } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

// ==============================|| CONSOLE — NO MODELS CARD ||============================== //
// 没有可用模型时摆在工作区中间的一张卡：图标 + 一句原因 + 一个出口按钮。
// 文案与去向都由上层传入（管理员与普通用户看到的不是同一句话），本组件不认识身份。

export default function NoModelsCard({ title, description, actionLabel, actionHref, className }) {
  return (
    <div className={cn('mx-auto flex max-w-md flex-col items-center gap-3 rounded-xl border border-border bg-card p-6 text-center', className)}>
      <span className="flex size-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <PackageOpen className="size-5" />
      </span>
      {title && <h2 className="text-sm font-semibold text-foreground">{title}</h2>}
      {description && <p className="text-xs leading-relaxed text-muted-foreground">{description}</p>}
      {actionHref && actionLabel && (
        <Button asChild size="sm" variant="outline">
          <Link to={actionHref}>{actionLabel}</Link>
        </Button>
      )}
    </div>
  );
}

NoModelsCard.propTypes = {
  title: PropTypes.node,
  description: PropTypes.node,
  actionLabel: PropTypes.node,
  actionHref: PropTypes.string,
  className: PropTypes.string
};
