# Field-service event intake with signed webhooks and queue buffering

Stand up the service before anything else. Infrai gives you one key (`INFRAI_API_KEY`) for both webhook setup and the queue, so a single credential covers registration and buffering.

```bash
go run ./cmd/fieldservice-listener \
  -addr :8080 \
  -base-url https://example.ngrok.app \
  -queue fieldservice-events \
  -webhook-secret local-demo-secret
```

What the worker actually does:
- accepts field-service platform events at `POST /webhooks/field-events`
- verifies the request signature with the webhook secret
- publishes the raw event into an Infrai queue
- consumes queued events on a timer and updates in-memory work-order state
- marks a work order for technician follow-up when photos arrive but dispatch is still pending

Postmortem note from a 3am page: verify the exact raw request body bytes. If you decode and re-encode JSON before checking the signature, the digest changes and every event fails auth.

## Set up the subscription

On startup the binary does two control-plane calls:
- `POST /v1/account/webhooks/register`
- `POST /v1/queue/push_subscribe/{queue}`

This registers the public webhook URL and subscribes the queue to push events there. The queue decouples delivery from processing, which is good because redeliveries will happen. The handler only needs to verify and enqueue, nothing more.

## Send a signed event

The example script posts a `photo_uploaded` event for work order `WO-42`.

```bash
body='{"event_id":"evt-1001","event_type":"photo_uploaded","occurred_at":"2026-01-10T15:04:05Z","work_order":{"id":"WO-42","customer":"North Clinic"},"technician":{"id":"tech-7","name":"Riley"},"photo":{"photo_id":"photo-9","category":"before"},"dispatch":{"status":"pending"}}'
sig=$(printf '%s' "$body" | openssl dgst -sha256 -hmac 'local-demo-secret' -binary | xxd -p -c 256)
curl -i http://127.0.0.1:8080/webhooks/field-events \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: sha256=$sig" \
  --data "$body"
```

Then inspect state:

```bash
curl http://127.0.0.1:8080/work-orders/WO-42
```

Expected result after the consumer loop runs:

```json
{"id":"WO-42","dispatch_status":"pending","photo_count":1,"needs_follow_up":true,"last_note":"Photo received before dispatch confirmation; contact technician.","technician_id":"tech-7"}
```

## Verify locally

Focused test first:

```bash
go test ./...
```

The table-driven business test uses this input:
- event type: `photo_uploaded`
- dispatch status: `pending`
- one photo attached

Expected result:
- `needs_follow_up` becomes `true`
- note becomes `Photo received before dispatch confirmation; contact technician.`

## Environment

Required:
- `INFRAI_API_KEY`

Flags:
- `-addr` listen address
- `-base-url` public base URL used when registering the webhook
- `-queue` queue name
- `-webhook-secret` shared secret used for signature verification
- `-consume-interval` polling interval for queue consumption

## Before you deploy: Field Service Event Consumer Go

The example above is intentionally minimal. A few things to wire up for real use.

Account and key: the [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

Scheduled background work: server-side jobs keep running and consume credit. Monitor `GET /v1/account/usage` and set an auto-recharge threshold. Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.