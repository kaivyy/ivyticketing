package enterprise

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

const (
	// maxDeliveryAttempts is the attempt ceiling before a delivery is parked as
	// DEAD (see MarkWebhookRetry's CASE). 6 attempts spans ~roughly an hour of
	// exponential backoff, enough to ride out a brief receiver outage.
	maxDeliveryAttempts = 6

	// deliveryTimeout caps how long we wait on a single receiver so one slow
	// endpoint can't stall the dispatch loop.
	deliveryTimeout = 10 * time.Second

	// signatureHeader carries the hex HMAC-SHA256 of the raw body, and
	// timestampHeader the unix seconds the request was signed at. Receivers
	// recompute HMAC(secret, body) and compare in constant time.
	signatureHeader = "X-IvyTicketing-Signature"
	timestampHeader = "X-IvyTicketing-Timestamp"
	eventTypeHeader = "X-IvyTicketing-Event"
	deliveryHeader  = "X-IvyTicketing-Delivery"
)

// Dispatcher drains the PENDING webhook-delivery ledger and POSTs each payload
// to its endpoint, signing the body with the endpoint secret. On failure it
// schedules an exponential-backoff retry; after maxDeliveryAttempts the row is
// parked DEAD by MarkWebhookRetry. Delivery is idempotent from the receiver's
// side: the same event_key maps to a single delivery row (UNIQUE constraint),
// and the X-IvyTicketing-Delivery header lets receivers dedupe on their end.
type Dispatcher struct {
	repo   Repository
	client *http.Client
	log    *slog.Logger
	batch  int32
}

// NewDispatcher constructs a Dispatcher. batch caps how many due deliveries are
// drained per tick.
func NewDispatcher(repo Repository, log *slog.Logger, batch int32) *Dispatcher {
	if batch <= 0 {
		batch = 50
	}
	return &Dispatcher{
		repo:   repo,
		client: &http.Client{Timeout: deliveryTimeout},
		log:    log,
		batch:  batch,
	}
}

// DispatchJob returns a worker.Job that drains one batch of due deliveries.
func (d *Dispatcher) DispatchJob() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		due, err := d.repo.ListDueWebhookDeliveries(ctx, d.batch)
		if err != nil {
			return err
		}
		for _, del := range due {
			d.deliver(ctx, del)
		}
		return nil
	}
}

// deliver attempts a single delivery and records the outcome. Any error path
// schedules a retry; it never propagates (one bad endpoint must not abort the
// batch).
func (d *Dispatcher) deliver(ctx context.Context, del db.WebhookDelivery) {
	ep, err := d.repo.GetWebhookEndpointByID(ctx, del.EndpointID)
	if err != nil {
		// Endpoint vanished (deleted) — park the delivery so we stop retrying.
		d.retry(ctx, del, "endpoint not found")
		return
	}

	status, err := d.post(ctx, ep, del)
	if err != nil {
		d.retry(ctx, del, err.Error())
		return
	}
	if status < 200 || status >= 300 {
		d.retry(ctx, del, fmt.Sprintf("non-2xx status: %d", status))
		return
	}
	if err := d.repo.MarkWebhookDelivered(ctx, del.ID); err != nil {
		d.logWarn("mark delivered failed", "delivery", del.ID, "err", err)
	}
}

// post signs and sends the delivery, returning the response status code.
func (d *Dispatcher) post(ctx context.Context, ep db.WebhookEndpoint, del db.WebhookDelivery) (int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, ep.Url, bytes.NewReader(del.Payload))
	if err != nil {
		return 0, err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(eventTypeHeader, del.EventType)
	req.Header.Set(deliveryHeader, del.ID.String())
	req.Header.Set(timestampHeader, ts)
	req.Header.Set(signatureHeader, sign(ep.Secret, ts, del.Payload))

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// retry schedules the next attempt with exponential backoff. MarkWebhookRetry
// parks the row as DEAD once attempts+1 reaches maxDeliveryAttempts.
func (d *Dispatcher) retry(ctx context.Context, del db.WebhookDelivery, reason string) {
	next := time.Now().Add(backoff(int(del.Attempts) + 1))
	err := d.repo.MarkWebhookRetry(ctx, db.MarkWebhookRetryParams{
		ID:            del.ID,
		LastError:     pgtype.Text{String: truncate(reason, 500), Valid: true},
		NextAttemptAt: pgtype.Timestamptz{Time: next, Valid: true},
		Attempts:      maxDeliveryAttempts,
	})
	if err != nil {
		d.logWarn("mark retry failed", "delivery", del.ID, "err", err)
	}
}

// sign computes the hex HMAC-SHA256 over "<timestamp>.<body>" so the signature
// is bound to both the payload and the send time (guards against replay).
func sign(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// backoff returns the delay before attempt n (1-indexed): 30s, 1m, 2m, 4m, ...
// capped at 30m.
func backoff(attempt int) time.Duration {
	d := 30 * time.Second
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= 30*time.Minute {
			return 30 * time.Minute
		}
	}
	return d
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (d *Dispatcher) logWarn(msg string, args ...any) {
	if d.log != nil {
		d.log.Warn(msg, args...)
	}
}
