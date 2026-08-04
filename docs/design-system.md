# Design system

How this site implements the Vercel **Geist** design language in raw CSS (no component
libraries, zero runtime deps). Layered cascade:
`tokens → base → layout → components → chroma → print` (`assets/css/00-layers.css`). Hugo
concatenates those ordered sources into one fingerprinted stylesheet.

Verified by the automated visual, layout, contrast, and interaction tests over the built
`public/` tree. The one-off Geist probe scripts have been removed now that their findings
are codified in tokens and tests.

## Runtime boundaries

Templates own semantic markup; behavior lives in small fingerprinted controllers:

- `site.js`: theme controls, navigation menu, and back-to-top behavior on every page.
- `command-palette.js`: one deferred native-dialog controller for quick navigation, global
  <kbd>G</kbd>-then-key shortcuts, and search. The much larger Pagefind API remains lazy and
  loads only after a non-empty query.
- `copy-code.js`: emitted only when the code-block render hook records a copyable block in
  the page store.
- The homepage uses Joshua Profitt's bare-tree photograph through immutable 720, 1440, and
  2400-pixel variants on the site's R2-backed CDN. The monochrome crop fills the text row on
  desktop, uses a contained 16:10 stage on tablet, and uses the same bottom-anchored,
  near-square crop on phones. The favicon remains a separate brand asset.
- `mermaid.js`: diagram-page-only; the much larger, version-pinned CDN runtime loads only
  when a diagram enters the viewport.

Only the pre-paint theme resolver and the configuration-bearing analytics bootstrap remain
inline. Analytics waits until five seconds after `load`, then yields to browser idle time.
The site shell uses `.site-footer` for global chrome; semantic footers inside articles and
blockquotes deliberately do not inherit its layout or print rules.

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
`--content-top-band`.

### Type scale (rendered px at 17px root)

`--fs-2xs .72rem` (eyebrows/meta) · `--fs-sm .85rem` (nav/meta/code/tables) ·
`--fs-md .9rem` (toc/excerpt) · `--fs-base 1rem` (body/h4/site title) · `--fs-lg 1.1rem` ·
`--fs-list-title clamp(23→26px)`. Display: `--fs-h1` steps 40→48px, `--fs-h2` stays 32px,
and `--fs-h3` steps 24→28px. Article body steps down to **17px/25.5 at ≤960px** (phones +
iPad portrait), **18px/28 above** (`--fs-article`/`--lh-article`, hard `max-width:960`
step).

### Radii / weights / motion

Radii: `--radius-sm 4px` (compact interior controls) · `--radius 6px` (icon controls/boxes)
· `--radius-lg 12px` · `--radius-pill 9999px` (text actions and metadata pills). Every
radius in the codebase is tokenized. Weights: 400/500/600 only. Motion: `--motion .2s`,
`--motion-fast .15s`, shared `--transition-control` (color/border/bg). Focus: `--ring` =
`0 0 0 2px bg, 0 0 0 4px accent` (the shadcn `ring-2 + ring-offset-2` pattern).

## Article vertical rhythm (matches Vercel)

- **24px** between every block (`--article-gap`) — paragraphs, lists, code, tables, alerts,
  blockquotes, mermaid all unified to this.
- **72px** lead-in above `h2` (`--article-gap` + `--article-h2-lead`), **64px** above `h3+`
  (`--article-gap` + `--article-h3-lead`).

## Component catalog

| component                                                             | size                                 | radius        | notes                                         |
| --------------------------------------------------------------------- | ------------------------------------ | ------------- | --------------------------------------------- |
| Command search trigger                                                | 32px desktop / 44px mobile           | pill          | text + shortcut desktop; icon-only mobile     |
| Menu trigger                                                          | 44px, 16px glyph                     | 6px           | opens an explicit compact popover             |
| Copy button                                                           | 32px, 16px glyph                     | 6px           | filled `--surface`, border `--border`         |
| Command palette                                                       | 640px max / viewport-bound on mobile | 12px          | lazy Pagefind, route/action shortcut hints    |
| Theme switcher                                                        | 32px pill, 14px icons, 26px segments | 6px / 4px seg | active segment raised via `--toggle-active`   |
| Tag / metadata action                                                 | 32px desktop / 44px mobile           | pill          | mono uppercase, transparent + strong border   |
| Pagination button                                                     | 32px desktop / 44px mobile           | pill          | outlined secondary action                     |
| Back-to-top FAB                                                       | 44px                                 | 6px           | fixed, outside reading column                 |
| Boxed content (code, alert, blockquote, table, summary, toc, mermaid) | —                                    | 6px           | border `--border` (alerts use variant border) |
| Menu group label                                                      | `--fs-2xs` / 400                     | —             | mono uppercase, `--faint`                     |
| Section/result eyebrow                                                | `--fs-2xs` 600                       | —             | uppercase, `0.04em`, `--faint`                |

