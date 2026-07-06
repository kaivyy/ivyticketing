<script lang="ts">
  import { listPermittedEvents, ApiError, type PermittedEvent } from '../api';

  // Loads the caller's Permitted_Events (api.listPermittedEvents) and lets staff
  // pick one to scan against. Emits the full selection { organizationId, eventId }
  // via `onSelect` so downstream verify/check-in/pickup calls have the orgId they
  // need (the scan endpoints are mounted under /organizations/{orgId}/events/{eventId}).
  export interface EventSelection {
    organizationId: string;
    eventId: string;
    name: string;
  }

  interface Props {
    onSelect?: (selection: EventSelection) => void;
    selectedEventId?: string | null;
  }

  let { onSelect, selectedEventId = null }: Props = $props();

  let events = $state<PermittedEvent[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Load the permitted events once, aborting the request if the component is
  // torn down before it resolves.
  $effect(() => {
    const controller = new AbortController();
    loading = true;
    error = null;

    listPermittedEvents(controller.signal)
      .then((result) => {
        events = result.events;
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        if (err instanceof ApiError) {
          error = err.message || 'Gagal memuat daftar acara.';
        } else {
          error = 'Tidak dapat memuat daftar acara.';
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          loading = false;
        }
      });

    return () => controller.abort();
  });

  function select(event: PermittedEvent): void {
    onSelect?.({
      organizationId: event.organizationId,
      eventId: event.eventId,
      name: event.name,
    });
  }
</script>

<section class="picker">
  <h2 class="picker__title">Pilih acara</h2>

  {#if loading}
    <p class="picker__hint">Memuat…</p>
  {:else if error}
    <p class="picker__error" role="alert">{error}</p>
  {:else if events.length === 0}
    <p class="picker__hint">Tidak ada acara yang diizinkan untuk akun ini.</p>
  {:else}
    <ul class="picker__list">
      {#each events as event (event.eventId)}
        <li>
          <button
            class="picker__item"
            class:picker__item--active={event.eventId === selectedEventId}
            type="button"
            onclick={() => select(event)}
          >
            <span class="picker__name">{event.name}</span>
            <span class="picker__status">{event.status}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .picker {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    width: 100%;
  }

  .picker__title {
    font-size: 1.125rem;
    margin: 0;
  }

  .picker__hint {
    opacity: 0.7;
    font-size: 0.875rem;
    margin: 0;
  }

  .picker__error {
    color: #f87171;
    font-size: 0.875rem;
    margin: 0;
  }

  .picker__list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .picker__item {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    border-radius: 0.5rem;
    border: 1px solid rgba(148, 163, 184, 0.3);
    background: rgba(15, 23, 42, 0.6);
    color: inherit;
    font-size: 0.9375rem;
    text-align: left;
    cursor: pointer;
  }

  .picker__item--active {
    border-color: #2563eb;
    background: rgba(37, 99, 235, 0.15);
  }

  .picker__name {
    font-weight: 600;
  }

  .picker__status {
    font-size: 0.75rem;
    opacity: 0.7;
    text-transform: uppercase;
  }
</style>
