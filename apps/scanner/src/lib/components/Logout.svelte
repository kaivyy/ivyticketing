<script lang="ts">
  import { clearSessionToken } from '../session';

  // Logout control: clears the session token from Local_Store (Req 1.6) and
  // notifies the parent so it can return to the Login screen. Kept as its own
  // component so it can live in the app header, decoupled from the login form.
  interface Props {
    onLoggedOut?: () => void;
    label?: string;
  }

  let { onLoggedOut, label = 'Keluar' }: Props = $props();

  function handleLogout(): void {
    clearSessionToken();
    onLoggedOut?.();
  }
</script>

<button class="logout" type="button" onclick={handleLogout}>{label}</button>

<style>
  .logout {
    padding: 0.375rem 0.75rem;
    border-radius: 0.375rem;
    border: 1px solid rgba(148, 163, 184, 0.35);
    background: transparent;
    color: inherit;
    font-size: 0.8125rem;
    cursor: pointer;
  }

  .logout:hover {
    background: rgba(148, 163, 184, 0.15);
  }
</style>