UI icons are **stroked** (`stroke-width: 2`, Lucide style); brand/social icons are
**filled**. Capitalization tiers: UPPERCASE menu groups, section/result eyebrows, tags,
breadcrumbs, article metadata, and footer · Title-Case primary nav.

## Keyboard navigation

The command panel is both universal search and the shortcut reference. Its empty state is
one bounded navigation list; typing replaces it with a separately labelled content-results
view with excerpts and page kinds. A single fixed backdrop layer supplies restrained blur
(10px desktop, 6px mobile); no scrolling child or animated element carries a filter.

The shortcut grammar uses sequential <kbd>G</kbd> chords: <kbd>/</kbd> opens search,
<kbd>?</kbd> opens the shortcut reference, navigation destinations use “go to” mnemonics,
and <kbd>G D</kbd> toggles dark mode. Theme switching also remains a searchable palette
command: press <kbd>/</kbd>, type `theme`, then press <kbd>Enter</kbd>. No global modifier
bindings are claimed, leaving browser and operating-system shortcuts untouched. Interactive
and editable controls suppress global shortcuts.

| chord | action          |
| ----- | --------------- |
| `G H` | Home            |
| `G A` | Archive         |
| `G T` | Tags            |
| `G P` | About / profile |
| `G M` | Maxims          |
| `G B` | Blogroll        |
| `G D` | Toggle theme    |

The sequence expires after 1.2 seconds. There are deliberately no shortcuts for every
section or control. Route chords pause inside editable controls so search text can never
navigate unexpectedly. Within the command palette, arrow keys move, <kbd>Enter</kbd> opens
or runs a command, and <kbd>Escape</kbd> closes.

## Control-state matrix

- **rest → hover**: bordered buttons → fill `--surface-2`, border `--border-strong`, text
  full-contrast (uniform across nav icons, copy, pagination, tags, back-to-top). Prose
  links: animated gradient underline (0→100%).
- **focus-visible**: every interactive control gets `--ring`; article text links get a
  wrapping `2px` outline. No control lacks a visible focus indicator.
- **active**: pagination, tags, and theme-toggle press to `--surface-2`.

## Dark-mode rule

Drop-shadows vanish on `#0a0a0a`, so **elevation comes from surface lightness + border,
never shadow alone**: theme-switcher active segment is _lighter_ than its pill
(`--toggle-active #2e2e2e`); skip-link uses `--bg-2`; `--shadow-key` (kbd) is a solid-color
line. Hover legibility in dark relies on the border step (`#2e2e2e → #454545`).

## Accessibility

Deeper-than-Geist text for contrast; `contrast_test.go` covers light + dark. Focus rings on
all controls; 44px mobile touch targets for nav/search/tags/pagination/connect rows. The
command palette traps focus, makes the page inert while open, exposes every supported
shortcut, and searches through Pagefind's low-level API only after the first query. Pagefind
indexes all writing and evergreen pages (including Maxims); weighted hidden tag metadata
makes taxonomy terms searchable without duplicating visible content. Queries are debounced
before loading the index. Theme defaults to **System** (follows OS), overridable.
`prefers-reduced-motion` cancels transitions/animations. Skip-link, `sr-only`, forced-colors
fallback all present.

## Responsive

The primary layout breakpoint is 640px, with purpose-specific steps at 360px (tight phone),
700/900px (content and frame), 960px (reading type + stacked hero), and 1023px (compact
header search). The label-only navigation popover and command shortcut panel use simple
single-column rows, and both are viewport-bound from 320px phones through desktop. Display
type scales via `clamp()`. Verified: **no horizontal overflow 320→1440px**; the reading
column fills width on small screens and caps at 720px centered on desktop.
