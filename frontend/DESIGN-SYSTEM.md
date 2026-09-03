# Quorum Glassmorphism Design System

## 1. Purpose and operating rules

Quorum uses a calm, high-clarity glassmorphism system for operational interfaces. It makes dense service information feel layered and responsive without making the interface decorative, ambiguous, or hard to read. The visual language is deliberately light, blue-dominant, and data-first.

This document is the canonical frontend visual contract. It governs every route, feature, entity, shared primitive, responsive state, and user-visible system state in `frontend/src`. Read it with `frontend/AGENTS.md` before changing UI or global styles.

The system has five non-negotiable principles:

1. Clarity over effect. Blur and translucency create hierarchy; they never carry essential information.
2. One visual action hierarchy. A view has one primary action, supporting actions, then quiet utility actions.
3. Stable semantics. Blue means interaction, status colours mean status, and slate means content or structure.
4. Layered restraint. A screen has a canvas, first-level glass panels, and optionally one nested glass level. It never has glass inside glass inside glass.
5. Accessible by construction. Keyboard focus, readable text, semantic HTML, responsive layouts, loading states, and reduced-motion support are requirements, not polish.

The system is implemented in `src/app/styles.css` using Tailwind CSS v4 `@theme` and `@utility`. There is no Tailwind configuration file. Components use local, shadcn-compatible primitives under `src/shared/ui`; domain composition belongs in `entities`, `features`, and `pages` according to the Feature-Sliced Design contract.

## 2. System map

| Layer | Responsibility | Visual responsibility |
|---|---|---|
| `src/app/styles.css` | Global tokens and utilities | Canvas, type tokens, glass anatomy, focus, motion, preference fallbacks |
| `src/shared/ui` | Product-agnostic primitives | Consistent interaction, sizing, state treatment, and accessible Radix wrappers |
| `src/entities` | Reusable business views | Domain status, API errors, request metadata, and reusable feedback patterns |
| `src/features` | One user action | Forms, mutation feedback, loading, empty, and error outcomes |
| `src/pages` | Route composition | Container width, grid, panel order, responsive rhythm, header, and footer |

Do not create one-off CSS files, page-specific token values, or component-local visual systems. Use Tailwind utilities for composition and `cn` from `src/shared/lib/utils.ts` whenever classes are combined. A shared primitive may gain a general prop; it must not be changed to solve a single screen's styling problem.

## 3. Foundations

### 3.1 Token source of truth

Use semantic tokens when a choice represents a role and scale tokens when a choice represents a calibrated value. The semantic set makes a future theme possible without requiring components to understand a colour formula.

| Token family | Tokens | Meaning |
|---|---|---|
| Canvas | `canvas` | The fixed, soft blue page background |
| Ink | `ink`, `ink-muted`, `ink-subtle` | Primary, secondary, and supporting content |
| Surfaces | `surface-panel`, `surface-nested`, `surface-control`, `surface-raised`, `surface-inverse` | First-level, nested, control, overlay, and inverse surfaces |
| Edges | `edge-subtle`, `edge-default`, `edge-strong` | Glass borders and highlight intensity |
| Brand | `brand-50` through `brand-900` | Interaction, selection, focus, and information accent |
| Status | `emerald-500`, `amber-500`, `red-500` | Success, warning, and error only |
| Data accents | `purple-500`, `cyan-500` | Optional data series; never action or status meaning |

Existing `slate-*` utilities remain the standard neutral scale for text, code, and separators. Do not invent near-duplicate greys or use arbitrary hex values in JSX.

### 3.2 Brand and neutral scale

| Token | Value | Approved use |
|---|---:|---|
| `brand-50` | `#eff6ff` | Ghost hover and quiet selected backgrounds |
| `brand-100` | `#dbeafe` | Active tint and subtle focus context |
| `brand-200` | `#bfdbfe` | Text selection background |
| `brand-500` | `#2563eb` | Primary buttons, links, selected controls, focus outline |
| `brand-600` | `#1d4ed8` | Primary hover, active link, insertion caret |
| `brand-700` | `#1e40af` | Primary pressed state |
| `slate-900` | `#0f172a` | Primary text and strongest neutral content |
| `slate-700` | `#334155` | Body copy and default field labels |
| `slate-600` | `#475569` | Supporting text and metadata |
| `slate-500` | `#64748b` | Quiet labels, placeholders, and unavailable values |
| `slate-300` | `#cbd5e1` | Neutral control edge |

