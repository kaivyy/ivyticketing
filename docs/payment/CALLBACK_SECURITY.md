# Callback Security

## Why the Webhook Endpoint Has No Auth Token

The webhook receiver (`POST /webhooks/duitku`, `POST /webhooks/xendit`) does not
require a Bearer token or API key from the caller.

This is intentional. Webhook callbacks are initiated by the payment gateway's servers,
not by users. Gateways do not carry your application's auth tokens — they call a URL
you registered in their dashboard. Requiring a Bearer token would mean embedding that
token in the callback URL or having the gateway somehow obtain it, neither of which
is a supported pattern.

The webhook endpoint is intended to be network-accessible to the gateway's IP ranges
only (enforced at the load balancer or firewall level in production). The application
layer verifies authenticity via gateway-specific signature mechanisms.

---

## Signature Verification as the Auth Mechanism

Each gateway uses a different verification mechanism:

### Duitku — MD5 Signature

```
signature = hex( MD5(merchantCode + amount + merchantOrderId + apiKey) )
```

The `apiKey` is a shared secret known only to Duitku and the platform. An attacker
without the API key cannot forge a valid signature. Comparison is constant-time.

### Xendit — x-callback-token Header

```
x-callback-token: <XENDIT_CALLBACK_TOKEN>
```

A static shared secret set in the Xendit dashboard and in `XENDIT_CALLBACK_TOKEN`.
Comparison is constant-time.

Both mechanisms are checked in `VerifySignature` before any payment state is modified.

---

## Store-First Design

The raw callback payload is persisted in `payment_webhooks` **before** signature
verification runs. This is intentional:

- Even invalid-signature callbacks are stored for audit and incident investigation.
- A flood of forged callbacks is detectable and traceable without any payment state
  being affected.
- The store operation is lightweight (single INSERT with `status='RECEIVED'`).

An invalid signature results in:
1. The webhook row updated to `status='REJECTED'`, `error_detail='INVALID_SIGNATURE'`
2. `HTTP 401 Unauthorized` returned to the caller
3. No payment or order state changed

---

## What Happens on Invalid Signature

```
Gateway (or attacker) → POST /webhooks/duitku
  body: { ... }

Webhook receiver:
  1. g.VerifySignature(headers, rawBody) → false
  2. INSERT payment_webhooks (status='RECEIVED')
  3. UPDATE payment_webhooks SET status='REJECTED', error_detail='INVALID_SIGNATURE'
  4. INSERT audit_log (PAYMENT_CALLBACK_REJECTED, gateway=duitku, reason=INVALID_SIGNATURE)
  5. return HTTP 401 Unauthorized

Payment state: unchanged
Order state:   unchanged
```

The `401` response tells a legitimate gateway that something is wrong (misconfigured
secret, wrong endpoint). It tells an attacker that the attempt was detected.

For Xendit, the gateway will retry on `401`. The store-first design ensures all retry
attempts are recorded, making it easy to detect and diagnose misconfiguration.

---

## Never Logging Secret Values

The following values are never written to logs, error messages, or audit entries:

- `DUITKU_API_KEY` — used only inside `VerifySignature` for MD5 computation
- `XENDIT_CALLBACK_TOKEN` — used only inside `VerifySignature` for comparison
- `XENDIT_SECRET_KEY` — used only for `CreateCharge` HTTP requests (Phase 23)

Log entries for webhook events include only:
- Gateway name (e.g., `"duitku"`)
- Merchant reference (e.g., `"PAY-20260607-A3F9Z2"`)
- Payment status (e.g., `"PAID"`)
- Rejection reason (e.g., `"INVALID_SIGNATURE"`) — never the secret itself

This applies to structured logs (`slog`), audit log entries, and
`payment_webhooks.error_detail`. The `payment_webhooks.signature` column stores the
**received** signature value (not the secret key) for forensic purposes — this is
safe because it is a one-way hash output, not the key material.
