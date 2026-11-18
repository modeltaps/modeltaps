import { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { Loader2, Pencil } from 'lucide-react';

import { cn } from '@/lib/utils';

// Title of an object panel that can be renamed in place: a click on it (or `editing` turned on
// from elsewhere, such as a "Rename" menu item) swaps it for an input. Enter or leaving the input
// saves through `onSave(name)`, which resolves to true when it worked; Escape, or an unchanged or
// empty name, leaves the title as it was.
export default function EditableTitle({ value, editing, onEditingChange, onSave, label }) {
  if (editing) return <TitleInput value={value} onDone={() => onEditingChange(false)} onSave={onSave} label={label} />;
  return (
    <button
      type="button"
      title={label}
      onClick={() => onEditingChange(true)}
      className={cn(
        'group -mx-1.5 flex max-w-full items-center gap-1.5 rounded-md px-1.5 text-left hover:bg-accent',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50'
      )}
    >
      <span className="truncate">{value}</span>
      <Pencil className="size-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100" />
    </button>
  );
}

// One editing session: mounted when editing starts, so it starts from the current name.
function TitleInput({ value, onDone, onSave, label }) {
  const [text, setText] = useState(value);
  const [saving, setSaving] = useState(false);
  const inputRef = useRef(null);
  // Escape and Enter end editing themselves: the blur that follows must not save again.
  const doneRef = useRef(false);

  useEffect(() => {
    // A menu that started editing gives focus back to its trigger when it closes: take it back.
    const id = setTimeout(() => {
      if (document.activeElement !== inputRef.current) inputRef.current?.focus();
    }, 0);
    return () => clearTimeout(id);
  }, []);

  const finish = async (save) => {
    if (doneRef.current) return;
    doneRef.current = true;
    const name = text.trim();
    if (!save || !name || name === value) {
      onDone();
      return;
    }
    setSaving(true);
    const ok = await onSave(name);
    setSaving(false);
    if (ok) onDone();
    else {
      doneRef.current = false;
      inputRef.current?.focus();
    }
  };

  return (
    <span className="flex items-center gap-2">
      <input
        ref={inputRef}
        autoFocus
        aria-label={label}
        value={text}
        disabled={saving}
        onFocus={(e) => e.target.select()}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => finish(true)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            finish(true);
          } else if (e.key === 'Escape') {
            // Escape would also close the panel.
            e.preventDefault();
            e.stopPropagation();
            finish(false);
          }
        }}
        className="-mx-1.5 h-7 w-full min-w-0 rounded-md border border-input bg-field px-1.5 text-base font-medium outline-none focus-visible:border-primary"
      />
      {saving && <Loader2 className="size-4 shrink-0 animate-spin text-muted-foreground" />}
    </span>
  );
}

TitleInput.propTypes = {
  value: PropTypes.string.isRequired,
  onDone: PropTypes.func.isRequired,
  onSave: PropTypes.func.isRequired,
  label: PropTypes.string.isRequired
};

EditableTitle.propTypes = {
  value: PropTypes.string.isRequired,
  editing: PropTypes.bool.isRequired,
  onEditingChange: PropTypes.func.isRequired,
  onSave: PropTypes.func.isRequired,
  label: PropTypes.string.isRequired
};