Blue is the only interactive colour. It is not a success colour. Emerald, amber, and red must always be paired with a visible label, icon, or descriptive text. A red or amber border alone is not a complete error or warning treatment.

### 3.3 Typography

The font stack is Roboto, then the system sans-serif fallback. Use the predefined text utilities instead of hand-authored size and line-height pairs.

| Utility | Size / line height | Weight | Use |
|---|---|---:|---|
| `text-display` | `36px / 40px` | 700 | Rare landing or empty-state hero title |
| `text-h1` | `30px / 36px` | 700 | Route title when it is the page's principal heading |
| `text-h2` | `24px / 32px` | 600 | Compact route/header title |
| `text-h3` | `20px / 28px` | 600 | Panel, dialog, and section heading |
| `text-body` | `16px / 24px` | 400 | Default explanatory copy |
| `text-small` | `14px / 20px` | 400 | Dense body content and controls |
| `text-caption` | `12px / 16px` | 500 | Labels, metadata, timestamps, and concise help |

Use sentence case for headings, labels, and buttons. Lead with a meaningful noun or verb: “Send echo”, “Request budget”, or “Retry in 12 seconds”. Avoid title case, all caps, decorative punctuation, and vague verbs such as “Submit” when a specific action is available. Use `font-mono tabular-nums` for identifiers, latency, counts, status codes, and other values that change in place. Long IDs wrap with `break-all` inside a contained code surface.

### 3.4 Spacing, size, and radii

The base spacing unit is 4px. Use Tailwind's default scale and keep gaps intentional.

| Context | Standard |
|---|---|
| Inline icon and label | `gap-1.5` or `gap-2` |
| Label and input | `space-y-2` |
| Related result lines | `space-y-2` or `space-y-3` |
| Panel content | `space-y-3` or `space-y-4` |
| Panel padding | `p-6`; use `p-4` only for a compact nested item |
| Screen grid | `gap-6` |
| Route vertical padding | `py-8` on standard console routes |
| Minimum interactive target | 40px by 40px (`h-10`, `min-h-10`, or `size-10`) |

Use `rounded-md` (8px) for compact controls and code, `rounded-lg` (12px) for buttons, cards, alerts, and standard controls, and `rounded-xl` (16px) for first-level panels, popovers, and dialogs. Do not mix arbitrary radii on a single composed area.

## 4. Glass surfaces and elevation

Glass is a surface treatment, not a background colour. Its contrast is created by a soft canvas, translucent white fill, border highlight, background blur, and a restrained cool shadow. Text always sits on a surface with enough opacity for its final contrast.

| Utility | Layer | Use | Anatomy |
|---|---|---|---|
| `glass-panel` | 1 | Main cards and durable page sections | 62% white, 18px blur, 65% white edge, blue shadow, top highlight |
| `glass-card` | 2 | Compact nested rows and code/result wells | 45% white, 14px blur, subtle edge and shadow |
| `glass-input` | Control | Text inputs, textareas, and select triggers | 70% white, 8px blur, slate edge, built-in states |
| `glass-navigation` | Navigation | Route header and persistent utility bars | 50% white, bottom edge, 18px blur |
| `glass-popover` | Overlay | Menus, select content, dialogs, and transient raised content | 78% white, 22px blur, strong edge, raised shadow |
| `glass-code` | Inline/data | Inline IDs, endpoints, commands, and machine-readable values | Quiet white code chip with a border |
| `glass-divider` | Structure | Borders that separate regions within the same canvas | White 60% divider edge |
| `glass-skeleton` | Loading | Decorative placeholder blocks | Translucent block with a restrained sheen |

Use one first-level surface per major content block. A first-level panel may contain `glass-card` items, but those nested items may not contain another glass surface. Overlays use `glass-popover`, never `glass-panel`. Do not use opaque grey cards to imitate depth, dark glass on the light canvas, strong shadows on every child, or blur behind small text that is directly on the page background.

### Surface recipes

| Situation | Recipe |
|---|---|
| Dashboard panel | `Card` with its default first-level surface, `CardHeader`, then `CardContent` |
| Nested dependency/result item | `Card nested` with `p-3`, or a status-specific border/tint applied to that second-level surface |
| Error or success feedback | `rounded-lg border p-3` plus a low-opacity semantic tint and explicit icon/text |
| Inline identifier | `glass-code px-1.5 py-0.5 text-caption` |
| Menu/dialog | Shared Radix wrapper; do not recreate portal, focus trapping, or overlay behaviour |

## 5. Layout and responsive system

