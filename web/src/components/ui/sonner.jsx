import * as React from 'react';
import { Toaster as Sonner, toast } from 'sonner';

// Detect the active theme from the `.dark` class on <html> and stay in sync
// with the sandbox toggle (no next-themes dependency in this repo).
function useHtmlTheme() {
  const getTheme = () =>
    typeof document !== 'undefined' && document.documentElement.classList.contains('dark') ? 'dark' : 'light';
  const [theme, setTheme] = React.useState(getTheme);

  React.useEffect(() => {
    const observer = new MutationObserver(() => setTheme(getTheme()));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
    return () => observer.disconnect();
  }, []);

  return theme;
}

const Toaster = ({ ...props }) => {
  const theme = useHtmlTheme();

  return (
    <Sonner
      theme={theme}
      className="toaster group"
      style={{
        '--normal-bg': 'var(--color-card)',
        '--normal-text': 'var(--color-card-foreground)',
        '--normal-border': 'var(--color-border)'
      }}
      toastOptions={{
        classNames: {
          toast:
            'group toast group-[.toaster]:bg-card group-[.toaster]:text-card-foreground group-[.toaster]:border-border group-[.toaster]:shadow-lg',
          description: 'group-[.toast]:text-muted-foreground',
          actionButton: 'group-[.toast]:bg-primary group-[.toast]:text-primary-foreground',
          cancelButton: 'group-[.toast]:bg-muted group-[.toast]:text-muted-foreground'
        }
      }}
      {...props}
    />
  );
};

export { Toaster, toast };
