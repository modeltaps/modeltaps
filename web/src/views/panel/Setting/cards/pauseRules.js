// Client-side check of the PauseRules option before it is saved: empty (built-in rules) or a JSON
// array. The backend validates the rules themselves and answers with its own message.
export const pauseRulesError = (value) => {
  if (!value || !String(value).trim()) return null;
  try {
    return Array.isArray(JSON.parse(value)) ? null : 'notArray';
  } catch {
    return 'invalidJson';
  }
};
