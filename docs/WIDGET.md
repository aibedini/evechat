# EveChat widget JavaScript API

```html
<script>
  window.EveChat = { projectKey: "pk_...", apiUrl: "https://chat.example.com" };
</script>
<script async src="https://chat.example.com/widget.js"></script>
```

## Configuration

| Option | Default | Description |
|---|---|---|
| `projectKey` | — | **required** project key from Settings → Widget |
| `apiUrl` | current origin | EveChat server origin |
| `locale` | `fa` | `fa` or `en` (Persian is RTL by default) |
| `position` | `bottom-left` | `bottom-left` or `bottom-right` |
| `autoOpen` | `false` | open the panel on load |
| `theme` | `auto` | `auto`, `light`, `dark` (auto follows `prefers-color-scheme`) |
| `font` | `vazirmatn` | `vazirmatn`, `inherit` (host page font), `none` (system stack) |
| `visitor.name/email/mobile` | — | identity merged into the visitor row |
| `visitor.metadata` | `{}` | arbitrary key/values used by routing rules |
| `visitor.subscription` | `{}` | `{plan, expiry}` used by `has_subscription` |
| `colors` | — | `{primary, launcher, text, background}` overrides |

## Runtime commands

```js
window.ec = window.ec || function () { (ec.q = ec.q || []).push(arguments); };
ec("open");
ec("close");
ec("toggle");
ec("identify", { name: "علی", mobile: "09120000000", metadata: { eve_user_id: "u1" } });
ec("setVariable", "utm_source", "google");
ec("send", "سلام");
ec("on", "message", function (msg) { /* agent message */ });
ec("on", "csat", function (data) { /* {score} */ });
```

Commands issued before the bundle loads are queued and replayed.

## Events

| Event | Payload |
|---|---|
| `ready` | — |
| `open` / `close` | — |
| `message` | agent/bot message object |
| `connection` | `connecting / open / reconnecting / degraded / offline` |
| `csat` | `{score}` |

## Behaviour guarantees

- Transport: WebSocket first; after two failures it switches to SSE + REST;
  after two more to long-polling. Every 60s while degraded it probes WebSocket
  again. Backoff is 1s, 2s, 4s, 8s, 15s, 30s (cap) with ±25% jitter.
- Every outbound message is written to IndexedDB first and removed only after
  the server acknowledges it (`message.sent`). Duplicate `client_message_id`s
  are collapsed server-side, so retries are safe.
- Reconnect sends `sync.request` with the last server timestamp and reconciles
  by message id.
- The Vazirmatn webfont is injected on the first panel open only (never on page
  load) and lives outside the 16 KB bundle budget.
- `Esc` closes the panel; the message list is `role="log"` + `aria-live="polite"`;
  the launcher announces its unread count.

## Embedding notes

- The widget renders in a shadow root, so host-page CSS cannot leak in.
- The project domain allow-list (Settings → Widget → domains) must include the
  host page; an empty list means "dev mode — allow everything".
- The `visitor` object is hashed into the project; if the page knows the user,
  pass `mobile` so EVE enrichment can fill the reseller and subscription fields.
