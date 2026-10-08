// 试用区专用 Key 由后端 /api/token/playground 以固定名创建(controller/token.go),前端按名识别:
// 该 Key 由系统托管,列表中不提供复制明文与编辑入口(删除仍保留)。
export const PLAYGROUND_TOKEN_NAME = 'sys_playground';

export function isPlaygroundToken(token) {
  return token?.name === PLAYGROUND_TOKEN_NAME;
}
