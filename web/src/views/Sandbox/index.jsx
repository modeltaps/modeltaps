import { useEffect, useState } from 'react';
import { Moon, Sun } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { toast } from '@/components/ui/sonner';

// ==============================|| SANDBOX — shadcn/ui foundation smoke test ||============================== //

export default function Sandbox() {
  const [dark, setDark] = useState(() => document.documentElement.classList.contains('dark'));

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle('dark', dark);
    root.classList.toggle('light', !dark);
    return () => {
      root.classList.remove('dark');
      root.classList.remove('light');
    };
  }, [dark]);

  return (
    <div className="min-h-screen bg-background text-foreground p-8">
      <div className="mx-auto max-w-2xl space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold">Sandbox</h1>
          <Button variant="outline" size="icon" aria-label="Toggle theme" onClick={() => setDark((v) => !v)}>
            {dark ? <Sun /> : <Moon />}
          </Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>shadcn/ui foundation OK</CardTitle>
            <CardDescription>Tailwind v4 + design tokens + 5 core primitives, rendered in {dark ? 'dark' : 'light'} mode.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-4">
              <Label htmlFor="sandbox-name">Name</Label>
              <Input id="sandbox-name" placeholder="Type something…" />
            </div>
            <div className="flex flex-wrap gap-2">
              <Button>Default</Button>
              <Button variant="secondary">Secondary</Button>
              <Button variant="outline">Outline</Button>
              <Button variant="ghost">Ghost</Button>
              <Button variant="link">Link</Button>
            </div>
          </CardContent>
          <CardFooter className="gap-2">
            <Button onClick={() => toast.success('Saved!', { description: 'shadcn/ui + sonner are wired up.' })}>
              Show toast
            </Button>
            <Button
              variant="destructive"
              onClick={() => toast.error('Deleted', { description: 'Destructive action fired a sonner toast.' })}
            >
              Delete
            </Button>
          </CardFooter>
        </Card>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="text-xl">Sizes</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center gap-2">
              <Button size="sm">Small</Button>
              <Button size="default">Default</Button>
              <Button size="lg">Large</Button>
              <Button size="icon" aria-label="icon">
                <Moon />
              </Button>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="text-xl">Muted surface</CardTitle>
              <CardDescription>Tokens drive both themes.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="rounded-md bg-muted p-4 text-muted-foreground text-sm">
                This block uses <code>bg-muted</code> / <code>text-muted-foreground</code>.
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
