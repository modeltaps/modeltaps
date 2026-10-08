import PropTypes from 'prop-types';
import { ExternalLink } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';

// ==============================|| SETTINGS — SECURITY PAGE PRIMITIVES ||============================== //
// 账号安全页的一行:图标 + 标题 / 副标题 + 右侧唯一的动作。状态只用文字,不用带边框的标签,
// 右侧要么是一个按钮(本站动作,或带小箭头的外链),要么是一段灰字(如「当前设备」)。

export function MethodRow({ icon: Icon, title, subtitle, action, href, onAction, disabled = false, children }) {
  let control = children;
  if (control === undefined) {
    if (href && action) {
      control = (
        <Button asChild variant="outline" size="sm">
          <a href={href} target="_blank" rel="noopener noreferrer">
            {action}
            <ExternalLink className="opacity-60" aria-hidden="true" />
          </a>
        </Button>
      );
    } else if (action) {
      control = (
        <Button variant="outline" size="sm" onClick={onAction} disabled={disabled}>
          {action}
        </Button>
      );
    } else {
      control = null;
    }
  }
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border py-3 last:border-0">
      <div className="flex min-w-0 items-center gap-3">
        {Icon && (
          <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-foreground">
            <Icon className="size-[18px]" aria-hidden="true" />
          </span>
        )}
        <div className="min-w-0">
          <p className="text-sm font-medium">{title}</p>
          {subtitle && <p className="truncate text-xs text-muted-foreground">{subtitle}</p>}
        </div>
      </div>
      {control !== null && <div className="shrink-0">{control}</div>}
    </div>
  );
}

MethodRow.propTypes = {
  icon: PropTypes.elementType,
  title: PropTypes.node,
  subtitle: PropTypes.node,
  action: PropTypes.node,
  href: PropTypes.string,
  onAction: PropTypes.func,
  disabled: PropTypes.bool,
  children: PropTypes.node
};

// 小节标签:一张卡里的分组名(如「第三方登录」)。
export function RowGroupLabel({ children }) {
  return <p className="pb-1 pt-5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{children}</p>;
}

RowGroupLabel.propTypes = { children: PropTypes.node };

export function SecurityCard({ title, description, headerAction, children }) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
        <div className="space-y-1.5">
          <CardTitle className="text-lg">{title}</CardTitle>
          {description && <CardDescription>{description}</CardDescription>}
        </div>
        {headerAction}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

SecurityCard.propTypes = {
  title: PropTypes.node,
  description: PropTypes.node,
  headerAction: PropTypes.node,
  children: PropTypes.node
};
