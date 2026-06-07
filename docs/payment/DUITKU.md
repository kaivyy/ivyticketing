# Duitku Integration

## Overview

Duitku is an Indonesian payment aggregator supporting QRIS, virtual account (VA),
and e-wallet channels. The integration uses form-encoded callbacks and MD5-based
signature verification.

In V1 the `VerifySignature` and `ParseCallback` methods are fully implemented and
tested. `CreateCharge` and `QueryStatus` are stubs pending sandbox credentials.

---

## Callback Signature

Duitku signs each callback with an MD5 hash of four fields concatenated without any
separator:

```
raw   = merchantCode + amount + merchantOrderId + apiKey
sig   = hex( MD5(raw) )
```

Where:
- `merchantCode` — your Duitku merchant code (from `DUITKU_MERCHANT_CODE`)
- `amount` — the payment amount as a plain integer string (e.g., `"100000"`)
- `merchantOrderId` — the merchant reference (e.g., `PAY-20260607-A3F9Z2`)
- `apiKey` — your Duitku API key (from `DUITKU_API_KEY`)

The computed hex string must match the `signature` field in the callback body.
Comparison is constant-time to prevent timing attacks.

### Example

```
merchantCode  = "DS12345"
amount        = "100000"
merchantOrderId = "PAY-20260607-A3F9Z2"
apiKey        = "abc123"

raw = "DS12345" + "100000" + "PAY-20260607-A3F9Z2" + "abc123"
    = "DS12345100000PAY-20260607-A3F9Z2abc123"

sig = hex(MD5("DS12345100000PAY-20260607-A3F9Z2abc123"))
```

---

## Callback Format

Duitku sends `POST` with `Content-Type: application/x-www-form-urlencoded`.

| Form field       | Type   | Description                                     |
|------------------|--------|-------------------------------------------------|
| `merchantCode`   | string | Your merchant code                              |
| `amount`         | string | Payment amount in IDR                           |
| `merchantOrderId`| string | Merchant reference (maps to `MerchantReference`)|
| `reference`      | string | Duitku transaction reference (`GatewayReference`)|
| `resultCode`     | string | Payment result (see status codes below)         |
| `signature`      | string | MD5 signature for verification                  |

---

## Status Codes

| `resultCode` | Mapped Status | Description                  |
|--------------|---------------|------------------------------|
| `00`         | `PAID`        | Payment successful           |
| `01`         | `PENDING`     | Payment pending / in process |
| other        | `FAILED`      | Payment failed or unknown    |

---

## Registering the Callback URL with Duitku

1. Log in to the [Duitku merchant dashboard](https://www.duitku.com).
2. Navigate to **Settings → Callback URL**.
3. Set the callback URL to your webhook receiver's Duitku endpoint:
   ```
   https://<your-domain>/webhooks/duitku
   ```
   This corresponds to `POST /webhooks/duitku` on the webhook receiver binary.
4. Ensure the domain is publicly reachable from Duitku's servers. For local
   development use a tunnelling tool (e.g., ngrok) and update the URL accordingly.
5. Verify the merchant code and API key in the dashboard match your env vars.

---

## Environment Variables

| Variable              | Required when enabled | Description                                    |
|-----------------------|-----------------------|------------------------------------------------|
| `DUITKU_ENABLED`      | —                     | `true` to register Duitku adapter at startup   |
| `DUITKU_MERCHANT_CODE`| yes                   | Duitku merchant code from the dashboard         |
| `DUITKU_API_KEY`      | yes                   | Duitku API key (used in MD5 signature)         |
| `DUITKU_ENV`          | —                     | `sandbox` (default) or `production`            |

If `DUITKU_ENABLED=true` and either `DUITKU_MERCHANT_CODE` or `DUITKU_API_KEY` is
empty, the application refuses to start with a config error.

### Example `.env` (sandbox)

```bash
DUITKU_ENABLED=true
DUITKU_MERCHANT_CODE=DS12345
DUITKU_API_KEY=your-sandbox-api-key
DUITKU_ENV=sandbox
```
