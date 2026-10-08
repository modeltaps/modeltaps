import PropTypes from 'prop-types';
import { Loader2 } from 'lucide-react';

import ModelsError from 'views/panel/ApiCatalog/ModelsError';

// ==============================|| PLAYGROUND — WORKSPACE STATE ||============================== //
// 目录取数的两态在工作区居中呈现：加载中 / 加载失败(复用目录页的 ModelsError，含重试)。
// 无可用模型由各页自己画说明卡。取到数据就渲染 children。只吃 props。

export default function WorkspaceState({ loading, error, loadingLabel, onRetry, children }) {
  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center gap-2 p-6 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        {loadingLabel}
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 items-center justify-center p-6">
        <div className="w-full max-w-md">
          <ModelsError onRetry={onRetry} />
        </div>
      </div>
    );
  }

  return children;
}

WorkspaceState.propTypes = {
  loading: PropTypes.bool,
  error: PropTypes.bool,
  loadingLabel: PropTypes.string,
  onRetry: PropTypes.func,
  children: PropTypes.node
};