The service console uses a centered `max-w-6xl` container. Its default outer gutter is `px-4`, increasing to `sm:px-6`. A normal route has a compact glass navigation header, a `py-8` main area, and a quiet footer. The page shell is `relative min-h-screen overflow-x-clip` so oversized content cannot create horizontal scrolling.

| Viewport | Layout rule |
|---|---|
| 320–639px | One column, 16px gutter, controls may wrap, dialogs retain 16px page inset |
| 640–767px | 24px gutter, preserve one-column reading order unless a composition specifically benefits from two compact items |
| 768–1023px | Two-column dashboard grid when each panel remains readable; full-width panels span both columns |
| 1024px and above | Keep the `max-w-6xl` content cap; do not stretch dashboard cards into long, hard-to-scan rows |

Mobile order is desktop reading order. Never use CSS ordering to create a different narrative on small screens. Forms stack their label, help, field, error, and action vertically when space is constrained. Long URLs and request IDs must wrap, status metadata must flex-wrap, and important buttons must retain their 40px target.

## 6. Components

### 6.1 Shared primitives

| Primitive | Contract |
|---|---|
| `Button` | Three variants: `primary`, `secondary`, and `ghost`. Default height is 40px. Primary is the sole prominent action; secondary is a glass alternative; ghost is a low-emphasis action or link-like control. |
| `Card` | First-level `glass-panel` by default. Set `nested` only for its immediate child surface. Use the supplied header, title, description, content, and footer subcomponents for consistent rhythm. |
| `Badge` | Compact label, not a button. Use default for neutral/informational context and semantic variants only for status. |
| `Input`, `Textarea`, `SelectTrigger` | Use the shared control rather than rebuilding field states. Labels remain external semantic `<label>` elements. |
| `Dialog`, `DropdownMenu`, `SelectContent` | Radix-based, portal-aware overlays. They use `glass-popover`, animation, focus management, and accessible keyboard behaviour. |
| `Skeleton` | Decorative only. Marked `aria-hidden`; the parent carries loading announcement and `aria-busy` when required. |
| `Meter` | Numeric state paired with a text value. It exposes `role="meter"`; never use it as the sole presentation of a quota or threshold. |
| `StatusDot` | Decorative visual reinforcement for a textual status. It is intentionally `aria-hidden`. |
| `RequestId`, `RequestMetaRow`, `CopyButton`, `JsonBlock` | Operational-data primitives. Preserve monospaced formatting, allow wrap, and keep copy controls accessible. |

Before adding a primitive, confirm it is product-agnostic and has at least two credible consumers. If not, compose it in the owning feature or entity. New primitives need an explicit props interface, named export, keyboard/focus state, loading/disabled behaviour where relevant, and colocated tests.

### 6.2 Button hierarchy and states

| Variant | Default | Hover | Pressed | Intended use |
|---|---|---|---|---|
| Primary | `bg-brand-500 text-white shadow-sm` | `bg-brand-600` | `bg-brand-700` | The one decisive action in a view or form |
| Secondary | White 60% glass with white edge and slate text | White 80% | White 90% | Alternative action that remains visible |
| Ghost | Brand text, transparent base | `bg-brand-50` | `bg-brand-100` | Low-emphasis, inline, or reversible action |

All button variants use `focus-ring`, disabled opacity, disabled pointer behaviour, and a 150ms `interactive-lift`. Use icons to reinforce a label, not replace it, unless the control is a familiar compact action with an `aria-label`. Icon-only controls still need a 40px target. Buttons expose `aria-busy` while an action is in progress, retain a stable label where helpful, and must never be replaced by a non-semantic clickable `div`.

### 6.3 Form controls

| State | Required treatment |
|---|---|
| Default | `glass-input`, slate-300 edge, legible slate-900 value, slate-500 placeholder |
| Hover | Slate-400 edge |
| Focus | Brand-500 edge and 3px brand halo; never remove the focus indication |
| Invalid | `aria-invalid="true"`, red-500 edge, red halo on focus, and programmatic error relationship |
| Disabled | Slate-100 fill, 60% opacity, `cursor-not-allowed`, and no interactive lift |
| Pending | Keep the value visible, use `aria-busy` on the pending control or result region, and prevent duplicate submission |

Every field has a visible label, unless a well-established label is visibly replaced by another accessible name. Help text and errors use `aria-describedby`; validation errors use concise corrective language. Place the field error immediately after the control. Do not rely on placeholder text as a label, show validation only through colour, clear typed input on a network failure, or disable a form without explaining why when the reason is not obvious.

