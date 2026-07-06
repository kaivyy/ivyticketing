<script lang="ts">
  import type { VerifyResult } from '../api';

  // Presents the non-sensitive participant fields from a VerifyResult (name,
  // BIB, category, ticket status) plus duplicate warnings with their original
  // timestamps (already-picked-up / already-checked-in, Req 3.1-3.3, 6.1, 6.4).
  interface Props {
    result: VerifyResult;
  }

  let { result }: Props = $props();

  const display = $derived(result.display);
  const bib = $derived(display.bibNumber || '—');

  function formatTimestamp(iso?: string): string {
    if (!iso) {
      return '';
    }
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) {
      return iso;
    }
    return date.toLocaleString();
  }

  const pickedUpAt = $derived(formatTimestamp(result.pickedUpAt));
  const checkedInAt = $derived(formatTimestamp(result.checkedInAt));
</script>

<article class="card">
  <header class="card__header">
    <h2 class="card__name">{display.participantName}</h2>
    <span class="card__status card__status--{display.ticketStatus.toLowerCase()}">
      {display.ticketStatus}
    </span>
  </header>

  <dl class="card__fields">
    <div class="card__field">
      <dt>BIB</dt>
      <dd>{bib}</dd>
    </div>
    <div class="card__field">
      <dt>Kategori</dt>
      <dd>{display.category || '—'}</dd>
    </div>
  </dl>

  {#if result.alreadyPickedUp}
    <p class="card__warning" role="status">
      Sudah mengambil race pack{pickedUpAt ? ` • ${pickedUpAt}` : ''}
    </p>
  {/if}

  {#if result.alreadyCheckedIn}
    <p class="card__warning" role="status">
      Sudah check-in{checkedInAt ? ` • ${checkedInAt}` : ''}
    </p>
  {/if}
</article>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    padding: 1rem;
    border-radius: 0.75rem;
    border: 1px solid rgba(148, 163, 184, 0.3);
    background: rgba(15, 23, 42, 0.6);
    width: 100%;
  }

  .card__header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .card__name {
    font-size: 1.25rem;
    margin: 0;
  }

  .card__status {
    font-size: 0.6875rem;
    font-weight: 700;
    padding: 0.25rem 0.5rem;
    border-radius: 999px;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    background: rgba(148, 163, 184, 0.25);
  }

  .card__status--valid {
    background: rgba(34, 197, 94, 0.2);
    color: #4ade80;
  }

  .card__status--used {
    background: rgba(234, 179, 8, 0.2);
    color: #facc15;
  }

  .card__status--cancelled {
    background: rgba(239, 68, 68, 0.2);
    color: #f87171;
  }

  .card__fields {
    display: flex;
    gap: 2rem;
    margin: 0;
  }

  .card__field {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
  }

  .card__field dt {
    font-size: 0.6875rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    opacity: 0.6;
  }

  .card__field dd {
    margin: 0;
    font-size: 1.125rem;
    font-weight: 600;
  }

  .card__warning {
    margin: 0;
    padding: 0.5rem 0.75rem;
    border-radius: 0.5rem;
    background: rgba(234, 179, 8, 0.15);
    color: #facc15;
    font-size: 0.875rem;
    font-weight: 600;
  }
</style>
