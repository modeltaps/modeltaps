// The playground's API key is created by the backend under a fixed name (model.PlaygroundTokenName)
// the first time POST /api/token/playground issues a playground key; the console recognizes it by
// name. It is managed by the system: the list offers no edit or reset for it (delete stays).
export const PLAYGROUND_TOKEN_NAME = 'sys_playground';

export function isPlaygroundToken(token) {
  return token?.name === PLAYGROUND_TOKEN_NAME;
}
