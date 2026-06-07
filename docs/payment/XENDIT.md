# Xendit Integration

## Overview

Xendit is an Indonesian and Southeast Asian payment infrastructure provider supporting
QRIS, virtual account, e-wallet, and card channels. The integration uses JSON callbacks
and a shared static token for verification.

In V1 the `VerifySignature` and `ParseCallback` methods are fully implemented and
tested. `CreateCharge` and `QueryStatus` are stubs pending sandbox credentials.

---

## Callback Verification

Xendit uses a static shared secret in the `x-callback-token` HTTP header. Unlike
per-request HMAC signatures, every callback from Xendit carries the same token value.

```
x-callback-token: <your-callback-token>
```

The header value is compared to `XENDIT_CALLBACK_TOKEN` using constant-time
comparison to prevent timing attacks. The token is set once in the Xendit dashboard
and must match the env var exactly (case-sensitive).

There is no per-request cryptographic signature. Security relies on:
1. The token being kept secret (never logged, never returned in API responses).
2. HTTPS being enforced on the webhook endpoint in production.
3. Optional: IP allowlisting at the load balancer / firewall level.

---

## Callback Format

Xendit sends `POST` with `Content-Type: application/json`.

Key fields used by the adapter:

| JSON field    | Type   | Description                                       |
|---------------|--------|---------------------------------------------------|
| `external_id` | string | Merchant reference (maps to `MerchantReference`)  |
| `id`          | string | Xendit invoice/payment ID (`GatewayReference`)    |
| `status`      | string | Payment status (see mapping below)                |
| `amount`      | number | Payment amount in IDR                             |

Additional fields may be present (payer details, payment method info, etc.) and are
ignored by the adapter but preserved in the raw `payment_webhooks.payload` column.

---

## Status Values

| Xendit `status`                              | Mapped status |
|----------------------------------------------|---------------|
| `PAID`, `SETTLED`, `SUCCEEDED`, `COMPLETED`  | `PAID`        |
| `PENDING`, `ACTIVE`                          | `PENDING`     |
| `EXPIRED`                                    | `EXPIRED`     |
| anything else                                | `FAILED`      |

Comparison is case-insensitive (`strings.ToUpper` applied before matching).

---

## Setting the Callback Token in Xendit Dashboard

1. Log in to the [Xendit dashboard](https://dashboard.xendit.co).
2. Navigate to **Settings → Webhooks**.
3. Under **Callback verification token**, set or copy your token.
4. Set this token as `XENDIT_CALLBACK_TOKEN` in your environment.
5. Set the webhook URL for invoices (and any payment methods you enable) to:
   ```
   https://<your-domain>/webhooks/xendit
   ```
   This corresponds to `POST /webhooks/xendit` on the webhook receiver binary.
6. For local development, use a tunnelling tool (e.g., ngrok) and update the URL.

---

## Environment Variables

| Variable               | Required when enabled | Description                                          |
|------------------------|-----------------------|------------------------------------------------------|
| `XENDIT_ENABLED`       | —                     | `true` to register Xendit adapter at startup         |
| `XENDIT_SECRET_KEY`    | yes                   | Xendit secret key (for `CreateCharge` in Phase 23)   |
| `XENDIT_CALLBACK_TOKEN`| yes                   | Static token set in Xendit dashboard                 |
| `XENDIT_ENV`           | —                     | `sandbox` (default) or `production`                  |

If `XENDIT_ENABLED=true` and either `XENDIT_SECRET_KEY` or `XENDIT_CALLBACK_TOKEN` is
empty, the application refuses to start with a config error.

### Example `.env` (sandbox)

```bash
XENDIT_ENABLED=true
XENDIT_SECRET_KEY=xnd_development_...
XENDIT_CALLBACK_TOKEN=your-callback-token
XENDIT_ENV=sandbox
```
