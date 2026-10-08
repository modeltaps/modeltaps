import DOMPurify from 'dompurify';

// 运营可配内容（公告 / 首页 / 关于 / 更新说明）在进 dangerouslySetInnerHTML 前统一过一遍
// DOMPurify：剥离 script / iframe / on* 事件等可执行载荷，保留排版标签。
export function sanitizeHtml(html) {
  if (typeof html !== 'string' || html === '') return '';
  return DOMPurify.sanitize(html);
}
