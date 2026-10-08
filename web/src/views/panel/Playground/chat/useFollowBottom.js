import { useEffect, useRef } from 'react';

// ==============================|| PLAYGROUND — FOLLOW BOTTOM ||============================== //
// 消息区「跟着新消息走」：signal 变化（新一轮 / 流式增量）时把滚动容器拉到底。
// 是否跟随只在用户自己滚动时改判——距底 ≤ THRESHOLD 视为「贴着底看」，手动上翻超过这个
// 距离就停止强拉，回滚到底部附近又恢复。写完内容再量距离是量不准的（新内容本身就把距离
// 撑开了），所以状态记在 scroll 事件里。
// 容器高度变化（窗口缩放）时若仍处于跟随态，同样重新拉到底；手动上翻后不拉回。
// active 为 false（如对话还没有消息、区里是示例卡）时不跟随，停在顶部；变回 true 时恢复跟随并拉到底。

export const THRESHOLD = 80;

const distanceToBottom = (el) => el.scrollHeight - el.scrollTop - el.clientHeight;

// 给滚动容器挂上 scroll 与 resize 监听，返回清理函数。following 是 { current } 形状的跟随标记。
export function watchFollow(el, following, ResizeObserverImpl = globalThis.ResizeObserver) {
  const onScroll = () => {
    following.current = distanceToBottom(el) <= THRESHOLD;
  };
  el.addEventListener('scroll', onScroll, { passive: true });

  let lastHeight = el.clientHeight;
  const observer = ResizeObserverImpl
    ? new ResizeObserverImpl(() => {
        if (el.clientHeight === lastHeight) return;
        lastHeight = el.clientHeight;
        if (following.current) el.scrollTop = el.scrollHeight;
      })
    : null;
  observer?.observe(el);

  return () => {
    el.removeEventListener('scroll', onScroll);
    observer?.disconnect();
  };
}

export default function useFollowBottom(signal, active = true) {
  const ref = useRef(null);
  const following = useRef(active);

  useEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    return watchFollow(el, following);
  }, []);

  useEffect(() => {
    following.current = active;
    const el = ref.current;
    if (el && !active) el.scrollTop = 0;
  }, [active]);

  useEffect(() => {
    const el = ref.current;
    if (el && active && following.current) el.scrollTop = el.scrollHeight;
  }, [signal, active]);

  return ref;
}
