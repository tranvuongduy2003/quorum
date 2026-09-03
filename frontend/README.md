# Quorum frontend

React 19 + TypeScript on Vite, Tailwind CSS v4, Radix primitives, oxlint, Vitest.

```bash
npm install
npm run dev
npm run lint
npm run test -- --run
npm run build
```

## Layout

Feature-Sliced Design, with the dependency rule `app/pages → features/entities → shared`.
Lower layers never import from higher ones, and features never import from each other.

| Directory | Responsibility |
|---|---|
| `src/app` | Bootstrap, providers, global styles and theme |
| `src/pages` | Route-level composition |
| `src/features/<area>` | One user action, with its own UI, state, API calls and `index.ts` |
| `src/entities/<area>` | Reusable business-entity views and types |
| `src/shared/api` | HTTP client and shared API contracts |
| `src/shared/ui` | Generic presentational components |
| `src/shared/lib` | Framework-independent utilities |

Every directory under `pages`, `features` and `entities` is a slice with exactly one public
entry point (`index.ts`) and only these segments: `ui`, `model`, `api`, `lib`.
`shared` has the segments `api`, `ui`, `lib`.

There are no path aliases — import by relative path.

The layout, import matrix, and naming rules are documented in `frontend/AGENTS.md`.

## Design language — blue glassmorphism

The theme lives in `@theme` inside `src/app/styles.css`. There is no `tailwind.config.js`
and Tailwind v4 does not want one.

### Surfaces

A screen is built in layers: a soft blue radial gradient on `body`, glass panels floating on
top of it, and at most one level of glass nested inside those. **Never stack more than two
levels of glass** — the blur compounds into noise.

| Utility | Level | Anatomy |
|---|---|---|
| `glass-panel` | First | white 62%, `blur(18px) saturate(140%)`, white 65% border, `shadow-glass`, top highlight line |
| `glass-card` | Second (nested) | white 45%, `blur(14px)`, `shadow-glass-soft` |
| `glass-input` | Controls | white 70%, `blur(8px)`, with hover, focus, invalid and disabled states |
| `glass-skeleton` | Loading | translucent block with a sheen sweep |
| `focus-ring` | Any control | 2px `brand-500` outline, 2px offset, on `:focus-visible` |

`Card` renders `glass-panel`, or `glass-card` with the `nested` prop. Prefer the components in
`src/shared/ui` over hand-rolling a surface.

### Colour

- **Blue is the only interactive colour.** `brand-500` for actions, links, focus rings and
  selected states; `brand-600` hover; `brand-700` active.
- **Status colours are only for status**: `emerald` success, `amber` warning, `red` error.
  Blue is never a success indicator, and status is never conveyed by colour alone — pair it
  with a text label.
- Slate carries text and neutral structure.
- Data visualisations may add **one** accent (`purple-500` or `cyan-500`), never for interaction.

### Type

Roboto, with the scale exposed as `text-display`, `text-h1`, `text-h2`, `text-h3`, `text-body`,
`text-small`, `text-caption` — each carrying its own line height and weight. Numeric data uses
`tabular-nums`.

`cn` in `src/shared/lib/utils.ts` teaches `tailwind-merge` about that scale, so
`text-h3` and a caller's `text-slate-600` merge instead of cancelling each other out. Always
merge classes through `cn`; never concatenate class strings by hand.

### Radius, shadow, motion

Radius is `rounded-md` 8px for small controls, `rounded-lg` 12px for standard controls,
`rounded-xl` 16px for panels. Shadows are `shadow-sm`, `shadow-glass`, `shadow-glass-soft`,
`shadow-lg`. Overlays animate with `animate-glass-in` / `animate-glass-out` and
`animate-fade-in` / `animate-fade-out`; all motion collapses under
`prefers-reduced-motion: reduce`.

Author only the standard `backdrop-filter` — Lightning CSS emits the `-webkit-` prefix during
the build. Declaring both by hand makes the minifier drop the unprefixed one.

### Before a screen is done

One primary action per view; secondary and ghost for the rest. Loading, empty, error, hover,
focus and disabled states all present. Every control has an accessible name and a visible
keyboard focus. Normal text holds 4.5:1 against the surface it actually renders on — which is
why error text is `red-600` and placeholders are `slate-500`, not the 500-level border colours.
Dynamic regions announce themselves with `aria-live` and `aria-busy`.

`src/App.tsx` is the running reference for all of the above.
