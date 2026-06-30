# Design system

How this site implements the Vercel **Geist** design language in raw CSS (no component
libraries, zero runtime deps). Layered cascade:
`tokens → base → layout → components → chroma → vendor → print`
(`assets/css/00-layers.css`).

Verified by the automated visual, layout, contrast, and interaction tests over the built
`public/` tree. The one-off Geist probe scripts have been removed now that their findings
are codified in tokens and tests.

## Deliberate deviations from stock Geist

These are intentional and must not be "fixed":

- **Deeper text** for accessibility (no global `antialiased`; dark-mode only). Geist runs
  lighter.
- **17px root** (Geist ~16px) — the whole scale is tuned around it.
- **List-style home feed** (not a card grid).
- **Full-width framed `<main>`** with crosshair corners + rails; the reading column centers
  _inside_ the frame, so on wide viewports the header title (at the rail) sits left of the
  body text. Intentional grid language.
- **Weight-400 display headings** — confirmed in range: vercel.com/blog ships h1 at both 400
  and 600 across posts.

## Tokens (`02-tokens.css`)

### Color — light / dark

| role                               | light       | dark      |
| ---------------------------------- | ----------- | --------- |
| `--bg`                             | `#fafafa`   | `#0a0a0a` |
| `--bg-2` (elevated)                | `#fafafa`   | `#1a1a1a` |
| `--text`                           | `#171717`   | `#ededed` |
| `--muted`                          | `#4d4d4d`   | `#a1a1a1` |
| `--faint`                          | `#6e6e6e`   | `#8f8f8f` |
| `--code-bg` / `--surface`          | `#f2f2f2`   | `#1a1a1a` |
| `--surface-2` (hover/active fill)  | `#ebebeb`   | `#1f1f1f` |
| `--toggle-active` (raised segment) | `var(--bg)` | `#2e2e2e` |
| `--border`                         | `#ebebeb`   | `#2e2e2e` |
| `--border-strong`                  | `#c9c9c9`   | `#454545` |
| `--link` / `--visited`             | `#0062d1`   | `#52a8ff` |

Semantic alert scale: `{blue,green,purple,amber,red}-{200 fill / 400 border / 900 text}`,
each carrying its own dark value. Raw hex appears **only** in `08-print.css`; everywhere
else is tokens.

### Spacing — `.25rem` step (≈ px/4 at 17px root)

`--space-1 .25rem … --space-12 3rem`. Structural bands: `--rail` (clamp .5–2rem),
`--grid-top-band`, `--content-top-band`.

### Type scale (rendered px at 17px root)

`--fs-2xs .72rem` (eyebrows/meta) · `--fs-sm .85rem` (nav/meta/code/tables) ·
`--fs-md .9rem` (toc/excerpt) · `--fs-base 1rem` (body/h4) · `--fs-lg 1.1rem` (site title) ·
`--fs-list-title clamp(21→26px)`. Display: `--fs-h1 clamp(40→48)`, `--fs-h2 clamp(24→32)`,
`--fs-h3 clamp(20→24)`. Article body steps down to **17px/25.5 at ≤960px** (phones + iPad
portrait), **18px/28 above** (`--fs-article`/`--lh-article`, hard `max-width:960` step —
not a fluid ramp, which drifts smaller than Vercel mid-range).

### Radii / weights / motion

Radii: `--radius-sm 4px` (chips/badges) · `--radius 6px` (controls/boxes) ·
`--radius-lg 12px` · `--radius-pill 9999px`. Every radius in the codebase is tokenized.
Weights: 400/500/600 only. Motion: `--motion .2s`, `--motion-fast .15s`, shared
`--transition-control` (color/border/bg). Focus: `--ring` = `0 0 0 2px bg, 0 0 0 4px accent`
(the shadcn `ring-2 + ring-offset-2` pattern).

## Article vertical rhythm (matches Vercel)

- **24px** between every block (`--article-gap`) — paragraphs, lists, code, tables, alerts,
  blockquotes, mermaid all unified to this.
- **64px** lead-in above `h2` (`--article-h-top`), **44px** above `h3+`
  (`--article-h-top-sub`).

## Component catalog

| component                                                             | size                                 | radius        | notes                                         |
| --------------------------------------------------------------------- | ------------------------------------ | ------------- | --------------------------------------------- |
| Standalone icon button (search, hamburger, copy)                      | 32px, 16px glyph                     | 6px           | filled `--surface`, border `--border`         |
| Theme switcher                                                        | 32px pill, 14px icons, 26px segments | 6px / 4px seg | active segment raised via `--toggle-active`   |
| Badge / tag                                                           | 32px min, `4.25/8.5` pad             | 4px           | `--code-bg` fill                              |
| Pagination button                                                     | text, `8.5/17` pad                   | 6px           | outline variant (bg `--bg`)                   |
| Back-to-top FAB                                                       | 44px                                 | 6px           | fixed, outside reading column                 |
| Boxed content (code, alert, blockquote, table, summary, toc, mermaid) | —                                    | 6px           | border `--border` (alerts use variant border) |
| Eyebrow label (SITE/WRITING/MORE, section titles)                     | `--fs-2xs` 600                       | —             | uppercase, `0.04em`, `lh 1.2`, `--faint`      |

UI icons are **stroked** (`stroke-width: 2`, Lucide style); brand/social icons are
**filled**. Capitalization tiers: UPPERCASE eyebrows · Title-Case primary nav · lowercase
quiet meta (breadcrumbs + footer).

## Control-state matrix

- **rest → hover**: bordered buttons → fill `--surface-2`, border `--border-strong`, text
  full-contrast (uniform across nav icons, copy, pagination, tags, back-to-top). Prose
  links: animated gradient underline (0→100%).
- **focus-visible**: every interactive control gets `--ring`; article text links get a
  wrapping `2px` outline. No control lacks a visible focus indicator.
- **active**: pagination + theme-toggle press to `--surface-2`.

## Dark-mode rule

Drop-shadows vanish on `#0a0a0a`, so **elevation comes from surface lightness + border,
never shadow alone**: theme-switcher active segment is _lighter_ than its pill
(`--toggle-active #2e2e2e`); skip-link uses `--bg-2`; `--shadow-key` (kbd) is a solid-color
line. Hover legibility in dark relies on the border step (`#2e2e2e → #454545`).

## Accessibility

Deeper-than-Geist text for contrast; `contrast_test.go` covers light + dark. Focus rings on
all controls; 44px mobile touch targets for nav/tags/pagination/connect rows. Theme defaults
to **System** (follows OS), overridable. `prefers-reduced-motion` cancels
transitions/animations. Skip-link, `sr-only`, forced-colors fallback all present.

## Responsive

Single 640px breakpoint (+900px for the list-grid frame); type scales via `clamp()`.
Verified: **no horizontal overflow 320→1440px**; reading column fills width on small
screens, caps at 720px centered on desktop.
