const CJK = /[\u3000-\u303f\u3400-\u4dbf\u4e00-\u9fff\uff00-\uffef]/;

const TASK_CODE_PATTERNS = [
  /\b(SEC|UX|OPS|IA|UI|TK|OC|TOK|TST|LOGX?|DSH|MNY|FLT|THM|MNT|REL|DEV|ICON)-\d+\b/,
  /\b(Wave|Task)\s+[A-Za-z0-9]+\b/,
  /^[A-Z]{1,2}\d{1,2}(\/[A-Z]\d)?[:\uff1a\s]/,
  /\bW\d+-[A-Z0-9]+\b/,
  /\bP\d-\d\b/,
];

const FORBIDDEN_TRAILERS = [
  /co-authored-by:/i,
  /agent-id:/i,
  /linked-note-id:/i,
  /generated with/i,
  /noreply@anthropic\.com/i,
];

export default {
  extends: ['@commitlint/config-conventional'],
  plugins: [
    {
      rules: {
        'no-cjk': (parsed) => [
          !CJK.test(parsed.raw ?? ''),
          'commit message must be English only: no CJK or full-width characters',
        ],
        'no-task-code-in-header': (parsed) => {
          const header = parsed.header ?? '';
          const offender = TASK_CODE_PATTERNS.find((re) => re.test(header));
          return [
            !offender,
            'task codes must not appear in the header; put them in a "Refs:" footer',
          ];
        },
        'no-tool-trailers': (parsed) => {
          const raw = parsed.raw ?? '';
          const rest = raw.split('\n').slice(1).join('\n');
          const offender = FORBIDDEN_TRAILERS.find((re) => re.test(rest));
          return [
            !offender,
            'commit body and footer must not contain co-author, agent or "Generated with" trailers',
          ];
        },
      },
    },
  ],
  rules: {
    'type-enum': [
      2,
      'always',
      [
        'feat',
        'fix',
        'docs',
        'refactor',
        'test',
        'chore',
        'ci',
        'build',
        'perf',
        'style',
        'revert',
        'deps',
      ],
    ],
    'header-max-length': [2, 'always', 72],
    'subject-case': [2, 'never', ['sentence-case', 'start-case', 'pascal-case', 'upper-case']],
    'subject-full-stop': [2, 'never', '.'],
    'body-max-line-length': [2, 'always', 100],
    'no-cjk': [2, 'always'],
    'no-task-code-in-header': [2, 'always'],
    'no-tool-trailers': [2, 'always'],
  },
};
