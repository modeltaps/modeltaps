import { useRef, useState } from 'react';
import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| AGENT DEVICE — CODE CELLS ||============================== //
// A device code (`BCDF-GHJK`) as two groups of four cells, the way an agent prints it, so the user
// can compare the two side by side. Editable, one real <input> lies over the cells and takes typing,
// paste and the caret; read-only, the same cells show the code being confirmed.

export const CODE_LENGTH = 8;

// Only letters count; the dash is drawn between the groups.
export const codeLetters = (raw) =>
  String(raw || '')
    .toUpperCase()
    .replace(/[^A-Z]/g, '')
    .slice(0, CODE_LENGTH);

function Cells({ letters, caret, invalid }) {
  const cell = (i) => (
    <span
      key={i}
      className={cn(
        'flex h-12 w-10 items-center justify-center rounded-lg border bg-field font-mono text-xl font-medium tabular-nums text-foreground transition-colors sm:h-14 sm:w-11',
        invalid ? 'border-destructive/60' : caret === i ? 'border-foreground/40 ring-2 ring-ring/20' : 'border-input'
      )}
    >
      {letters[i] || (caret === i ? <span className="h-5 w-px bg-foreground" /> : '')}
    </span>
  );
  return (
    <span className="flex items-center justify-center gap-1.5 sm:gap-2" aria-hidden="true">
      {[0, 1, 2, 3].map(cell)}
      <span className="w-2 text-center font-mono text-lg text-muted-foreground sm:w-3">-</span>
      {[4, 5, 6, 7].map(cell)}
    </span>
  );
}

Cells.propTypes = { letters: PropTypes.string, caret: PropTypes.number, invalid: PropTypes.bool };

export default function CodeCells({ id, value, onChange, onComplete, label, invalid = false, readOnly = false, autoFocus = false }) {
  const inputRef = useRef(null);
  const [focused, setFocused] = useState(false);
  const letters = codeLetters(value);

  if (readOnly) {
    return (
      <div role="img" aria-label={`${label}: ${letters.slice(0, 4)}-${letters.slice(4)}`} data-testid="device-code-shown">
        <Cells letters={letters} caret={-1} />
      </div>
    );
  }

  const caret = focused ? Math.min(letters.length, CODE_LENGTH - 1) : -1;
  return (
    <div className="relative">
      <Cells letters={letters} caret={letters.length < CODE_LENGTH ? caret : -1} invalid={invalid} />
      <input
        ref={inputRef}
        id={id}
        aria-label={label}
        aria-invalid={invalid || undefined}
        autoComplete="one-time-code"
        autoCapitalize="characters"
        autoCorrect="off"
        spellCheck={false}
        autoFocus={autoFocus}
        inputMode="text"
        maxLength={CODE_LENGTH + 4}
        value={letters.length > 4 ? `${letters.slice(0, 4)}-${letters.slice(4)}` : letters}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onChange={(e) => {
          const next = codeLetters(e.target.value);
          onChange(next);
          if (next.length === CODE_LENGTH && letters.length < CODE_LENGTH) onComplete?.(next);
        }}
        className="absolute inset-0 size-full cursor-text bg-transparent text-transparent caret-transparent outline-none selection:bg-transparent"
      />
    </div>
  );
}

CodeCells.propTypes = {
  id: PropTypes.string,
  value: PropTypes.string,
  onChange: PropTypes.func,
  onComplete: PropTypes.func,
  label: PropTypes.string.isRequired,
  invalid: PropTypes.bool,
  readOnly: PropTypes.bool,
  autoFocus: PropTypes.bool
};
