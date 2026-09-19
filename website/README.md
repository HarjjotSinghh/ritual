# ritual — website

The landing page for [ritual](https://github.com/HarjjotSinghh/ritual). Static,
no backend, and deliberately so: ritual never receives anyone's data, so this
site has nothing to receive it with.

```bash
npm install
npm run dev     # http://localhost:3000
npm run build
```

Next.js App Router, Tailwind v4, shadcn/ui primitives (`button`, `tabs`) on a
warm paper-and-ink token set defined in `app/globals.css`. Type is JetBrains
Mono for anything structural and Newsreader for prose, because a page about a
terminal tool should look like one without being a wall of monospace.

The theme is resolved in `app/layout.tsx` before first paint; `ThemeToggle`
persists the choice to `localStorage` and falls back to the system preference.
