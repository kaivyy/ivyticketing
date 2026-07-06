<script lang="ts">
  import { checkIn, pickup, ApiError } from '../api';
  import { enqueueScan } from '../offline-db';
  import type { ScanMode } from './ModeToggle.svelte';

  // Confirms a pickup or check-in for the currently verified ticket. When online
  // it calls the API directly (api.checkIn / api.pickup) with a fresh
  // Idempotency-Key; when offline it enqueues a provisional Scan_Operation
  // (offline-db.enqueueScan). Duplicates — a 200 duplicate check-in, a
  // 409 ALREADY_* pickup, or an offline DUPLICATE outcome — surface a
  // Duplicate_Warning instead of an error (Req 6.1, 6.2, 7.4, 8.4).
  interface Props {
    mode: ScanMode;
    orgId: string;
    eventId: string;
    ticketId: string;
    /** Raw scanned token; required to enqueue an offline operation. */
    qrToken: string;
    counterId?: string;
    slotId?: string;
    /** Connectivity; defaults to the live navigator state. */
    online?: boolean;
    onConfirmed?: () => void;
  }

  let {
    mode,
    orgId,
    eventId,
    ticketId,
    qrToken,
    counterId,
    slotId,
    online = typeof navigator !== 'undefined' ? navigator.onLine : true,
    onConfirmed,
  }: Props = $props();

  type Feedback =
    | { kind: 'success'; message: string }
    | { kind: 'queued'; message: string }
    | { kind: 'duplicate'; message: string }
    | { kind: 'error'; message: string };

  let submitting = $state(false);
  let feedback = $state<Feedback | null>(null);

  const actionLabel = $derived(mode === 'CHECKIN' ? 'Konfirmasi Check-In' : 'Konfirmasi Pengambilan');

  async function confirmOnline(): Promise<Feedback> {
    const idempotencyKey = crypto.randomUUID();
    const scannedAt = new Date().toISOString();

    if (mode === 'CHECKIN') {
      const result = await checkIn(orgId, eventId, { ticketId, scannedAt }, idempotencyKey);
      return result.duplicate
        ? { kind: 'duplicate', message: 'Peserta sudah check-in sebelumnya.' }
        : { kind: 'success', message: 'Check-in berhasil.' };
    }

    try {
      await pickup(
        orgId,
        eventId,
        { ticketId, counterId: counterId ?? '', slotId, scannedAt },
        idempotencyKey,
      );
      return { kind: 'success', message: 'Pengambilan race pack berhasil.' };
    } catch (err) {
      if (err instanceof ApiError && err.code === 'ALREADY_PICKED_UP') {
        return { kind: 'duplicate', message: 'Race pack sudah diambil sebelumnya.' };
      }
      throw err;
    }
  }

  async function confirmOffline(): Promise<Feedback> {
    const outcome = await enqueueScan({
      token: qrToken,
      selectedEventId: eventId,
      kind: mode,
      counterId,
      slotId,
    });

    switch (outcome.status) {
      case 'ENQUEUED':
        return { kind: 'queued', message: 'Tersimpan offline. Akan disinkronkan saat online.' };
      case 'DUPLICATE':
        return { kind: 'duplicate', message: 'Sudah diproses sebelumnya (terdeteksi offline).' };
      case 'REJECTED':
        return { kind: 'error', message: 'Kode QR tidak valid untuk acara ini.' };
    }
  }

  async function handleConfirm(): Promise<void> {
    if (submitting) {
      return;
    }
    submitting = true;
    feedback = null;
    try {
      const next = online ? await confirmOnline() : await confirmOffline();
      feedback = next;
      if (next.kind !== 'error') {
        onConfirmed?.();
      }
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message || 'Gagal memproses.' : 'Tidak dapat memproses.';
      feedback = { kind: 'error', message };
    } finally {
      submitting = false;
    }
  }
</script>

<div class="confirm">
  <button class="confirm__btn" type="button" onclick={handleConfirm} disabled={submitting}>
    {submitting ? 'Memproses…' : actionLabel}
  </button>

  {#if feedback}
    <p class="confirm__feedback confirm__feedback--{feedback.kind}" role="status">
      {feedback.message}
    </p>
  {/if}
</div>

<style>
  .confirm {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    width: 100%;
  }

  .confirm__btn {
    padding: 0.875rem 1rem;
    border-radius: 0.5rem;
    border: none;
    background: #16a34a;
    color: white;
    font-size: 1rem;
    font-weight: 700;
    cursor: pointer;
  }

  .confirm__btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .confirm__feedback {
    margin: 0;
    padding: 0.625rem 0.75rem;
    border-radius: 0.5rem;
    font-size: 0.875rem;
    font-weight: 600;
  }

  .confirm__feedback--success {
    background: rgba(34, 197, 94, 0.15);
    color: #4ade80;
  }

  .confirm__feedback--queued {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
  }

  .confirm__feedback--duplicate {
    background: rgba(234, 179, 8, 0.15);
    color: #facc15;
  }

  .confirm__feedback--error {
    background: rgba(239, 68, 68, 0.15);
    color: #f87171;
  }
</style>
