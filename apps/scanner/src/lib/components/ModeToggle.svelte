<script lang="ts" module>
  // The two scanning modes (Req 5.4). PICKUP = race-pack distribution,
  // CHECKIN = gate check-in (VALID -> USED). Aligns with offline-db ScanKind.
  export type ScanMode = 'PICKUP' | 'CHECKIN';
</script>

<script lang="ts">
  // Toggles between Pickup and Check-In mode and emits the current mode. The
  // parent owns the flow; this component just reflects and reports the choice.
  interface Props {
    mode?: ScanMode;
    onChange?: (mode: ScanMode) => void;
  }

  let { mode = $bindable('PICKUP'), onChange }: Props = $props();

  function setMode(next: ScanMode): void {
    if (mode === next) {
      return;
    }
    mode = next;
    onChange?.(next);
  }
</script>

<div class="toggle" role="group" aria-label="Mode pemindaian">
  <button
    class="toggle__btn"
    class:toggle__btn--active={mode === 'PICKUP'}
    type="button"
    aria-pressed={mode === 'PICKUP'}
    onclick={() => setMode('PICKUP')}
  >
    Pengambilan
  </button>
  <button
    class="toggle__btn"
    class:toggle__btn--active={mode === 'CHECKIN'}
    type="button"
    aria-pressed={mode === 'CHECKIN'}
    onclick={() => setMode('CHECKIN')}
  >
    Check-In
  </button>
</div>

<style>
  .toggle {
    display: inline-flex;
    border-radius: 0.5rem;
    border: 1px solid rgba(148, 163, 184, 0.3);
    overflow: hidden;
    background: rgba(15, 23, 42, 0.6);
  }

  .toggle__btn {
    padding: 0.5rem 1rem;
    border: none;
    background: transparent;
    color: inherit;
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
  }

  .toggle__btn--active {
    background: #2563eb;
    color: white;
  }
</style>
