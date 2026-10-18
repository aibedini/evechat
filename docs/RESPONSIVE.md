# Responsive design

Five breakpoints, mobile first. They are defined once in
`web/dashboard/src/styles/breakpoints.css` and mirrored in the widget
(`web/widget/src/ui/styles.ts`) and the setup wizard (`web/setup/src/style.css`).

| Name | Width | Meaning |
|---|---|---|
| `xs` | 360px | small phones |
| `sm` | 480px | phones: the widget goes full screen |
| `md` | 768px | tablets: sidebar becomes a drawer, tables become cards, kanban becomes tabs, inbox collapses to one column |
| `lg` | 1024px | small laptops: the visitor panel appears |
| `xl` | 1440px | desktop: the widget panel grows to 400x600 |

## Widget

- The panel uses `100dvh` with a `100vh` fallback, and `env(safe-area-inset-*)`
  on the launcher and composer so notches and home indicators do not overlap it.
- **Mobile keyboard**: mobile browsers do not resize the layout viewport when the
  keyboard opens, so `window.innerHeight` stays put and the composer ends up
  behind the keyboard. The widget listens to `visualViewport` (`resize` and
  `scroll`) and exposes `--ec-vh` (layout height), `--ec-vv` (visible height) and
  `--ec-kb` (keyboard height) on the host element. Under 480px the panel pins
  itself to `--ec-vv` and adds `--ec-kb` as bottom padding to the message list,
  so the composer always sits directly above the keyboard.
- Opening the full-screen panel locks page scroll (`body{overflow:hidden}`) and
  restores the previous value on close.
- Five breakpoints: 360, 480, 768, 1024, 1440.

## Dashboard

- **App shell**: under `md` the sidebar is an off-canvas drawer with a backdrop
  and a burger in the mobile bar; the drawer closes on navigation.
- **Inbox**: under `md` the list and the chat are two screens — `data-active`
  hides the list when a conversation is open, and the chat header gets a back
  button. The visitor panel disappears below `lg`.
- **Tables** (visitors, team): add `.table-cards` and `data-label` on the cells;
  under `md` rows render as cards and labels come from the attribute.
- **Tickets**: the five-column kanban becomes a tab strip under `md`, one column
  at a time.

## Setup wizard

`.shell` uses safe-area padding, 16px inputs (prevents iOS zoom-on-focus), full
width actions under `md` and a tighter card at 360px.

## Tests

`web/e2e/tests/responsive.spec.ts` runs 8 devices x 2 orientations x 2 themes =
32 combinations and asserts eight things per combination: launcher visible and
inside the viewport, panel opens, panel fits the viewport, composer visible above
the fold, no horizontal page scroll, a sent message renders, the theme reaches the
panel background, and closing hides the panel. A final test checks the dashboard
drawer.
