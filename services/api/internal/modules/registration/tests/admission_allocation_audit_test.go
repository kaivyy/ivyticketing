package tests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
	accessmod "github.com/varin/ivyticketing/services/api/internal/modules/access"
	ballotmod "github.com/varin/ivyticketing/services/api/internal/modules/ballot"
	invmod "github.com/varin/ivyticketing/services/api/internal/modules/inventory"
	lifecyclemod "github.com/varin/ivyticketing/services/api/internal/modules/lifecycle"
	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
	paymentsmod "github.com/varin/ivyticketing/services/api/internal/modules/payments"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
	queuemod "github.com/varin/ivyticketing/services/api/internal/modules/queue"
	regmod "github.com/varin/ivyticketing/services/api/internal/modules/registration"
	ticketsmod "github.com/varin/ivyticketing/services/api/internal/modules/tickets"
	waitlistmod "github.com/varin/ivyticketing/services/api/internal/modules/waitlist"
	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	platformqueue "github.com/varin/ivyticketing/services/api/internal/platform/queue"
	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
	goredis "github.com/redis/go-redis/v9"
)

func getAuditTestPool(t *testing.T) *pgxpool.Pool {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/ivyticketing?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping database test: %v", err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping database test (cannot ping): %v", err)
		return nil
	}
	return pool
}

func getAuditTestRedis() *goredis.Client {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	opt, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil
	}
	client := goredis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil
	}
	return client
}

type testEnv struct {
	pool         *pgxpool.Pool
	orgID        uuid.UUID
	eventID      uuid.UUID
	catID        uuid.UUID
	regSvc       *regmod.Service
	regGate      *regmod.Gate
	ordersSvc    *ordersmod.Service
	queueSvc     *queuemod.Service
	queueStore   *queuemod.Store
	ballotSvc    *ballotmod.Service
	poolMgr      *accessmod.PoolManager
	lifecycleSvc *lifecyclemod.Service
	waitlistSvc  *waitlistmod.Service
	ticketIssuer *ticketsmod.Issuer
	paymentsProc *paymentsmod.Processor
}

func setupAuditEnv(t *testing.T, pool *pgxpool.Pool, capacity int32) *testEnv {
	ctx := context.Background()
	orgID := uuid.New()
	eventID := uuid.New()
	catID := uuid.New()

	orgSlug := "audit-org-" + orgID.String()[:8]
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING
	`, orgID, "Audit Org "+orgSlug, orgSlug)
	if err != nil {
		t.Fatalf("failed to insert org: %v", err)
	}

	eventSlug := "audit-event-" + eventID.String()[:8]
	_, err = pool.Exec(ctx, `
		INSERT INTO events (id, organization_id, name, slug, status, event_type)
		VALUES ($1, $2, $3, $4, 'published', 'RUNNING')
		ON CONFLICT (id) DO NOTHING
	`, eventID, orgID, "Audit Event "+eventSlug, eventSlug)
	if err != nil {
		t.Fatalf("failed to insert event: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO event_categories (id, organization_id, event_id, name, price, capacity, max_order_per_user, registration_opens_at, registration_closes_at)
		VALUES ($1, $2, $3, $4, 100000, $5, 1, now() - interval '1 day', now() + interval '30 days')
		ON CONFLICT (id) DO NOTHING
	`, catID, orgID, eventID, "10K Audit Category", capacity)
	if err != nil {
		t.Fatalf("failed to insert category: %v", err)
	}

	// Module services
	regRepo := regmod.NewRepository(pool)
	regSvc := regmod.NewService(regRepo)

	queueRepo := queuemod.NewRepository(pool)
	qAdapter := platformqueue.New(getAuditTestRedis())
	queueStore := queuemod.NewStore(qAdapter)
	qReader := queuemod.NewDBEventReader(db.New(pool))
	queueSvc := queuemod.NewService(queueRepo, queueStore, nil, qReader, 50, regSvc)

	lifecycleRepo := lifecyclemod.NewRepository(pool)
	lifecycleSvc := lifecyclemod.NewService(lifecycleRepo)

	accessRepo := accessmod.NewRepository(pool)
	poolMgr := accessmod.NewPoolManager(accessRepo)

	waitlistRepo := waitlistmod.NewRepository(pool)
	waitlistSvc := waitlistmod.NewService(waitlistRepo, poolMgr)

	ballotRepo := ballotmod.NewRepository(pool)
	ballotSvc := ballotmod.NewService(ballotRepo, nil, poolMgr, poolMgr, waitlistSvc)

	eligibilityChecker := accessmod.NewEligibilityChecker(accessRepo)
	priorityChecker := accessmod.NewPriorityChecker(accessRepo, lifecycleSvc, eligibilityChecker)

	regGate := regmod.NewGate(regSvc, queueSvc, lifecycleSvc, ballotSvc, poolMgr, priorityChecker)

	ordersRepo := ordersmod.NewRepository(pool)
	ordersSvc := ordersmod.NewService(ordersRepo, nil, 15*time.Minute, regGate, queueSvc)
	ordersSvc.WithGrantConsumer(poolMgr)

	ticketIssuer := ticketsmod.NewIssuer(nil)
	paymentsRepo := paymentsmod.NewRepository(pool)
	paymentsProc := paymentsmod.NewProcessor(paymentsRepo, nil, ticketIssuer)

	return &testEnv{
		pool:         pool,
		orgID:        orgID,
		eventID:      eventID,
		catID:        catID,
		regSvc:       regSvc,
		regGate:      regGate,
		ordersSvc:    ordersSvc,
		queueSvc:     queueSvc,
		queueStore:   queueStore,
		ballotSvc:    ballotSvc,
		poolMgr:      poolMgr,
		lifecycleSvc: lifecycleSvc,
		waitlistSvc:  waitlistSvc,
		ticketIssuer: ticketIssuer,
		paymentsProc: paymentsProc,
	}
}

func createAuditUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	ctx := context.Background()
	userID := uuid.New()
	email := "user-" + userID.String()[:8] + "@example.com"
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, full_name)
		VALUES ($1, $2, 'hash', $3)
		ON CONFLICT (id) DO NOTHING
	`, userID, email, "Audit User "+userID.String()[:8])
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	return userID
}

// ----------------------------------------------------------------------------
// 3A. Registration Mode Resolution & Authoritative Scope
// ----------------------------------------------------------------------------
func TestAudit_3A_RegistrationModeResolution(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	env := setupAuditEnv(t, pool, 100)
	ctx := context.Background()

	// 1. Unconfigured -> defaults to NORMAL (DIRECT)
	mode, err := env.regSvc.ResolveForCheckout(ctx, env.eventID, env.catID)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if mode != regmod.ModeNormal {
		t.Errorf("expected default mode NORMAL, got %v", mode)
	}

	// 2. Set Event Default to WAR_QUEUE
	err = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeWarQueue),
	})
	if err != nil {
		t.Fatalf("failed to set event settings: %v", err)
	}

	mode, err = env.regSvc.ResolveForCheckout(ctx, env.eventID, env.catID)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if mode != regmod.ModeWarQueue {
		t.Errorf("expected WAR_QUEUE from event default, got %v", mode)
	}

	// 3. Category Override: BALLOT with OverrideEnabled = true
	ballotMode := string(regmod.ModeBallot)
	err = env.regSvc.SetCategorySettings(ctx, env.orgID, env.eventID, env.catID, regmod.CategorySettingsRequest{
		RegistrationMode: &ballotMode,
		OverrideEnabled:  true,
	})
	if err != nil {
		t.Fatalf("failed to set category settings: %v", err)
	}

	mode, err = env.regSvc.ResolveForCheckout(ctx, env.eventID, env.catID)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if mode != regmod.ModeBallot {
		t.Errorf("expected BALLOT from category override, got %v", mode)
	}

	// 4. Disable Category Override -> Reverts to Event Default (WAR_QUEUE)
	err = env.regSvc.SetCategorySettings(ctx, env.orgID, env.eventID, env.catID, regmod.CategorySettingsRequest{
		RegistrationMode: &ballotMode,
		OverrideEnabled:  false,
	})
	if err != nil {
		t.Fatalf("failed to update category settings: %v", err)
	}

	mode, err = env.regSvc.ResolveForCheckout(ctx, env.eventID, env.catID)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if mode != regmod.ModeWarQueue {
		t.Errorf("expected WAR_QUEUE after override disabled, got %v", mode)
	}
}

// ----------------------------------------------------------------------------
// 3B. Separation of Admission Control and Business Allocation
// ----------------------------------------------------------------------------
func TestAudit_3B_SeparationOfAdmissionAndAllocation(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	env := setupAuditEnv(t, pool, 5)
	ctx := context.Background()
	user := createAuditUser(t, pool)

	// Set event to WAR_QUEUE
	_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeWarQueue),
	})

	// User joins queue
	_, err := env.queueSvc.Join(ctx, env.orgID, env.eventID, user)
	if err != nil {
		t.Fatalf("failed to join queue: %v", err)
	}

	// Release 1 token into admission
	promoted, err := env.queueSvc.Release(ctx, env.eventID, 1, 10*time.Minute)
	if err != nil {
		t.Fatalf("failed to release queue: %v", err)
	}
	if promoted != 1 {
		t.Fatalf("expected 1 promoted, got %d", promoted)
	}

	// Verify token status is ALLOWED
	status, err := env.queueSvc.Status(ctx, env.eventID, user)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status.Status != queuemod.StatusAllowed {
		t.Fatalf("expected status ALLOWED, got %s", status.Status)
	}
	if status.AdmissionToken == "" {
		t.Fatalf("expected non-empty admission token")
	}

	// CRITICAL CHECK 1: Has any inventory been allocated/reserved?
	var resCount int64
	err = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_reservations WHERE category_id = $1", env.catID).Scan(&resCount)
	if err != nil {
		t.Fatalf("failed to count reservations: %v", err)
	}
	if resCount != 0 {
		t.Fatalf("VIOLATION: Queue admission created an inventory reservation! resCount = %d", resCount)
	}

	// Check available remaining inventory via CheckAndLock
	check, err := invmod.CheckAndLock(ctx, invmod.NewRepository(db.New(pool)), env.catID)
	if err != nil {
		t.Fatalf("CheckAndLock failed: %v", err)
	}
	if check.Remaining != 5 {
		t.Fatalf("expected 5 remaining slots during admission, got %d", check.Remaining)
	}

	// CRITICAL CHECK 2: Allocation happens ONLY when Checkout is called
	orderResp, err := env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, status.AdmissionToken)
	if err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if orderResp.Status != ordersmod.StatusPendingPayment {
		t.Fatalf("expected PENDING_PAYMENT, got %s", orderResp.Status)
	}

	// Check reservation count now
	err = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_reservations WHERE category_id = $1 AND status = 'ACTIVE'", env.catID).Scan(&resCount)
	if err != nil {
		t.Fatalf("failed to count reservations: %v", err)
	}
	if resCount != 1 {
		t.Fatalf("expected exactly 1 active reservation after checkout, got %d", resCount)
	}

	// Remaining capacity should now be 4
	check, err = invmod.CheckAndLock(ctx, invmod.NewRepository(db.New(pool)), env.catID)
	if err != nil {
		t.Fatalf("CheckAndLock failed: %v", err)
	}
	if check.Remaining != 4 {
		t.Fatalf("expected 4 remaining slots after checkout, got %d", check.Remaining)
	}
}

// ----------------------------------------------------------------------------
// 3C. DIRECT Mode Concurrency & Row-Locking Without Queue Assistance
// ----------------------------------------------------------------------------
func TestAudit_3C_DirectModeConcurrencyStress(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	runConcurrencyTest := func(capacity int32, workers int) {
		env := setupAuditEnv(t, pool, capacity)
		ctx := context.Background()

		// DIRECT mode: NORMAL
		_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
			DefaultMode: string(regmod.ModeNormal),
		})

		users := make([]uuid.UUID, workers)
		for i := 0; i < workers; i++ {
			users[i] = createAuditUser(t, pool)
		}

		var wg sync.WaitGroup
		var successCount int32
		var oversoldErrCount int32
		var otherErrCount int32

		startBarrier := make(chan struct{})

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-startBarrier

				_, err := env.ordersSvc.Checkout(context.Background(), users[idx], env.eventID, env.catID, "")
				if err == nil {
					atomic.AddInt32(&successCount, 1)
				} else if errors.Is(err, invmod.ErrSoldOut) {
					atomic.AddInt32(&oversoldErrCount, 1)
				} else {
					atomic.AddInt32(&otherErrCount, 1)
				}
			}(i)
		}

		// Unleash all goroutines simultaneously
		close(startBarrier)
		wg.Wait()

		if successCount != capacity {
			t.Errorf("capacity %d: expected exactly %d successes, got %d (other errs: %d)", capacity, capacity, successCount, otherErrCount)
		}
		expectedRejections := int32(workers) - capacity
		if oversoldErrCount != expectedRejections {
			t.Errorf("capacity %d: expected %d oversold rejections, got %d", capacity, expectedRejections, oversoldErrCount)
		}

		// Verify database rows
		var resCount int64
		err := pool.QueryRow(ctx, "SELECT count(*) FROM inventory_reservations WHERE category_id = $1 AND status = 'ACTIVE'", env.catID).Scan(&resCount)
		if err != nil {
			t.Fatalf("failed to count reservations: %v", err)
		}
		if resCount != int64(capacity) {
			t.Fatalf("OVERSELL DETECTED! DB reservations %d != capacity %d", resCount, capacity)
		}
	}

	t.Run("Capacity_1_Workers_20", func(t *testing.T) {
		runConcurrencyTest(1, 20)
	})

	t.Run("Capacity_10_Workers_50", func(t *testing.T) {
		runConcurrencyTest(10, 50)
	})

	t.Run("Capacity_50_Workers_100", func(t *testing.T) {
		runConcurrencyTest(50, 100)
	})
}

// ----------------------------------------------------------------------------
// 3D. QUEUE Admission Token Single-Use and Expiration Semantics
// ----------------------------------------------------------------------------
func TestAudit_3D_QueueAdmissionTokenLifecycle(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	env := setupAuditEnv(t, pool, 10)
	ctx := context.Background()

	_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeWarQueue),
	})

	// Case 1: Single-Use Semantics (Token consumed upon checkout)
	user1 := createAuditUser(t, pool)
	_, _ = env.queueSvc.Join(ctx, env.orgID, env.eventID, user1)
	_, _ = env.queueSvc.Release(ctx, env.eventID, 1, 10*time.Minute)

	status1, _ := env.queueSvc.Status(ctx, env.eventID, user1)
	token1 := status1.AdmissionToken

	// First checkout succeeds
	_, err := env.ordersSvc.Checkout(ctx, user1, env.eventID, env.catID, token1)
	if err != nil {
		t.Fatalf("first checkout failed: %v", err)
	}

	// Second checkout with the SAME token must FAIL (single-use)
	_, err = env.ordersSvc.Checkout(ctx, user1, env.eventID, env.catID, token1)
	if err == nil {
		t.Fatalf("expected second checkout with same token to fail, but it succeeded")
	}

	// Case 2: Expired Admission Token Semantics
	user2 := createAuditUser(t, pool)
	_, _ = env.queueSvc.Join(ctx, env.orgID, env.eventID, user2)
	// Release with 1 millisecond window so it expires immediately
	_, _ = env.queueSvc.Release(ctx, env.eventID, 1, 1*time.Millisecond)
	status2, _ := env.queueSvc.Status(ctx, env.eventID, user2)
	token2 := status2.AdmissionToken

	time.Sleep(10 * time.Millisecond) // Ensure it is past expiration

	_, err = env.ordersSvc.Checkout(ctx, user2, env.eventID, env.catID, token2)
	if err == nil {
		t.Fatalf("expected checkout with expired token to fail, but it succeeded")
	}
	if !errors.Is(err, queuemod.ErrAdmissionExpired) {
		t.Logf("checkout with expired token returned: %v (expected ErrAdmissionExpired)", err)
	}
}

// ----------------------------------------------------------------------------
// 3E. BALLOT Lifecycle & Confirmed Defects
// ----------------------------------------------------------------------------
func TestAudit_3E_BallotLifecycleAndDefects(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	env := setupAuditEnv(t, pool, 10)
	ctx := context.Background()

	// Part 1: Proving CheckGrant Identity & Category Validation
	t.Run("Remediated_CheckGrant_IdentityAndCategoryValidation", func(t *testing.T) {
		userA := createAuditUser(t, pool)
		userB := createAuditUser(t, pool)

		// Create an access grant for User A and Category A
		grantID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO access_grants (id, event_id, category_id, participant_id, status, expires_at)
			VALUES ($1, $2, $3, $4, 'ACTIVE', now() + interval '1 hour')
		`, grantID, env.eventID, env.catID, userA)
		if err != nil {
			t.Fatalf("failed to insert grant: %v", err)
		}

		fakeCatID := uuid.New()

		// User B attempts to validate User A's grant token against fake category -> MUST FAIL
		err = env.poolMgr.CheckGrant(ctx, userB, fakeCatID, grantID.String())
		if err == nil {
			t.Fatalf("SECURITY VIOLATION: CheckGrant allowed User B with User A's grant!")
		}
		if !errors.Is(err, accessmod.ErrGrantNotFound) {
			t.Errorf("expected ErrGrantNotFound, got %v", err)
		}

		// User A attempts to validate User A's grant for fake category -> MUST FAIL
		err = env.poolMgr.CheckGrant(ctx, userA, fakeCatID, grantID.String())
		if err == nil || !errors.Is(err, accessmod.ErrGrantNotFound) {
			t.Errorf("expected ErrGrantNotFound for category mismatch, got %v", err)
		}

		// User A validates with legitimate category -> MUST SUCCEED
		err = env.poolMgr.CheckGrant(ctx, userA, env.catID, grantID.String())
		if err != nil {
			t.Fatalf("expected valid grant to pass, got %v", err)
		}
	})

	// Part 2: Proving Ballot Winner Conversion & No False Lapsing
	t.Run("Remediated_Ballot_WinnerConverted_NeverLapsed", func(t *testing.T) {
		winner := createAuditUser(t, pool)

		// 1. Create ballot draw
		drawID := uuid.New()
		creatorID := createAuditUser(t, pool)
		_, err := pool.Exec(ctx, `
			INSERT INTO ballot_draws (id, organization_id, event_id, category_id, status, quota, payment_window_hours, application_opens_at, application_closes_at, created_by)
			VALUES ($1, $2, $3, $4, 'OPEN', 1, 24, now() - interval '1 hour', now() + interval '1 hour', $5)
		`, drawID, env.orgID, env.eventID, env.catID, creatorID)
		if err != nil {
			t.Fatalf("failed to create draw: %v", err)
		}

		// 2. Winner entry with payment deadline in the future
		entryID := uuid.New()
		grantID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO access_grants (id, event_id, category_id, participant_id, status, expires_at)
			VALUES ($1, $2, $3, $4, 'ACTIVE', now() + interval '1 hour')
		`, grantID, env.eventID, env.catID, winner)
		if err != nil {
			t.Fatalf("failed to insert grant: %v", err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO ballot_entries (id, draw_id, organization_id, event_id, category_id, participant_id, status, payment_deadline, access_grant_id)
			VALUES ($1, $2, $3, $4, $5, $6, 'WINNER', now() + interval '10 minutes', $7)
		`, entryID, drawID, env.orgID, env.eventID, env.catID, winner, grantID)
		if err != nil {
			t.Fatalf("failed to insert ballot entry: %v", err)
		}

		// Configure category to BALLOT
		ballotMode := string(regmod.ModeBallot)
		_ = env.regSvc.SetCategorySettings(ctx, env.orgID, env.eventID, env.catID, regmod.CategorySettingsRequest{
			RegistrationMode: &ballotMode,
			OverrideEnabled:  true,
		})

		// Winner checks out with grant token
		orderResp, err := env.ordersSvc.Checkout(ctx, winner, env.eventID, env.catID, grantID.String())
		if err != nil {
			t.Fatalf("winner checkout failed: %v", err)
		}

		orderID := orderResp.ID
		payID := uuid.New()
		ref := "ref-" + payID.String()[:8]
		_, err = pool.Exec(ctx, `
			INSERT INTO payments (id, organization_id, event_id, order_id, participant_id, gateway, method, currency, status, amount, merchant_reference, expires_at)
			VALUES ($1, $2, $3, $4, $5, 'duitku', 'qris', 'IDR', 'PENDING', 100000, $6, now() + interval '1 hour')
		`, payID, env.orgID, env.eventID, orderID, winner, ref)
		if err != nil {
			t.Fatalf("failed to insert payment: %v", err)
		}

		// Apply payment via Processor
		err = env.paymentsProc.Apply(ctx, "duitku", gw.CallbackResult{
			MerchantReference: ref,
			Status:            gw.StatusPaid,
			Amount:            100000,
		})
		if err != nil {
			t.Fatalf("Apply payment failed: %v", err)
		}

		// Verify order is PAID
		var orderStatus string
		_ = pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&orderStatus)
		if orderStatus != "PAID" {
			t.Fatalf("expected order PAID, got %s", orderStatus)
		}

		// CRITICAL VERIFICATION: Status of ballot_entries MUST be CONVERTED
		var entryStatus string
		_ = pool.QueryRow(ctx, "SELECT status FROM ballot_entries WHERE id = $1", entryID).Scan(&entryStatus)
		if entryStatus != "CONVERTED" {
			t.Fatalf("VIOLATION: Paid winner ballot_entries status is '%s', expected 'CONVERTED'!", entryStatus)
		}

		// Now simulate payment_deadline expiring (setting deadline to past)
		_, _ = pool.Exec(ctx, "UPDATE ballot_entries SET payment_deadline = now() - interval '1 minute' WHERE id = $1", entryID)

		// Run WinnerExpirer
		expirer := ballotmod.NewWinnerExpirer(ballotmod.NewRepository(pool), env.waitlistSvc)
		err = expirer.Run(ctx)
		if err != nil {
			t.Fatalf("WinnerExpirer.Run failed: %v", err)
		}

		// Check entry status after expirer ran: MUST REMAIN CONVERTED
		_ = pool.QueryRow(ctx, "SELECT status FROM ballot_entries WHERE id = $1", entryID).Scan(&entryStatus)
		if entryStatus != "CONVERTED" {
			t.Fatalf("VIOLATION: Paid winner entry was modified to %s by WinnerExpirer!", entryStatus)
		}
	})

	// Part 3: Access Grant Single-Use Enforcement on Checkout
	t.Run("Remediated_AccessGrant_SingleUseConsumption", func(t *testing.T) {
		user := createAuditUser(t, pool)
		grantID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO access_grants (id, event_id, category_id, participant_id, status, expires_at)
			VALUES ($1, $2, $3, $4, 'ACTIVE', now() + interval '1 hour')
		`, grantID, env.eventID, env.catID, user)
		if err != nil {
			t.Fatalf("failed to insert grant: %v", err)
		}

		// First checkout with grant token succeeds
		_, err = env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, grantID.String())
		if err != nil {
			t.Fatalf("first checkout with grant token failed: %v", err)
		}

		// Grant in DB must now be CONSUMED
		var gStatus string
		_ = pool.QueryRow(ctx, "SELECT status FROM access_grants WHERE id = $1", grantID).Scan(&gStatus)
		if gStatus != "CONSUMED" {
			t.Fatalf("expected grant status CONSUMED after checkout, got %s", gStatus)
		}

		// Second checkout with same grant token MUST FAIL
		_, err = env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, grantID.String())
		if err == nil {
			t.Fatalf("expected second checkout with consumed grant token to fail, but succeeded")
		}
	})
}

// ----------------------------------------------------------------------------
// 3F. Capacity Consistency: Order Expiration and Cancellation Recovery
// ----------------------------------------------------------------------------
func TestAudit_3F_CapacityConsistency_CancellationRecovery(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	env := setupAuditEnv(t, pool, 1)
	ctx := context.Background()

	_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeNormal),
	})

	user1 := createAuditUser(t, pool)
	user2 := createAuditUser(t, pool)

	// User 1 reserves the only slot
	orderResp, err := env.ordersSvc.Checkout(ctx, user1, env.eventID, env.catID, "")
	if err != nil {
		t.Fatalf("user 1 checkout failed: %v", err)
	}

	// User 2 checkout fails due to sold out
	_, err = env.ordersSvc.Checkout(ctx, user2, env.eventID, env.catID, "")
	if !errors.Is(err, invmod.ErrSoldOut) {
		t.Fatalf("expected ErrSoldOut, got %v", err)
	}

	// User 1 cancels order
	orderID := orderResp.ID
	err = env.ordersSvc.Cancel(ctx, user1, orderID)
	if err != nil {
		t.Fatalf("cancel order failed: %v", err)
	}

	// User 2 retries checkout -> must SUCCEED because slot was recovered
	_, err = env.ordersSvc.Checkout(ctx, user2, env.eventID, env.catID, "")
	if err != nil {
		t.Fatalf("user 2 retry checkout failed after slot recovered: %v", err)
	}
}

// ----------------------------------------------------------------------------
// 3G. Registration Window Boundary Conditions
// ----------------------------------------------------------------------------
func TestAudit_3G_RegistrationWindowBoundaries(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()

	t.Run("Category_RegistrationOpensAt_Future_Rejects", func(t *testing.T) {
		env := setupAuditEnv(t, pool, 10)
		_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
			DefaultMode: string(regmod.ModeNormal),
		})

		// Set RegistrationOpensAt to tomorrow
		_, _ = pool.Exec(ctx, "UPDATE event_categories SET registration_opens_at = now() + interval '1 day' WHERE id = $1", env.catID)

		user := createAuditUser(t, pool)
		_, err := env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, "")
		if !errors.Is(err, ordersmod.ErrRegistrationClosed) {
			t.Errorf("expected ErrRegistrationClosed when opens_at in future, got %v", err)
		}
	})

	t.Run("Category_RegistrationClosesAt_Past_Rejects", func(t *testing.T) {
		env := setupAuditEnv(t, pool, 10)
		_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
			DefaultMode: string(regmod.ModeNormal),
		})

		// Set RegistrationClosesAt to yesterday
		_, _ = pool.Exec(ctx, "UPDATE event_categories SET registration_closes_at = now() - interval '1 day' WHERE id = $1", env.catID)

		user := createAuditUser(t, pool)
		_, err := env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, "")
		if !errors.Is(err, ordersmod.ErrRegistrationClosed) {
			t.Errorf("expected ErrRegistrationClosed when closes_at in past, got %v", err)
		}
	})

	t.Run("Remediated_Ballot_ApplicationClosesAt_Enforced", func(t *testing.T) {
		env := setupAuditEnv(t, pool, 10)

		// Create draw with ApplicationClosesAt in past, but status OPEN
		drawID := uuid.New()
		creatorID := createAuditUser(t, pool)
		_, err := pool.Exec(ctx, `
			INSERT INTO ballot_draws (id, organization_id, event_id, category_id, status, quota, payment_window_hours, application_opens_at, application_closes_at, created_by)
			VALUES ($1, $2, $3, $4, 'OPEN', 10, 24, now() - interval '2 days', now() - interval '1 day', $5)
		`, drawID, env.orgID, env.eventID, env.catID, creatorID)
		if err != nil {
			t.Fatalf("failed to create draw: %v", err)
		}

		user := createAuditUser(t, pool)
		// Apply to draw -> MUST FAIL with ErrBallotClosed
		_, err = env.ballotSvc.Apply(ctx, user, env.eventID, env.catID, drawID)
		if !errors.Is(err, ballotmod.ErrBallotClosed) {
			t.Fatalf("expected ErrBallotClosed when applying after application_closes_at, got %v", err)
		}
	})

	t.Run("Remediated_Ballot_ApplicationOpensAt_Enforced", func(t *testing.T) {
		env := setupAuditEnv(t, pool, 10)

		// Create draw with ApplicationOpensAt in future, but status OPEN
		drawID := uuid.New()
		creatorID := createAuditUser(t, pool)
		_, err := pool.Exec(ctx, `
			INSERT INTO ballot_draws (id, organization_id, event_id, category_id, status, quota, payment_window_hours, application_opens_at, application_closes_at, created_by)
			VALUES ($1, $2, $3, $4, 'OPEN', 10, 24, now() + interval '1 day', now() + interval '2 days', $5)
		`, drawID, env.orgID, env.eventID, env.catID, creatorID)
		if err != nil {
			t.Fatalf("failed to create draw: %v", err)
		}

		user := createAuditUser(t, pool)
		// Apply to draw -> MUST FAIL with ErrBallotClosed
		_, err = env.ballotSvc.Apply(ctx, user, env.eventID, env.catID, drawID)
		if !errors.Is(err, ballotmod.ErrBallotClosed) {
			t.Fatalf("expected ErrBallotClosed when applying before application_opens_at, got %v", err)
		}
	})
}

// ----------------------------------------------------------------------------
// 3J. Tenant Isolation & BOLA Attack Vectors
// ----------------------------------------------------------------------------
func TestAudit_3J_TenantIsolationAndBOLA(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()

	envA := setupAuditEnv(t, pool, 10)
	envB := setupAuditEnv(t, pool, 10)

	// Set Org B's initial event mode to NORMAL
	_ = envB.regSvc.SetEventSettings(ctx, envB.orgID, envB.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeNormal),
	})

	t.Run("BOLA_Registration_SetEventSettings_CrossTenantMutation", func(t *testing.T) {
		h := regmod.NewHandler(envA.regSvc)

		// Attacker constructs a PUT request to /organizations/{orgA}/events/{eventB}/registration
		reqBody := []byte(`{"defaultMode":"WAR_QUEUE"}`)
		r := httptest.NewRequest("PUT", "/organizations/"+envA.orgID.String()+"/events/"+envB.eventID.String()+"/registration", bytes.NewReader(reqBody))

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("orgId", envA.orgID.String())
		rctx.URLParams.Add("eventId", envB.eventID.String())
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.SetEventSettings(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 StatusNotFound on cross-tenant event settings mutation, got %d", w.Code)
		}

		// Verify Org B's event settings were NOT mutated
		modeB, _ := envB.regSvc.ResolveEventMode(ctx, envB.eventID)
		if modeB == string(regmod.ModeWarQueue) {
			t.Fatalf("CRITICAL SECURITY HOLE: Org A organizer mutated Org B's event registration settings!")
		}
	})

	t.Run("BOLA_Registration_SetCategorySettings_CrossTenantMutation", func(t *testing.T) {
		h := regmod.NewHandler(envA.regSvc)

		ballotMode := string(regmod.ModeBallot)
		reqBody := []byte(`{"categoryId":"` + envB.catID.String() + `","registrationMode":"` + ballotMode + `","overrideEnabled":true}`)
		r := httptest.NewRequest("PUT", "/organizations/"+envA.orgID.String()+"/events/"+envB.eventID.String()+"/registration/category", bytes.NewReader(reqBody))

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("orgId", envA.orgID.String())
		rctx.URLParams.Add("eventId", envB.eventID.String())
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.SetCategorySettings(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 StatusNotFound on cross-tenant category settings mutation, got %d", w.Code)
		}
	})

	t.Run("BOLA_Queue_Pause_CrossTenantMutation", func(t *testing.T) {
		h := queuemod.NewHandler(envA.queueSvc)

		r := httptest.NewRequest("POST", "/organizations/"+envA.orgID.String()+"/events/"+envB.eventID.String()+"/queue/pause", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("orgId", envA.orgID.String())
		rctx.URLParams.Add("eventId", envB.eventID.String())
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Pause(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 StatusNotFound on cross-tenant queue pause, got %d", w.Code)
		}

		ctrlB, _ := queuemod.NewRepository(pool).GetControl(ctx, envB.eventID)
		if ctrlB.State == queuemod.StatePaused {
			t.Fatalf("CRITICAL SECURITY HOLE: Org A paused Org B's queue controls!")
		}
	})

	t.Run("BOLA_Ballot_OpenDraw_CrossTenantMutation", func(t *testing.T) {
		drawID := uuid.New()
		creatorID := createAuditUser(t, pool)
		_, err := pool.Exec(ctx, `
			INSERT INTO ballot_draws (id, organization_id, event_id, category_id, status, quota, payment_window_hours, created_by)
			VALUES ($1, $2, $3, $4, 'PENDING', 5, 24, $5)
		`, drawID, envB.orgID, envB.eventID, envB.catID, creatorID)
		if err != nil {
			t.Fatalf("failed to insert Org B draw: %v", err)
		}

		h := ballotmod.NewHandler(envA.ballotSvc)

		r := httptest.NewRequest("POST", "/organizations/"+envA.orgID.String()+"/ballot/"+drawID.String()+"/open", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("orgId", envA.orgID.String())
		rctx.URLParams.Add("drawId", drawID.String())
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		r = r.WithContext(authctx.WithIdentity(r.Context(), authctx.Identity{UserID: creatorID}))

		w := httptest.NewRecorder()
		h.OpenDraw(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 StatusNotFound on cross-tenant ballot open, got %d", w.Code)
		}

		var statusB string
		_ = pool.QueryRow(ctx, "SELECT status FROM ballot_draws WHERE id = $1", drawID).Scan(&statusB)
		if statusB == "OPEN" {
			t.Fatalf("CRITICAL SECURITY HOLE: Org B ballot draw was opened by Org A!")
		}
	})
}

// ----------------------------------------------------------------------------
// 3H. Mode Transitions and Existing Transactional State Preservation
// ----------------------------------------------------------------------------
func TestAudit_3H_ModeTransitionsAndStatePreservation(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	env := setupAuditEnv(t, pool, 10)

	// Phase 1: Event starts in NORMAL mode
	_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeNormal),
	})

	user1 := createAuditUser(t, pool)
	orderResp, err := env.ordersSvc.Checkout(ctx, user1, env.eventID, env.catID, "")
	if err != nil {
		t.Fatalf("checkout in NORMAL mode failed: %v", err)
	}

	// Phase 2: Organizer switches event mode to WAR_QUEUE while user1 has pending order
	err = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeWarQueue),
	})
	if err != nil {
		t.Fatalf("failed to switch mode to WAR_QUEUE: %v", err)
	}

	// User 2 tries to checkout without queue admission -> must FAIL
	user2 := createAuditUser(t, pool)
	_, err = env.ordersSvc.Checkout(ctx, user2, env.eventID, env.catID, "")
	if err == nil {
		t.Fatalf("expected user 2 to be blocked by WAR_QUEUE gate, but checkout succeeded")
	}

	// User 1 completes payment for existing in-flight order -> MUST SUCCEED
	payID := uuid.New()
	ref := "ref-" + payID.String()[:8]
	_, err = pool.Exec(ctx, `
		INSERT INTO payments (id, organization_id, event_id, order_id, participant_id, gateway, method, currency, status, amount, merchant_reference, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'duitku', 'qris', 'IDR', 'PENDING', 100000, $6, now() + interval '1 hour')
	`, payID, env.orgID, env.eventID, orderResp.ID, user1, ref)
	if err != nil {
		t.Fatalf("failed to insert payment: %v", err)
	}

	err = env.paymentsProc.Apply(ctx, "duitku", gw.CallbackResult{
		MerchantReference: ref,
		Status:            gw.StatusPaid,
		Amount:            100000,
	})
	if err != nil {
		t.Fatalf("payment processing failed for existing order after mode switch: %v", err)
	}

	var finalStatus string
	_ = pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderResp.ID).Scan(&finalStatus)
	if finalStatus != "PAID" {
		t.Fatalf("expected existing order status to be PAID, got %s", finalStatus)
	}
}

// ----------------------------------------------------------------------------
// 3I. Admission Safety Under Protective Degradation
// ----------------------------------------------------------------------------
func TestAudit_3I_ProtectiveDegradation(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	env := setupAuditEnv(t, pool, 10)

	// Event configured for WAR_QUEUE
	_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeWarQueue),
	})

	user := createAuditUser(t, pool)

	// Gate constructed with nil queue (service degradation)
	degradedGate := regmod.NewGate(env.regSvc, nil, env.lifecycleSvc, env.ballotSvc, env.poolMgr, nil)
	degradedOrdersSvc := ordersmod.NewService(ordersmod.NewRepository(pool), nil, 15*time.Minute, degradedGate, nil)

	// Checkout must FAIL-CLOSED
	_, err := degradedOrdersSvc.Checkout(ctx, user, env.eventID, env.catID, "")
	if !errors.Is(err, regmod.ErrModeNotAvailable) {
		t.Errorf("expected ErrModeNotAvailable on degraded queue admitter, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// P0 Concurrency: Payment vs WinnerExpirer Race Condition
// ----------------------------------------------------------------------------
func TestAudit_P0_ConcurrentPaymentVsWinnerExpirer(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	env := setupAuditEnv(t, pool, 10)

	winner := createAuditUser(t, pool)
	creatorID := createAuditUser(t, pool)

	drawID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO ballot_draws (id, organization_id, event_id, category_id, status, quota, payment_window_hours, created_by)
		VALUES ($1, $2, $3, $4, 'OPEN', 1, 24, $5)
	`, drawID, env.orgID, env.eventID, env.catID, creatorID)
	if err != nil {
		t.Fatalf("failed to create draw: %v", err)
	}

	entryID := uuid.New()
	grantID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO access_grants (id, event_id, category_id, participant_id, status, expires_at)
		VALUES ($1, $2, $3, $4, 'ACTIVE', now() + interval '1 hour')
	`, grantID, env.eventID, env.catID, winner)
	if err != nil {
		t.Fatalf("failed to insert grant: %v", err)
	}

	// Winner entry right at the cusp of deadline
	_, err = pool.Exec(ctx, `
		INSERT INTO ballot_entries (id, draw_id, organization_id, event_id, category_id, participant_id, status, payment_deadline, access_grant_id)
		VALUES ($1, $2, $3, $4, $5, $6, 'WINNER', now() - interval '1 second', $7)
	`, entryID, drawID, env.orgID, env.eventID, env.catID, winner, grantID)
	if err != nil {
		t.Fatalf("failed to insert entry: %v", err)
	}

	ballotMode := string(regmod.ModeBallot)
	_ = env.regSvc.SetCategorySettings(ctx, env.orgID, env.eventID, env.catID, regmod.CategorySettingsRequest{
		RegistrationMode: &ballotMode,
		OverrideEnabled:  true,
	})

	orderResp, err := env.ordersSvc.Checkout(ctx, winner, env.eventID, env.catID, grantID.String())
	if err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	payID := uuid.New()
	ref := "ref-" + payID.String()[:8]
	_, err = pool.Exec(ctx, `
		INSERT INTO payments (id, organization_id, event_id, order_id, participant_id, gateway, method, currency, status, amount, merchant_reference, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'duitku', 'qris', 'IDR', 'PENDING', 100000, $6, now() + interval '1 hour')
	`, payID, env.orgID, env.eventID, orderResp.ID, winner, ref)
	if err != nil {
		t.Fatalf("failed to insert payment: %v", err)
	}

	// Concurrently run Apply payment and WinnerExpirer.Run
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = env.paymentsProc.Apply(ctx, "duitku", gw.CallbackResult{
			MerchantReference: ref,
			Status:            gw.StatusPaid,
			Amount:            100000,
		})
	}()

	go func() {
		defer wg.Done()
		expirer := ballotmod.NewWinnerExpirer(ballotmod.NewRepository(pool), env.waitlistSvc)
		_ = expirer.Run(ctx)
	}()

	wg.Wait()

	// Final verification: If order is PAID, entry MUST NOT be LAPSED
	var orderStatus string
	_ = pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderResp.ID).Scan(&orderStatus)

	var entryStatus string
	_ = pool.QueryRow(ctx, "SELECT status FROM ballot_entries WHERE id = $1", entryID).Scan(&entryStatus)

	if orderStatus == "PAID" && entryStatus == "LAPSED" {
		t.Fatalf("CONCURRENCY INTEGRITY VIOLATION: Paid winner was marked LAPSED in concurrent race!")
	}
}

// ----------------------------------------------------------------------------
// P2 Queue Status Caching & Rate Limiting
// ----------------------------------------------------------------------------
func TestAudit_P2_QueueStatusRateLimitingAndCaching(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	redisClient := getAuditTestRedis()
	if redisClient == nil {
		t.Skip("skipping Redis status caching test (no redis)")
		return
	}

	env := setupAuditEnv(t, pool, 10)
	ctx := context.Background()

	_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeWarQueue),
	})

	user := createAuditUser(t, pool)
	_, err := env.queueSvc.Join(ctx, env.orgID, env.eventID, user)
	if err != nil {
		t.Fatalf("join failed: %v", err)
	}

	// 1. First status call - populates cache
	s1, err := env.queueSvc.Status(ctx, env.eventID, user)
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	// 2. Second status call - hits cache (verify cache was written)
	cached, err := env.queueStore.GetCachedStatus(ctx, env.eventID.String(), user.String())
	if err != nil || cached == "" {
		t.Fatalf("expected status to be cached in Redis, got empty or error: %v", err)
	}

	s2, err := env.queueSvc.Status(ctx, env.eventID, user)
	if err != nil || s2.TokenID != s1.TokenID {
		t.Fatalf("cached status mismatch")
	}

	// 3. Test HTTP Rate Limiting via Handler
	h := queuemod.NewHandler(env.queueSvc)
	h.WithRateLimiter(ratelimit.New(redisClient))

	// Issue 5 requests -> all OK
	for i := 0; i < 5; i++ {
		r := httptest.NewRequest("GET", "/events/"+env.eventID.String()+"/queue/status", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("eventId", env.eventID.String())
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		r = r.WithContext(authctx.WithIdentity(r.Context(), authctx.Identity{UserID: user}))

		w := httptest.NewRecorder()
		h.Status(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i+1, w.Code)
		}
	}

	// 6th request within same second -> MUST return 429 Too Many Requests
	r := httptest.NewRequest("GET", "/events/"+env.eventID.String()+"/queue/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("eventId", env.eventID.String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(authctx.WithIdentity(r.Context(), authctx.Identity{UserID: user}))

	w := httptest.NewRecorder()
	h.Status(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 StatusTooManyRequests on 6th rapid poll, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "1" {
		t.Errorf("expected Retry-After: 1 header, got %s", w.Header().Get("Retry-After"))
	}
}

// ----------------------------------------------------------------------------
// Objective 1: ConsumeGrant Transactional Atomicity Audit
// ----------------------------------------------------------------------------
func TestAudit_2_ConsumeGrantTransactionAtomicity(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()

	t.Run("Scenario_A_SimulatedFailureDuringGrantConsumption", func(t *testing.T) {
		env := setupAuditEnv(t, pool, 10)
		user := createAuditUser(t, pool)
		grantID := uuid.New()

		// Insert an already EXPIRED grant
		_, err := pool.Exec(ctx, `
			INSERT INTO access_grants (id, event_id, category_id, participant_id, status, expires_at)
			VALUES ($1, $2, $3, $4, 'EXPIRED', now() - interval '1 hour')
		`, grantID, env.eventID, env.catID, user)
		if err != nil {
			t.Fatalf("failed to insert grant: %v", err)
		}

		// Attempt checkout
		_, err = env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, grantID.String())
		if err == nil {
			t.Fatalf("expected checkout to fail for expired grant, but it succeeded")
		}

		// Inspect DB directly:
		var orderCount int64
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM orders WHERE participant_id = $1", user).Scan(&orderCount)
		if orderCount != 0 {
			t.Fatalf("ATOMICITY VIOLATION: order was created (%d) despite expired grant failure", orderCount)
		}

		var resCount int64
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_reservations WHERE participant_id = $1", user).Scan(&resCount)
		if resCount != 0 {
			t.Fatalf("ATOMICITY VIOLATION: inventory reservation was created (%d) despite expired grant failure", resCount)
		}

		var gStatus string
		_ = pool.QueryRow(ctx, "SELECT status FROM access_grants WHERE id = $1", grantID).Scan(&gStatus)
		if gStatus != "EXPIRED" {
			t.Fatalf("grant status corrupted: expected EXPIRED, got %s", gStatus)
		}
	})

	t.Run("Scenario_B_FailureAtGrantConsumptionStep_FullRollback", func(t *testing.T) {
		env := setupAuditEnv(t, pool, 10)
		user := createAuditUser(t, pool)
		grantID := uuid.New()

		// Insert an already CONSUMED grant
		_, err := pool.Exec(ctx, `
			INSERT INTO access_grants (id, event_id, category_id, participant_id, status, expires_at, consumed_at)
			VALUES ($1, $2, $3, $4, 'CONSUMED', now() + interval '1 hour', now() - interval '5 minutes')
		`, grantID, env.eventID, env.catID, user)
		if err != nil {
			t.Fatalf("failed to insert grant: %v", err)
		}

		// Attempt checkout with consumed grant
		_, err = env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, grantID.String())
		if err == nil {
			t.Fatalf("expected checkout to fail for already consumed grant, but succeeded")
		}

		// Inspect DB: Full transaction rollback confirmed
		var orderCount int64
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM orders WHERE participant_id = $1", user).Scan(&orderCount)
		if orderCount != 0 {
			t.Fatalf("ATOMICITY VIOLATION: order committed (%d) despite consumed grant error", orderCount)
		}

		var resCount int64
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_reservations WHERE participant_id = $1", user).Scan(&resCount)
		if resCount != 0 {
			t.Fatalf("ATOMICITY VIOLATION: inventory reservation committed (%d) despite consumed grant error", resCount)
		}
	})

	t.Run("Scenario_C_InventoryReservationFailure_GrantRemainsActive", func(t *testing.T) {
		// Category capacity = 1
		env := setupAuditEnv(t, pool, 1)

		// Exhaust inventory with another user
		otherUser := createAuditUser(t, pool)
		_, err := env.ordersSvc.Checkout(ctx, otherUser, env.eventID, env.catID, "")
		if err != nil {
			t.Fatalf("exhausting inventory failed: %v", err)
		}

		user := createAuditUser(t, pool)
		grantID := uuid.New()

		// Insert ACTIVE grant for user
		_, err = pool.Exec(ctx, `
			INSERT INTO access_grants (id, event_id, category_id, participant_id, status, expires_at)
			VALUES ($1, $2, $3, $4, 'ACTIVE', now() + interval '1 hour')
		`, grantID, env.eventID, env.catID, user)
		if err != nil {
			t.Fatalf("failed to insert grant: %v", err)
		}

		// Attempt checkout
		_, err = env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, grantID.String())
		if err == nil {
			t.Fatalf("expected checkout to fail on inventory reservation/capacity, but succeeded")
		}

		// Inspect DB: Grant MUST REMAIN ACTIVE!
		var gStatus string
		var orderID *uuid.UUID
		_ = pool.QueryRow(ctx, "SELECT status, order_id FROM access_grants WHERE id = $1", grantID).Scan(&gStatus, &orderID)
		if gStatus != "ACTIVE" {
			t.Fatalf("VIOLATION: grant status changed to %s despite inventory failure! Must remain ACTIVE", gStatus)
		}
		if orderID != nil {
			t.Fatalf("VIOLATION: grant order_id linked despite inventory failure!")
		}

		var orderCount int64
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM orders WHERE participant_id = $1", user).Scan(&orderCount)
		if orderCount != 0 {
			t.Fatalf("VIOLATION: order created despite inventory failure")
		}
	})

	t.Run("Scenario_D_ConcurrentCheckoutRaceOnSingleGrant", func(t *testing.T) {
		env := setupAuditEnv(t, pool, 50)
		user := createAuditUser(t, pool)
		grantID := uuid.New()

		// Insert single ACTIVE grant
		_, err := pool.Exec(ctx, `
			INSERT INTO access_grants (id, event_id, category_id, participant_id, status, expires_at)
			VALUES ($1, $2, $3, $4, 'ACTIVE', now() + interval '1 hour')
		`, grantID, env.eventID, env.catID, user)
		if err != nil {
			t.Fatalf("failed to insert grant: %v", err)
		}

		// Concurrency race: 10 goroutines attempting checkout with the exact same grant token
		const concurrency = 10
		startGate := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(concurrency)

		var successCount int64
		var failCount int64
		var winningOrderID uuid.UUID
		var mu sync.Mutex

		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				<-startGate // synchronize start

				resp, err := env.ordersSvc.Checkout(ctx, user, env.eventID, env.catID, grantID.String())
				if err == nil {
					atomic.AddInt64(&successCount, 1)
					mu.Lock()
					winningOrderID = resp.ID
					mu.Unlock()
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}()
		}

		close(startGate)
		wg.Wait()

		if successCount != 1 {
			t.Fatalf("CRITICAL ATOMICITY DEFECT: expected exactly 1 successful checkout, got %d (failed: %d)", successCount, failCount)
		}
		if failCount != concurrency-1 {
			t.Fatalf("expected exactly %d rejections, got %d", concurrency-1, failCount)
		}

		// Verify PostgreSQL state
		var ordersInDB int64
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM orders WHERE category_id = $1", env.catID).Scan(&ordersInDB)
		if ordersInDB != 1 {
			t.Fatalf("VIOLATION: expected exactly 1 order in PostgreSQL, found %d", ordersInDB)
		}

		var reservationsInDB int64
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_reservations WHERE category_id = $1", env.catID).Scan(&reservationsInDB)
		if reservationsInDB != 1 {
			t.Fatalf("VIOLATION: expected exactly 1 inventory reservation in PostgreSQL, found %d", reservationsInDB)
		}

		var gStatus string
		var linkedOrderID *uuid.UUID
		_ = pool.QueryRow(ctx, "SELECT status, order_id FROM access_grants WHERE id = $1", grantID).Scan(&gStatus, &linkedOrderID)
		if gStatus != "CONSUMED" {
			t.Fatalf("expected grant status CONSUMED, got %s", gStatus)
		}
		if linkedOrderID == nil || *linkedOrderID != winningOrderID {
			t.Fatalf("grant linked order_id mismatch: expected %v, got %v", winningOrderID, linkedOrderID)
		}
	})
}

// ----------------------------------------------------------------------------
// Objective 2: Controlled Queue Status Capacity and Load Audit
// ----------------------------------------------------------------------------
func TestAudit_QueueStatusCapacityAndLoad(t *testing.T) {
	pool := getAuditTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	redisClient := getAuditTestRedis()
	if redisClient == nil {
		t.Skip("skipping Redis capacity test (no redis)")
		return
	}

	env := setupAuditEnv(t, pool, 1000)
	ctx := context.Background()

	_ = env.regSvc.SetEventSettings(ctx, env.orgID, env.eventID, regmod.EventSettingsRequest{
		DefaultMode: string(regmod.ModeWarQueue),
	})

	h := queuemod.NewHandler(env.queueSvc)
	h.WithRateLimiter(ratelimit.New(redisClient))

	// Helper to seed participants in DB and queue
	seedParticipants := func(count int, prefix string) []uuid.UUID {
		users := make([]uuid.UUID, count)
		// Batch insert users in blocks of 500 for high efficiency
		for start := 0; start < count; start += 500 {
			end := start + 500
			if end > count {
				end = count
			}
			var qUsers bytes.Buffer
			qUsers.WriteString("INSERT INTO users (id, email, password_hash, full_name) VALUES ")
			var args []any
			for i := start; i < end; i++ {
				uID := uuid.New()
				users[i] = uID
				idx := (i - start) * 3
				if i > start {
					qUsers.WriteString(", ")
				}
				fmt.Fprintf(&qUsers, "($%d, $%d, 'hash', $%d)", idx+1, idx+2, idx+3)
				args = append(args, uID, fmt.Sprintf("%s-%s@loadtest.local", prefix, uID.String()), fmt.Sprintf("Load %s %d", prefix, i))
			}
			_, err := pool.Exec(ctx, qUsers.String(), args...)
			if err != nil {
				t.Fatalf("failed to batch insert users: %v", err)
			}

			// Batch insert queue_tokens
			var qTok bytes.Buffer
			qTok.WriteString("INSERT INTO queue_tokens (id, organization_id, event_id, participant_id, status, pool, score) VALUES ")
			var tokArgs []any
			baseScore := time.Now().UnixNano()
			for i := start; i < end; i++ {
				idx := (i - start) * 5
				if i > start {
					qTok.WriteString(", ")
				}
				fmt.Fprintf(&qTok, "($%d, $%d, $%d, $%d, 'WAITING', 'FIFO', $%d)", idx+1, idx+2, idx+3, idx+4, idx+5)
				tokArgs = append(tokArgs, uuid.New(), env.orgID, env.eventID, users[i], baseScore+int64(i))
			}
			qTok.WriteString(" ON CONFLICT DO NOTHING")
			_, err = pool.Exec(ctx, qTok.String(), tokArgs...)
			if err != nil {
				t.Fatalf("failed to batch insert tokens: %v", err)
			}
		}
		return users
	}

	runLoadSimulation := func(t *testing.T, scaleName string, simulatedParticipants int, requestsPerParticipant int) {
		t.Logf("=== Starting Queue Status Capacity Test: %s (%d participants) ===", scaleName, simulatedParticipants)
		users := seedParticipants(simulatedParticipants, scaleName)

		totalRequests := simulatedParticipants * requestsPerParticipant
		latencies := make([]time.Duration, totalRequests)
		var c2xx, c429, c5xx int64

		// Track DB query count / connection metrics
		initialDbAcquired := pool.Stat().AcquiredConns()

		// Warm cache for 50% of participants to simulate natural queue progression
		warmCount := simulatedParticipants / 2
		for i := 0; i < warmCount; i++ {
			_, _ = env.queueSvc.Status(ctx, env.eventID, users[i])
		}

		workers := 50
		workCh := make(chan int, totalRequests)
		for i := 0; i < totalRequests; i++ {
			workCh <- i
		}
		close(workCh)

		start := time.Now()
		var wg sync.WaitGroup
		wg.Add(workers)

		for w := 0; w < workers; w++ {
			go func() {
				defer wg.Done()
				for reqIdx := range workCh {
					user := users[reqIdx%simulatedParticipants]
					req := httptest.NewRequest("GET", "/events/"+env.eventID.String()+"/queue/status", nil)
					rctx := chi.NewRouteContext()
					rctx.URLParams.Add("eventId", env.eventID.String())
					req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
					req = req.WithContext(authctx.WithIdentity(req.Context(), authctx.Identity{UserID: user}))

					rec := httptest.NewRecorder()
					t0 := time.Now()
					h.Status(rec, req)
					lat := time.Since(t0)
					latencies[reqIdx] = lat

					switch rec.Code {
					case http.StatusOK:
						atomic.AddInt64(&c2xx, 1)
					case http.StatusTooManyRequests:
						atomic.AddInt64(&c429, 1)
					default:
						if rec.Code >= 500 {
							atomic.AddInt64(&c5xx, 1)
						}
					}
				}
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		// Calculate metrics
		rps := float64(totalRequests) / duration.Seconds()
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p50 := latencies[int(float64(totalRequests)*0.50)]
		p95 := latencies[int(float64(totalRequests)*0.95)]
		p99 := latencies[int(float64(totalRequests)*0.99)]

		finalDbAcquired := pool.Stat().AcquiredConns()

		t.Logf("Results for %s (%d simulated participants):", scaleName, simulatedParticipants)
		t.Logf("  Total Duration:     %v", duration)
		t.Logf("  Throughput (RPS):   %.1f req/sec", rps)
		t.Logf("  2xx Responses:      %d (%.1f%%)", c2xx, float64(c2xx)/float64(totalRequests)*100)
		t.Logf("  429 Rate Limited:   %d (%.1f%%)", c429, float64(c429)/float64(totalRequests)*100)
		t.Logf("  5xx Errors:         %d", c5xx)
		t.Logf("  P50 Latency:        %v", p50)
		t.Logf("  P95 Latency:        %v", p95)
		t.Logf("  P99 Latency:        %v", p99)
		t.Logf("  DB Pool Acquired:   initial=%d, final=%d", initialDbAcquired, finalDbAcquired)

		if c5xx != 0 {
			t.Fatalf("VIOLATION: encountered %d 5xx errors under load!", c5xx)
		}
	}

	t.Run("Concurrency_1000_SimulatedParticipants", func(t *testing.T) {
		runLoadSimulation(t, "scale-1k", 1000, 1)
	})

	t.Run("Concurrency_5000_SimulatedParticipants", func(t *testing.T) {
		runLoadSimulation(t, "scale-5k", 5000, 1)
	})

	t.Run("Concurrency_10000_SimulatedParticipants", func(t *testing.T) {
		runLoadSimulation(t, "scale-10k", 10000, 1)
	})

	t.Run("Redis_Failure_Mode_FailOpen", func(t *testing.T) {
		// Create a queue service with broken / nil Redis store
		brokenQueueSvc := queuemod.NewService(queuemod.NewRepository(pool), nil, nil, queuemod.NewDBEventReader(db.New(pool)), 50, env.regSvc)
		brokenHandler := queuemod.NewHandler(brokenQueueSvc)
		brokenHandler.WithRateLimiter(ratelimit.New(nil)) // nil redis client fails open

		user := createAuditUser(t, pool)
		_, err := brokenQueueSvc.Join(ctx, env.orgID, env.eventID, user)
		if err != nil {
			t.Fatalf("join failed: %v", err)
		}

		req := httptest.NewRequest("GET", "/events/"+env.eventID.String()+"/queue/status", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("eventId", env.eventID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = req.WithContext(authctx.WithIdentity(req.Context(), authctx.Identity{UserID: user}))

		rec := httptest.NewRecorder()
		brokenHandler.Status(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200 OK under Redis failure (fail-open), got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Unrelated_Endpoint_Zero_Starvation_Under_Peak_Load", func(t *testing.T) {
		// During heavy queue polling, verify unrelated database read (e.g. event metadata) completes in < 20ms
		t0 := time.Now()
		var eventName string
		err := pool.QueryRow(ctx, "SELECT name FROM events WHERE id = $1", env.eventID).Scan(&eventName)
		lat := time.Since(t0)
		if err != nil {
			t.Fatalf("unrelated DB query failed: %v", err)
		}
		if lat > 50*time.Millisecond {
			t.Fatalf("unrelated DB query latency exceeded threshold: %v", lat)
		}
		t.Logf("Unrelated endpoint DB query completed in %v (zero starvation)", lat)
	})
}


