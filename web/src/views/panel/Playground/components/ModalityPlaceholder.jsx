import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { Play, TriangleAlert } from 'lucide-react';

import { Button } from '@/components/ui/button';

// ==============================|| PLAYGROUND — MODALITY PLACEHOLDER ||============================== //
// 各模态真实内容落地前的占位：一句说明 + 运行按钮。门禁只作用在按钮上 —— 置灰 + 一句
// 说明 + 去向链接，页面其余部分照常可见。只吃 props。
// 未上线的模态(视频)用空页面形态：给 icon / docsHref、不给 runLabel，底部按钮行整条不渲染。

export default function ModalityPlaceholder({
  icon: Icon,
  title,
  description,
  docsHref,
  docsLabel,
  runLabel,
  gated,
  gateMessage,
  actionHref,
  actionLabel
}) {
  return (
    <div className="flex flex-1 flex-col">
      <div className="flex flex-1 items-center justify-center p-6">
        <div className="flex max-w-md flex-col items-center gap-2 text-center">
          {Icon && <Icon className="size-8 text-muted-foreground" />}
          <p className="text-sm font-medium text-foreground">{title}</p>
          <p className="text-xs text-muted-foreground">{description}</p>
          {docsHref && (
            <Button asChild variant="outline" size="sm" className="mt-2">
              <Link to={docsHref}>{docsLabel}</Link>
            </Button>
          )}
        </div>
      </div>

      {runLabel && (
        <div className="flex flex-wrap items-center justify-end gap-2 border-t border-border px-3 py-2.5">
          {gated && (
            <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <TriangleAlert className="size-3.5" />
              {gateMessage}
              {actionHref && (
                <Link to={actionHref} className="font-medium text-primary hover:underline">
                  {actionLabel}
                </Link>
              )}
            </span>
          )}
          <Button size="sm" disabled={gated}>
            <Play />
            {runLabel}
          </Button>
        </div>
      )}
    </div>
  );
}

ModalityPlaceholder.propTypes = {
  icon: PropTypes.elementType,
  title: PropTypes.string,
  description: PropTypes.string,
  docsHref: PropTypes.string,
  docsLabel: PropTypes.string,
  runLabel: PropTypes.string,
  gated: PropTypes.bool,
  gateMessage: PropTypes.string,
  actionHref: PropTypes.string,
  actionLabel: PropTypes.string
};
