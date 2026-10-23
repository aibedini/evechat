# FCM push for the Android agent app

EveChat implements the server side of the contract in
[`docs/evechat-push.md`](https://github.com/aibedini/Messages/blob/master/docs/evechat-push.md).
Push is a hint: the app re-reads the conversation over REST and builds the
notification itself. Messages are **data-only**; EveChat never sends an FCM
`notification` block.

## Server configuration

Create a Firebase service account with the *Firebase Cloud Messaging API*
enabled and download its JSON key. Keep it outside the repository and readable
by the `evechat` service user, for example:

```bash
sudo install -o root -g evechat -m 0640 firebase-service-account.json \
  /opt/evechat/firebase-service-account.json
```

Then point EveChat at it:

```bash
FIREBASE_SERVICE_ACCOUNT_FILE=/opt/evechat/firebase-service-account.json
# or the full JSON in one line:
# FIREBASE_SERVICE_ACCOUNT_JSON={"type":"service_account",...}
# optional override if the JSON has no project_id:
# FIREBASE_PROJECT_ID=my-firebase-project
```

`FIREBASE_SERVICE_ACCOUNT_JSON` also accepts a path, and
`GOOGLE_APPLICATION_CREDENTIALS` works as the standard fallback. Without
credentials the registration endpoints remain available; delivery is skipped
and a warning is logged at startup.

The server needs outbound HTTPS to `oauth2.googleapis.com` and
`fcm.googleapis.com`. No inbound port is required.

## Endpoints

Both require the agent's normal bearer token.

```http
POST /api/v1/push/devices
Content-Type: application/json

{
  "device_id": "4f0a…-uuid",
  "platform": "android",
  "provider": "fcm",
  "token": "<FCM registration token>",
  "app_version": "3.9.0",
  "app_build": 155,
  "os_version": "Android 14 (34)",
  "device_model": "samsung SM-A546E",
  "locale": "fa-IR",
  "timezone": "Asia/Tehran"
}
```

The first registration returns `201`; a re-registration (same `device_id`)
returns `200` and moves the device to the calling agent. The body is ignored, so
an empty JSON object is returned.

```http
DELETE /api/v1/push/devices/{device_id}
```

Returns `204` when removed and `404` when the device does not exist. A normal
agent can delete only their own device; `admin`/`superadmin` can delete a device
in their project.

## Recipients and payload

| event | recipients | type |
|---|---|---|
| new unassigned conversation | every active agent with a device | `conversation.new` |
| visitor message, unassigned | every active agent with a device | `message.received` |
| visitor message, assigned | the assigned agent | `message.received` |
| assignment | new assignee (`agent.assigned`) + other agents (`conversation.updated`) | |
| resolve / reopen | the assigned agent | `conversation.resolved` / `conversation.updated` |

Agent messages, internal notes and system messages are never pushed. Payload
keys are `source=evechat`, `v=1`, `type`, `conversation_id`, `message_id`,
`agent_id`, `sent_at` (unix ms), and optional `title`/`body`. Android priority
is `HIGH`/`3600s` for messages and new conversations, and `NORMAL`/`600s` with
a conversation collapse key for lifecycle events.

FCM tokens reported as `UNREGISTERED` or `INVALID_ARGUMENT` are deleted
automatically. Registrations not refreshed for 60 days are pruned hourly.

## Quick check

```bash
TOKEN=...   # agent access token
curl -i -X POST https://chat.example.com/api/v1/push/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"manual-test","token":"<fcm-token>"}'
curl -i -X DELETE https://chat.example.com/api/v1/push/devices/manual-test \
  -H "Authorization: Bearer $TOKEN"
```

Then send a widget message to the agent and inspect the server log for
`FCM push enabled`; delivery failures are logged without the token.
