<script lang="ts">
  import Layout from './lib/components/Layout.svelte';
  import Login from './lib/components/Login.svelte';
  import Logout from './lib/components/Logout.svelte';
  import EventPicker, { type EventSelection } from './lib/components/EventPicker.svelte';
  import ModeToggle, { type ScanMode } from './lib/components/ModeToggle.svelte';
  import ScannerCamera from './lib/components/ScannerCamera.svelte';
  import ParticipantCard from './lib/components/ParticipantCard.svelte';
  import ConfirmAction from './lib/components/ConfirmAction.svelte';
  import OfflineSyncStatus from './lib/components/OfflineSyncStatus.svelte';

  import { api, ApiError, type PermittedEvent, type VerifyResult } from './lib/api';
  import { getSessionToken } from './lib/session';
  import { validateStructure, restoreQueue, type ScanOperation } from './lib/offline-db';
  import { SyncEngine, createApiTransport } from './lib/sync';

  // -------------------------------------------------------------------------
  // Full client flow (task 14.1):
  //   Login -> EventPicker -> (ModeToggle) -> ScannerCamera
  //     -> verify (online) OR structural-validate + enqueue (offline)
  //     -> ParticipantCard -> ConfirmAction -> sync
  // -------------------------------------------------------------------------

  type Screen = 'login' | 'events' | 'scanning';

  let screen = $state<Screen>(getSessionToken() ? 'events' : 'login');
  let selection = $state<EventSelection | null>(null);
  let mode = $state<ScanMode>('PICKUP');

  // Online-verify result (full display info) vs the minimal offline fallback
  // (structural validation only yields a ticketId — no server display info).
  let verified = $state<VerifyResult | null>(null);
  let offlineTicket = $state<{ ticketId: string } | null>(null);
  let currentToken = $state('');
  let verifying = $state(false);
  let scanError = $state<string | null>(null);

  // Connectivity is driven by the SyncEngine's own state (navigator.onLine +
  // online/offline listeners), so the verify-vs-offline branch and the
  // ConfirmAction stay in sync with the engine that drains the queue.
  let online = $state(typeof navigator !== 'undefined' ? navigator.onLine : true);

  // eventId -> organizationId map so the sync transport can resolve the org for
  // any queued op (built from the permitted-events list; falls back to the
  // currently-selected org). The scan endpoints live under
  // /organizations/{orgId}/events/{eventId}.
  const orgByEvent = new Map<string, string>();

  const transport = createApiTransport({
    checkIn: (orgId, eventId, body, key) => api.checkIn(orgId, eventId, body, key),
    pickup: (orgId, eventId, body, key) => api.pickup(orgId, eventId, body, key),
    resolveOrgId: (op: ScanOperation) =>
      orgByEvent.get(op.eventId) ?? selection?.organizationId ?? '',
  });

  // A dedicated engine wired with the real API transport so drain-on-reconnect
  // works; OfflineSyncStatus subscribes to this same instance.
  const engine = new SyncEngine(transport);

  const eventName = $derived(selection?.name ?? '');

  // A confirmation card is showing (online verify result or offline fallback);
  // pause the camera while confirming so we don't stack scans.
  const showingResult = $derived(verified !== null || offlineTicket !== null);
  const cameraActive = $derived(screen === 'scanning' && !showingResult && !verifying);

  const confirmTicketId = $derived(verified?.ticketId ?? offlineTicket?.ticketId ?? '');

  let resetTimer: ReturnType<typeof setTimeout> | undefined;

  // Bootstrap the sync engine + connectivity once. Restores any persisted queue,
  // pre-loads the org map, and tears everything down on unmount.
  $effect(() => {
    engine.start();
    const unsubscribe = engine.subscribe((state) => {
      online = state.online;
    });

    // Restore the persisted queue and reflect it in the status badge.
    void restoreQueue()
      .then(() => engine.refresh())
      .catch(() => {
        /* best-effort: offline/empty queue is fine */
      });

    // Pre-populate the eventId -> orgId map from the permitted events so queued
    // ops from a prior session can resolve their org even before a selection.
    if (getSessionToken()) {
      void loadOrgMap();
    }

    return () => {
      engine.stop();
      unsubscribe();
      if (resetTimer) {
        clearTimeout(resetTimer);
      }
    };
  });

  async function loadOrgMap(): Promise<void> {
    try {
      const { events } = await api.listPermittedEvents();
      for (const e of events as PermittedEvent[]) {
        orgByEvent.set(e.eventId, e.organizationId);
      }
    } catch {
      /* best-effort; the current selection covers the active session */
    }
  }

  function handleLoggedIn(): void {
    screen = 'events';
    void loadOrgMap();
  }

  function handleLoggedOut(): void {
    resetScanState();
    selection = null;
    screen = 'login';
  }

  function handleEventSelected(sel: EventSelection): void {
    selection = sel;
    orgByEvent.set(sel.eventId, sel.organizationId);
    resetScanState();
    screen = 'scanning';
  }

  function handleModeChange(): void {
    // Switching modes discards any in-progress result and restarts the camera.
    resetScanState();
  }

  function resetScanState(): void {
    if (resetTimer) {
      clearTimeout(resetTimer);
      resetTimer = undefined;
    }
    verified = null;
    offlineTicket = null;
    currentToken = '';
    verifying = false;
    scanError = null;
  }

  async function handleToken(token: string): Promise<void> {
    if (!selection || showingResult || verifying) {
      return;
    }
    scanError = null;
    currentToken = token;

    if (online) {
      verifying = true;
      try {
        verified = await api.verify(selection.organizationId, selection.eventId, token);
      } catch (err) {
        verified = null;
        currentToken = '';
        scanError =
          err instanceof ApiError
            ? err.message || 'Kode QR ditolak.'
            : 'Tidak dapat memverifikasi. Coba lagi.';
      } finally {
        verifying = false;
      }
      return;
    }

    // Offline: structural validation only (no HMAC secret on the client). A
    // valid structure yields a ticketId for a minimal card; ConfirmAction then
    // enqueues the provisional operation.
    const structural = validateStructure(token, selection.eventId);
    if (structural.ok) {
      offlineTicket = { ticketId: structural.ticketId };
    } else {
      currentToken = '';
      scanError = 'Kode QR tidak valid untuk acara ini.';
    }
  }

  function handleConfirmed(): void {
    // Reflect any newly-enqueued offline op in the badge, then return to the
    // camera for the next participant after the success feedback is visible.
    void engine.refresh();
    resetTimer = setTimeout(() => {
      resetScanState();
    }, 1500);
  }
