<script lang="ts">
  import type { Snippet } from 'svelte';

  // Base app-shell layout: a fixed header bar plus a scrollable content area.
  // The flow components (login, event picker, camera, confirm) added in later
  // tasks render into the `children` slot.
  interface Props {
    title?: string;
    header?: Snippet;
    children?: Snippet;
  }

  let { title = 'Ivy Scanner', header, children }: Props = $props();
</script>

<div class="shell">
  <header class="shell__header">
    <span class="shell__title">{title}</span>
    {#if header}
      {@render header()}
    {/if}
  </header>
  <main class="shell__main">
    {#if children}
      {@render children()}
    {/if}
  </main>
</div>

<style>
  .shell {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
    min-height: 100dvh;
  }

  .shell__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    padding-top: calc(0.75rem + env(safe-area-inset-top));
    background: rgba(15, 23, 42, 0.9);
    border-bottom: 1px solid rgba(148, 163, 184, 0.2);
    position: sticky;
    top: 0;
    z-index: 10;
  }

  .shell__title {
    font-weight: 600;
    font-size: 1rem;
  }

  .shell__main {
    flex: 1;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
</style>
