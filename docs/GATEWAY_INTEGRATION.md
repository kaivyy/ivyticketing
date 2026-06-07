# Gateway Integration

## The Gateway Interface

All payment gateways implement a single interface defined in
`internal/modules/payments/gateway/gateway.go`:

```go
type Gateway interface {
    // Name returns the unique lowercase identifier for this gateway (e.g., "duitku", "xendit").
    Name() string

    // CreateCharge creates a payment charge at the gateway.
    // Returns pay_url / qr_string / va_number for the participant to complete payment.
    CreateCharge(ctx context.Context, in CreateChargeInput) (CreateChargeResult, error)

    // VerifySignature validates the inbound callback's authenticity.
    // Returns false if the signature is invalid or missing.
    VerifySignature(headers http.Header, rawBody []byte) bool

    // ParseCallback extracts a CallbackResult from the raw callback body.
    ParseCallback(rawBody []byte) (CallbackResult, error)

    // QueryStatus fetches the current payment status directly from the gateway.
    // Used by the reconcile path when no callback was received.
    QueryStatus(ctx context.Context, gatewayReference string) (CallbackResult, error)
}
```

Key types:

```go
type CreateChargeInput struct {
    MerchantReference string     // PAY-YYYYMMDD-XXXXXX
    Amount            int64      // minor units (IDR cents)
    Method            string     // qris | va | ewallet
    Channel           string     // gateway-specific channel
    CustomerEmail     string
    CustomerName      string
    ExpiresAt         time.Time
}

type CreateChargeResult struct {
    GatewayReference string
    PayURL           string
    QRString         string
    VANumber         string
    Instructions     map[string]any
    ExpiresAt        time.Time
}

type CallbackResult struct {
    MerchantReference string
    GatewayReference  string
    Status            PaymentStatus  // PENDING | PAID | EXPIRED | FAILED
    Amount            int64
    PaidAt            *time.Time
    EventType         string
}
```

---

## Gateway Registry

`BuildPaymentRegistry` in `internal/app/payments.go` builds the registry at startup:

```go
func BuildPaymentRegistry(cfg Config) *paygw.Registry {
    r := paygw.NewRegistry()
    if cfg.DuitkuEnabled {
        r.Register(duitku.New(duitku.Config{ ... }))
    }
    if cfg.XenditEnabled {
        r.Register(xendit.New(xendit.Config{ ... }))
    }
    return r
}
```

The config loader (`LoadConfig`) enforces fail-fast: if `DUITKU_ENABLED=true` but
`DUITKU_MERCHANT_CODE` or `DUITKU_API_KEY` are empty, the process refuses to start.
Same for Xendit.

The registry is passed to both the API server (payment creation) and the webhook
receiver binary (callback processing) so they share the same gateway adapters.

---

## How to Add a New Gateway (Midtrans Example)

1. **Create the adapter package**

   ```
   services/api/internal/modules/payments/gateway/midtrans/
     midtrans.go
     midtrans_test.go
   ```

   ```go
   // midtrans.go
   package midtrans

   import (
       gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
   )

   type Config struct {
       ServerKey   string
       ClientKey   string
       Env         string
       HTTPClient  *http.Client
   }

   type Adapter struct { cfg Config; client *http.Client }

   func New(cfg Config) *Adapter { ... }
   func (a *Adapter) Name() string { return "midtrans" }

   func (a *Adapter) VerifySignature(headers http.Header, rawBody []byte) bool {
       // Midtrans uses SHA512: orderId + statusCode + grossAmount + serverKey
       ...
   }

   func (a *Adapter) ParseCallback(rawBody []byte) (gw.CallbackResult, error) {
       // Parse JSON notification body
       ...
   }

   func (a *Adapter) CreateCharge(ctx context.Context, in gw.CreateChargeInput) (gw.CreateChargeResult, error) {
       // POST to api.midtrans.com/v2/charge
       ...
   }

   func (a *Adapter) QueryStatus(ctx context.Context, ref string) (gw.CallbackResult, error) {
       // GET api.midtrans.com/v2/:orderId/status
       ...
   }
   ```

2. **Add config fields** to `internal/app/config.go`:

   ```go
   MidtransEnabled   bool
   MidtransServerKey string
   MidtransClientKey string
   MidtransEnv       string
   ```

   Add corresponding `os.Getenv` calls and a fail-fast guard in `LoadConfig`.

