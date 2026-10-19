# End-to-end behaviour suite

`web/e2e` holds the Playwright suite that covers the bug-fix sprint
(BUG-0 … BUG-4). It drives a real browser against a real instance; nothing is
mocked.

## What it covers

| # | Scenario | Defect |
|---|---|---|
| 1 | History survives a page reload | BUG-1 resume |
| 2 | Read messages are not resurrected as unread | BUG-1 resume |
| 3 | A resolved thread does not swallow the next message | BUG-1 lifecycle |
| 4 | A message on a pending conversation reaches the agent | BUG-3/4 room membership |
| 5 | Typing reaches the panel and expires server-side | BUG-3 typing + TTL |
| 6 | Claiming activates and owns the conversation | BUG-2 |
| 7 | The widget works with localStorage/IndexedDB blocked | BUG-0 |

`responsive.spec.ts` runs 8 devices x 2 orientations x 2 themes = 32
combinations with 8 assertions each (panel fits the viewport, composer above the
fold, no horizontal scroll, sent message renders, theme reaches the panel, ...)
plus a dashboard drawer test. See `docs/RESPONSIVE.md`.

## Running locally

```bash
# terminal 1: a throwaway instance
evechat setup --non-interactive --admin-password 'Passw0rd123!' --admin-email admin@evechat.local
EVECHAT_SECURITY__POW_DIFFICULTY=0 \
EVECHAT_RATELIMIT__WIDGET_SESSION_PER_MIN=600 evechat serve

# terminal 2
cd web/e2e
npm ci
npx playwright install chromium
EVECHAT_BASE=http://127.0.0.1:3000 npx playwright test
```

Disable PoW for the suite: the widget solves the challenge synchronously, and the
adaptive difficulty makes a 20-second boot timeout flaky on slower machines.
`responsive.spec.ts` closes its per-test browser context so 32 viewports do not
leave SSE/WebSocket sockets open.

The widget test page the suite drives is `GET /demo/test` (loopback only).

## Setup wizard

`wizard.spec.ts` needs an **unconfigured** instance, so it is opt-in and the
regular behaviour job skips it. `make fresh` (or `scripts/fresh.ps1` on
Windows) creates a throwaway instance: a temporary data directory, a free port,
temporary keys, and a setup token. It prints the URL and token and deletes
everything on Ctrl+C; it never touches `./data` or `./evechat.yaml`.

```powershell
make fresh          # or: powershell -File scripts/fresh.ps1
$env:EVECHAT_BASE = "http://127.0.0.1:<port>"
$env:EVECHAT_SETUP_TOKEN = "<printed token>"
$env:EVECHAT_WIZARD = "1"
cd web/e2e; npx playwright test wizard.spec.ts
```

The spec covers the loopback path (no token page), the token gate for a
simulated remote peer (`X-Forwarded-For` from the trusted loopback proxy), a
wrong token (Persian error), the correct token, 375px and 1440px layouts with
44px touch targets, and the 404 after setup completes. Screenshots land in
`docs/screenshots/wizard/`.

## CI

`.github/workflows/e2e.yml` job `behavior` builds the binary, starts an
instance on port 3212, and runs the behaviour and responsive specs. The
`wizard` job starts a second, unconfigured instance on port 3214 and runs
`wizard.spec.ts` with `EVECHAT_WIZARD=1`. A red run fails the workflow, so the
pull request cannot merge with a broken behaviour.