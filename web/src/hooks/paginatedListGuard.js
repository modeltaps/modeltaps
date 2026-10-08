// ==============================|| 分页列表请求守卫(纯逻辑,UX-13 竞态根治) ||============================== //
// 与 React 解耦的纯函数,供 usePaginatedList 使用并可在 node 环境下直接单测。
// 参照 web/src/views/panel/Log/index.jsx fetchData 的 reqId 守卫写法。

// 请求序号守卫:begin() 领取新序号,isCurrent(id) 判断该序号是否仍是最新。
// 快速切页/切筛选/切组织时,旧请求完成后 isCurrent 为 false,响应被丢弃,不会覆盖新数据。
export function createRequestGuard() {
  let latest = 0;
  return {
    begin: () => ++latest,
    isCurrent: (id) => id === latest
  };
}

// 守卫化的 fetch 生命周期:只有仍为最新的请求才应用结果/报错/收尾;过期请求静默丢弃。
// onFinally 同样受守卫保护——旧请求完成不得清掉新请求的 loading 态。
export async function runGuardedFetch(guard, doFetch, { onStart, onResult, onError, onFinally } = {}) {
  const reqId = guard.begin();
  onStart?.();
  try {
    const result = await doFetch();
    if (guard.isCurrent(reqId)) onResult?.(result);
  } catch (error) {
    if (guard.isCurrent(reqId)) onError?.(error);
  } finally {
    if (guard.isCurrent(reqId)) onFinally?.();
  }
}
