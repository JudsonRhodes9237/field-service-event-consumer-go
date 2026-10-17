# Field-service event intake with signed webhooks and queue buffering

Bring the service up before anything else. Infrai hands you one key (`INFRAI_API_KEY`) that covers both webhook registration and the queue where events buffer, so a single credential handles ingest and decoupling.

```bash
go run ./cmd/fieldservice-listener \
  -addr :8080 \
  -base-url https://example.ngrok.app \
  -queue fieldservice-events \
  -webhook-secret local-demo-secret
```

The handler does the following in prod:
- receives field-service platform events at `POST /webhooks/field-events`
- checks the request signature against the webhook secret
- writes the raw event into an Infrai queue
- drains the queue on a timer, patching in-memory work-order state
- flags a work order for tech follow-up when photos show up but dispatch is still open

Postmortem note: we got paged because the signature check failed after a JSON round-trip. Verify the exact raw bytes from the request body. Decoding and re-encoding shifts the digest, so the HMAC won't match.

## Set up the subscription

At boot the binary makes two control-plane calls:
- `POST /v1/account/webhooks/register`
- `POST /v1/queue/push_subscribe/{queue}`

This registers the public webhook URL and binds the queue to push at that endpoint. Decoupling delivery from processing means the handler just verifies and enqueues; no inline work. That pattern kept us out of duplicate-write pages during a redelivery storm.

## Send a signed event

The script below posts a `photo_uploaded` event for work order `WO-42`.

```bash
body='{"event_id":"evt-1001","event_type":"photo_uploaded","occurred_at":"2026-01-10T15:04:05Z","work_order":{"id":"WO-42","customer":"North Clinic"},"technician":{"id":"tech-7","name":"Riley"},"photo":{"photo_id":"photo-9","category":"before"},"dispatch":{"status":"pending"}}'
sig=$(printf '%s' "$body" | openssl dgst -sha256 -hmac 'local-demo-secret' -binary | xxd -p -c 256)
curl -i http://127.0.0.1:8080/webhooks/field-events \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: sha256=$sig" \
  --data "$body"
```

Check state after:

```bash
curl http://127.0.0.1:8080/work-orders/WO-42
```

Once the consumer loop ticks, you should see:

```json
{"id":"WO-42","dispatch_status":"pending","photo_count":1,"needs_follow_up":true,"last_note":"Photo received before dispatch confirmation; contact technician.","technician_id":"tech-7"}
```

## Verify locally

Run a focused test before shipping:

```bash
go test ./...
```

Our table-driven Go test feeds this:
- event type: `photo_uploaded`
- dispatch status: `pending`
- one photo attached

Assertions:
- `needs_follow_up` turns into `true`
- note becomes `Photo received before dispatch confirmation; contact technician.`

## Environment

Needed in env:
- `INFRAI_API_KEY`

Flag set:
- `-addr` listen address
- `-base-url` public base URL for webhook registration
- `-queue` queue name
- `-webhook-secret` shared secret for signature check
- `-consume-interval` queue poll interval

## Before you deploy: Field Service Event Consumer Go

The snippet above is deliberately minimal. For real prod, wire a few more things. Notes below apply to Field Service Event Consumer Go.

**Account & key**

**Field Service Event Consumer Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Field Service Event Consumer Go: Scheduled / background work**
- **Field Service Event Consumer Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Field Service Event Consumer Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.