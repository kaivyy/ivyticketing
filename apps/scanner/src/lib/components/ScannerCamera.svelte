<script lang="ts">
  import QrScanner from 'qr-scanner';

  // Owns the camera MediaStream + qr-scanner decode loop entirely inside a
  // single $effect. The effect's returned cleanup stops the decode loop, stops
  // the camera track, and destroys the scanner — so the camera is released on
  // unmount AND whenever `active` flips (e.g. the parent pauses scanning on a
  // mode change or while a confirmation is shown). Decoded token strings are
  // emitted through the `onToken` callback prop (no DOM manipulation).
  interface Props {
    onToken: (token: string) => void;
    /** When false the camera is torn down; flip on mode change to restart. */
    active?: boolean;
    onError?: (message: string) => void;
  }

  let { onToken, active = true, onError }: Props = $props();

  let videoEl = $state<HTMLVideoElement | undefined>(undefined);
  let permissionError = $state<string | null>(null);

  // Suppress identical back-to-back decodes so a QR held in frame does not fire
  // dozens of callbacks per second. A new/different token always passes through.
  let lastToken = '';
  let lastAt = 0;

  function handleDecode(token: string): void {
    const now = Date.now();
    if (token === lastToken && now - lastAt < 1500) {
      return;
    }
    lastToken = token;
    lastAt = now;
    onToken(token);
  }

  $effect(() => {
    // Reference reactive deps so the effect re-runs when they change.
    const el = videoEl;
    if (!active || !el) {
      return;
    }

    permissionError = null;

    const scanner = new QrScanner(
      el,
      (result: QrScanner.ScanResult) => handleDecode(result.data),
      {
        highlightScanRegion: true,
        highlightCodeOutline: true,
        preferredCamera: 'environment',
        maxScansPerSecond: 8,
      },
    );

    let cancelled = false;
    scanner.start().catch((err: unknown) => {
      if (cancelled) {
        return;
      }
      const name = err instanceof Error ? err.name : '';
      const message =
        name === 'NotAllowedError' || name === 'SecurityError'
          ? 'Akses kamera ditolak. Izinkan kamera untuk memindai.'
          : 'Kamera tidak tersedia.';
      permissionError = message;
      onError?.(message);
    });

    // Mandatory cleanup: stop the decode loop + camera track, then release the
    // scanner. Runs on unmount and before every effect re-run (active change).
    return () => {
      cancelled = true;
      scanner.stop();
      scanner.destroy();
    };
  });
</script>

<div class="camera">
  <!-- svelte-ignore a11y_media_has_caption -->
  <video class="camera__video" bind:this={videoEl} playsinline muted></video>
  {#if permissionError}
    <p class="camera__error" role="alert">{permissionError}</p>
  {/if}
</div>

<style>
  .camera {
    position: relative;
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .camera__video {
    width: 100%;
    max-height: 60vh;
    border-radius: 0.75rem;
    background: #000;
    object-fit: cover;
  }

  .camera__error {
    color: #f87171;
    font-size: 0.875rem;
    margin: 0;
    text-align: center;
  }
</style>
