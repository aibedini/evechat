# Accessibility

EveChat targets **WCAG 2.1 AA** for the dashboard, widget and setup wizard. This
document lists what is implemented and how it is verified.

## Internationalisation & direction

- `<html lang="fa" dir="rtl">` on the dashboard, setup wizard and docs page.
- The widget sets `lang="fa" dir="rtl"` on its shadow host.
- Numbers use Persian digits where the UI is Persian (`faNum`); dates use the
  Jalali calendar with `Intl` and a fallback algorithm.
- English strings exist for the widget (`locale: "en"`); the interface switches
  to LTR but keeps the same component sizes.

## Keyboard

- Every interactive element is reachable with `Tab`; the DOM order matches the
  visual RTL order.
- `:focus-visible` draws a 3px high-contrast ring (`#b45309` on light,
  `#fbbf24` on dark) with a 2px offset.
- `Esc` closes the widget panel, modals and the mobile navigation.
- The widget composer submits on `Enter` and inserts a newline on `Shift+Enter`.

## Screen readers

- Widget message list: `role="log"`, `aria-live="polite"`,
  `aria-relevant="additions text"`; the launcher exposes `aria-expanded` and an
  `aria-label` that includes the unread count.
- Modal: `role="dialog"`, `aria-modal="true"`, labelled by its title; focus
  returns to the trigger on close.
- Icon-only buttons have Persian `aria-label`s; decorative icons use `aria-hidden`.
- Form errors are announced via `aria-live` regions (login, setup, settings).
- The setup wizard uses `role="progressbar"` semantics through the step list.

## Visual

- Contrast budget enforced by `scripts/contrast-check.mjs` in CI:
  - body text ≥ 4.5:1, large text ≥ 3:1, focus ring ≥ 3:1.
  - danger buttons use `#dc2626` (white text = 4.8:1).
- Text scale: layouts survive 200% zoom and 320px width; the inbox switches to a
  single column below `sm` and hides the visitor sidebar below `xl`.
- No information is conveyed by colour alone (status chips include a glyph and
  text).
- `prefers-reduced-motion: reduce` disables panel slide-in, typing dots and
  launcher animations. `prefers-color-scheme` selects the initial theme; the
  toggle in the sidebar overrides it and is persisted.

## Media

- Images have `alt` text (screenshots and the empty states).
- The widget is usable without images; the launcher is a vector icon inline in
  the bundle.

## Testing performed

1. Keyboard-only walkthrough of login → inbox → reply → resolve.
2. VoiceOver/NVDA spot check of the widget log and the settings modals.
3. Axe-core scan of the five main pages during the screenshot run (no
   serious/critical findings).
4. `scripts/contrast-check.mjs` in CI on every push.
5. 320px, 768px, 1024px and 1440px viewport captures in `docs/screenshots`.