3. **Register in `BuildPaymentRegistry`**:

   ```go
   if cfg.MidtransEnabled {
       r.Register(midtrans.New(midtrans.Config{
           ServerKey: cfg.MidtransServerKey,
           ClientKey: cfg.MidtransClientKey,
           Env:       cfg.MidtransEnv,
       }))
   }
   ```

4. **Add webhook route** in `internal/modules/payments/webhook/http/server.go`:

   ```go
   r.Post("/webhooks/midtrans", s.handle("midtrans"))
   ```

5. **Add env vars** to `.env.example`.

6. **Write unit tests** for `VerifySignature` and `ParseCallback` (see existing
   `duitku_test.go` and `xendit_test.go` as templates).

No other code changes are required. The processor, reconciler, and deduplication
logic are gateway-agnostic.

---

## Duitku Integration Details

Duitku sends form-encoded callbacks (`application/x-www-form-urlencoded`).

### Signature Verification

MD5 of the concatenation (no separator):

```
signature = MD5(merchantCode + amount + merchantOrderId + apiKey)
```

Where:
- `merchantCode` — from `DUITKU_MERCHANT_CODE`
- `amount` — payment amount as a string (e.g., `"100000"`)
- `merchantOrderId` — the merchant reference (e.g., `PAY-20260607-A3F9Z2`)
- `apiKey` — from `DUITKU_API_KEY`

The computed hex string is compared to the `signature` form field using constant-time
comparison to prevent timing attacks.

### Status Codes

| Duitku `resultCode` | Mapped status |
|---------------------|---------------|
| `00`                | `PAID`        |
| `01`                | `PENDING`     |
| anything else       | `FAILED`      |

### Callback Fields

| Form field       | Mapped to                     |
|------------------|-------------------------------|
| `merchantOrderId`| `CallbackResult.MerchantReference` |
| `reference`      | `CallbackResult.GatewayReference`  |
| `resultCode`     | status (see table above)      |
| `amount`         | `CallbackResult.Amount`       |
| `signature`      | used for verification only    |

### Environment Variables

| Variable              | Required           | Description                              |
|-----------------------|--------------------|------------------------------------------|
| `DUITKU_ENABLED`      | no (default false) | Set to `true` to activate               |
| `DUITKU_MERCHANT_CODE`| if enabled         | Duitku merchant code from dashboard      |
| `DUITKU_API_KEY`      | if enabled         | Duitku API key (used for MD5 signature)  |
| `DUITKU_ENV`          | no (default sandbox) | `sandbox` or `production`             |

---

## Xendit Integration Details

Xendit sends JSON callbacks.

### Signature Verification

Xendit uses a shared static token in the `x-callback-token` header. There is no
per-request cryptographic signature.

```
x-callback-token: <XENDIT_CALLBACK_TOKEN>
```

The header value is compared to `XENDIT_CALLBACK_TOKEN` using constant-time comparison.

### Status Mapping

| Xendit `status`        | Mapped status |
|------------------------|---------------|
| `PAID`, `SETTLED`, `SUCCEEDED`, `COMPLETED` | `PAID`   |
| `PENDING`, `ACTIVE`    | `PENDING`     |
| `EXPIRED`              | `EXPIRED`     |
| anything else          | `FAILED`      |

### Callback Fields

| JSON field    | Mapped to                             |
|---------------|---------------------------------------|
| `external_id` | `CallbackResult.MerchantReference`    |
| `id`          | `CallbackResult.GatewayReference`     |
| `status`      | status (see table above)              |
| `amount`      | `CallbackResult.Amount`               |

### Environment Variables

| Variable               | Required            | Description                                     |
|------------------------|---------------------|-------------------------------------------------|
| `XENDIT_ENABLED`       | no (default false)  | Set to `true` to activate                       |
| `XENDIT_SECRET_KEY`    | if enabled          | Xendit secret key (for CreateCharge in Phase 23)|
| `XENDIT_CALLBACK_TOKEN`| if enabled          | Static token set in Xendit dashboard            |
| `XENDIT_ENV`           | no (default sandbox)| `sandbox` or `production`                       |

---

## V1 Stub Status

`CreateCharge` and `QueryStatus` are stubs in V1 — they return an error immediately.
This means:

- Payment creation via the API currently fails at the gateway call step for both Duitku
  and Xendit (since no real gateway call is made).
- `VerifySignature`, `ParseCallback`, and the full idempotent processor pipeline are
  fully implemented and tested.
- Adapters can be completed when sandbox credentials are available by implementing
  `CreateCharge` and `QueryStatus` in the respective adapter packages — no changes
  to the processor, reconciler, or webhook receiver are needed.