</script>

<Layout title="Ivy Scanner">
  {#snippet header()}
    {#if screen === 'scanning'}
      <div class="topbar">
        <OfflineSyncStatus {engine} />
        <Logout onLoggedOut={handleLoggedOut} />
      </div>
    {/if}
  {/snippet}

  {#if screen === 'login'}
    <Login onLoggedIn={handleLoggedIn} />
  {:else if screen === 'events'}
    <EventPicker onSelect={handleEventSelected} selectedEventId={selection?.eventId ?? null} />
  {:else if screen === 'scanning' && selection}
    <section class="scan">
      <div class="scan__bar">
        <span class="scan__event">{eventName}</span>
        <ModeToggle bind:mode onChange={handleModeChange} />
      </div>

      {#if showingResult}
        {#if verified}
          <ParticipantCard result={verified} />
        {:else if offlineTicket}
          <article class="offline-card">
            <h2 class="offline-card__title">Tiket terpindai (offline)</h2>
            <p class="offline-card__id">ID: {offlineTicket.ticketId}</p>
            <p class="offline-card__note">
              Info peserta tidak tersedia saat offline. Operasi akan disinkronkan
              dan diverifikasi saat kembali online.
            </p>
          </article>
        {/if}

        <ConfirmAction
          {mode}
          orgId={selection.organizationId}
          eventId={selection.eventId}
          ticketId={confirmTicketId}
          qrToken={currentToken}
          {online}
          onConfirmed={handleConfirmed}
        />
      {:else}
        <ScannerCamera onToken={handleToken} active={cameraActive} />
        {#if verifying}
          <p class="scan__hint" role="status">Memverifikasi…</p>
        {/if}
        {#if scanError}
          <p class="scan__error" role="alert">{scanError}</p>
        {/if}
      {/if}
    </section>
  {/if}
</Layout>

<style>
  .topbar {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .scan {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    width: 100%;
    max-width: 30rem;
    margin: 0 auto;
  }

  .scan__bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .scan__event {
    font-weight: 600;
    font-size: 1rem;
  }

  .scan__hint {
    margin: 0;
    text-align: center;
    opacity: 0.7;
    font-size: 0.875rem;
  }

  .scan__error {
    margin: 0;
    text-align: center;
    color: #f87171;
    font-size: 0.875rem;
  }

  .offline-card {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 1rem;
    border-radius: 0.75rem;
    border: 1px solid rgba(148, 163, 184, 0.3);
    background: rgba(15, 23, 42, 0.6);
    width: 100%;
  }

  .offline-card__title {
    font-size: 1.125rem;
    margin: 0;
  }

  .offline-card__id {
    margin: 0;
    font-size: 0.8125rem;
    opacity: 0.7;
    word-break: break-all;
  }

  .offline-card__note {
    margin: 0;
    font-size: 0.875rem;
    opacity: 0.8;
  }
</style>
