import { z } from 'zod';

// Zod's JIT probes for eval with `new Function('')`; the page CSP forbids eval, so every page
// using Zod logged a CSP violation. Jitless mode skips the probe and parses without eval.
// Imported first by index.jsx so it runs before any schema is built.
z.config({ jitless: true });
