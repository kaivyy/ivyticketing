<script lang="ts">
  import { syncEngine as defaultEngine, type SyncEngine, type SyncState } from '../sync';

  // Online/offline badge + pending count. Subscribes to the SyncEngine's
  // SyncState store and MUST unsubscribe in the $effect cleanup so no listener
  // leaks when the component unmounts (Req 7.5).
  interface Props {
    engine?: SyncEngine;
  }

  let { engine = defaultEngine }: Props = $props();

  // subscribe() below invokes the listener immediately, so this default is
  // overwritten with the engine's real state as soon as the effect runs.
  let state = $state<SyncState>({ online: true, pending: 0, failed: [] });

  $effect(() => {
    // subscribe() invokes the listener immediately and on every change, and
    // returns an unsubscribe fn — return it directly as the mandatory cleanup.
    const unsubscribe = engine.subscribe((next) => {
      state = next;
    });
    return unsubscribe;
  });

  const failedCount = $derived(state.failed.length);
</script>

<div class="sync" aria-live="polite">
  <span class="sync__badge" class:sync__badge--online={state.online}>
    {state.online ? 'Online' : 'Offline'}
  </span>

  {#if state.pending > 0}
    <span class="sync__pending">{state.pending} menunggu</span>
  {/if}

  {#if failedCount > 0}
    <span class="sync__failed">{failedCount} gagal</span>
  {/if}
</div>

<style>
  .sync {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.75rem;
  }

  .sync__badge {
    padding: 0.25rem 0.5rem;
    border-radius: 999px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    background: rgba(239, 68, 68, 0.2);
    color: #f87171;
  }

  .sync__badge--online {
    background: rgba(34, 197, 94, 0.2);
    color: #4ade80;
  }

  .sync__pending {
    padding: 0.25rem 0.5rem;
    border-radius: 999px;
    background: rgba(59, 130, 246, 0.2);
    color: #60a5fa;
    font-weight: 600;
  }

  .sync__failed {
    padding: 0.25rem 0.5rem;
    border-radius: 999px;
    background: rgba(234, 179, 8, 0.2);
    color: #facc15;
    font-weight: 600;
  }
</style>
