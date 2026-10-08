import js from '@eslint/js';
import { fixupPluginRules } from '@eslint/compat';
import prettierConfig from 'eslint-config-prettier';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import prettier from 'eslint-plugin-prettier';
import react from 'eslint-plugin-react';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';

// eslint-plugin-react and eslint-plugin-jsx-a11y still call context APIs that ESLint 10 removed;
// fixupPluginRules restores them until the plugins support ESLint 10 natively.
const reactPlugin = fixupPluginRules(react);
const jsxA11yPlugin = fixupPluginRules(jsxA11y);

export default [
  { ignores: ['build/', 'dist/'] },
  js.configs.recommended,
  {
    files: ['**/*.{js,jsx,mjs}'],
    plugins: {
      react: reactPlugin,
      'jsx-a11y': jsxA11yPlugin,
      'react-hooks': reactHooks,
      prettier
    },
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      parserOptions: { ecmaFeatures: { jsx: true, impliedStrict: true } },
      globals: { ...globals.browser, ...globals.node, ...globals.es2021 }
    },
    settings: { react: { version: 'detect' } },
    rules: {
      ...react.configs.recommended.rules,
      ...react.configs['jsx-runtime'].rules,
      ...jsxA11y.configs.recommended.rules,
      ...reactHooks.configs.recommended.rules,
      // eslint-plugin-react-hooks 7 adds React Compiler rules to "recommended" as errors.
      // The app does not use the compiler and predates them, so these report as warnings
      // until the flagged code is reworked; the other compiler rules stay errors.
      'react-hooks/set-state-in-effect': 'warn',
      'react-hooks/refs': 'warn',
      'react-hooks/purity': 'warn',
      'react-hooks/immutability': 'warn',
      'react-hooks/preserve-manual-memoization': 'warn',
      'react/jsx-uses-react': 'error',
      'react/jsx-uses-vars': 'error',
      'react/react-in-jsx-scope': 'off',
      'no-undef': 'error',
      'react/display-name': 'off',
      'react/jsx-filename-extension': 'off',
      'no-param-reassign': 'off',
      'react/prop-types': 'warn',
      'react/require-default-props': 'off',
      'react/no-array-index-key': 'off',
      'react/jsx-props-no-spreading': 'off',
      'react/forbid-prop-types': 'off',
      'no-console': 'off',
      'jsx-a11y/anchor-is-valid': 'off',
      'prefer-destructuring': 'off',
      'no-shadow': 'off',
      'jsx-a11y/no-autofocus': 'off',
      // ESLint 9 started reporting unused catch parameters by default; keep the ESLint 8 behaviour.
      'no-unused-vars': ['error', { ignoreRestSiblings: false, caughtErrors: 'none' }],
      'prettier/prettier': ['warn', { endOfLine: 'auto' }]
    }
  },
  // Last, so it switches off every stylistic rule that would fight Prettier.
  prettierConfig
];