### 6.4 Feedback and data states

Each feature state has a deliberate visual treatment.

| State | Pattern |
|---|---|
| Loading | Keep the panel shape stable with skeletons or an in-context pending label; set `aria-busy` on the live region |
| Empty / not requested | Quiet `text-caption text-slate-500` explanation; state what action or event will populate it |
| Success | Low-opacity emerald fill and edge, clear result copy, plus request metadata where available |
| Warning | Low-opacity amber fill and edge, explicit next step or limitation |
| Error | Low-opacity red fill and edge for failures, amber for recoverable/rate-limit conditions, alert semantics for urgent errors |
| Unavailable / unknown | Slate status and plain explanation; do not render a false success state |

Use `ApiErrorView` for API-envelope failures and `RequestMetaRow` after an operation where status, duration, or correlation is meaningful. Dynamic result blocks use `aria-live="polite"` unless the content needs immediate interruption. Repeated polling should not create disruptive announcements; announce meaningful state transitions and preserve the human-readable checked time.

### 6.5 Icons, code, and data visualisation

Use Lucide React icons at 14px for inline controls, 16px for standard buttons and menu items, 20px for compact identity marks, and 24px only for a prominent empty/error state. Icons inherit the adjacent text colour unless they communicate semantic status. Decorative icons use `aria-hidden="true"`.

Code, URLs, IDs, HTTP status, latency, dates, counts, and quota values use a monospaced face. Apply `tabular-nums` to values that update in place. Code blocks remain copyable/selectable and scroll only inside their own bounded area. Charts may introduce one data accent, purple or cyan, in addition to brand blue and semantic status colours. Each series needs a text label, tooltip or table equivalent, and a non-colour differentiator such as line style, marker, or direct label.

## 7. Motion and user preferences

Motion explains a state change; it does not decorate an idle screen. Glass overlays use `animate-glass-in` for entry, `animate-glass-out` for exit, and the backdrop uses the matching fade utilities. The normal entry duration is 180ms, normal exit is 140ms, and interaction transitions are 150ms. Do not add looping decorative animation. `glass-skeleton` is the sole allowed continuous motion pattern.

`prefers-reduced-motion: reduce` collapses animations and transitions and disables smooth scrolling. `prefers-reduced-transparency: reduce` removes backdrop filters and raises glass opacity. New visual work must continue to work with both preferences enabled. Do not communicate progress solely through motion.

## 8. Accessibility, quality, and browser behaviour

Normal text must meet 4.5:1 contrast against the final composited surface; large text must meet 3:1. Test text over the actual canvas and surface, not against the raw hex token alone. Use `focus-ring` on every custom interactive component. Ensure logical keyboard order, visible focus, `Escape` dismissal for overlays, focus trapping and restoration through the shared dialog, and semantic elements before ARIA additions.

Use `aria-live` for changing request results, errors, and concise progress messages. Avoid announcing decorative skeletons, status dots, or polling ticks. Respect zoom to 200%, narrow widths down to 320px, pointer targets of at least 40px, and user text-size settings. Do not prevent selection or copy from important operational data.

The system uses standard `backdrop-filter`; the build pipeline supplies browser prefixes. Do not hand-author prefixed backdrop declarations. The design must remain legible when transparency, blur, network fonts, or JavaScript animations are unavailable.

## 9. Implementation and review checklist

Before marking frontend work complete, verify all of the following:

- The route uses the shared container, responsive grid, and mobile reading order.
- Every major region has the correct canvas, panel, nested, control, or overlay layer; no surface is nested more than once.
- One primary action is visually evident, and every interactive target is at least 40px.
- Colour follows its semantic role and important meaning is repeated in text or iconography.
- Heading hierarchy, labels, helper text, monospaced data, and time/count formatting follow the type rules.
- Loading, empty, success, warning, error, disabled, hover, active, focus, and pending states are present where applicable.
- Keyboard interaction, visible focus, accessible names, error relationships, live regions, and reduced-motion/transparency behaviour are correct.
- The implementation uses `cn`, local shared primitives, relative imports, and only `src/app/styles.css` for authored CSS.
- Relevant component tests pass, then run `npm run lint`, `npm run typecheck`, and `npm run test -- --run` from `frontend`.

When the product gains a new route or shared component category, update this document and the relevant primitive tests in the same change. The goal is a system that remains cohesive as Quorum grows, not a frozen showcase of the current console.
