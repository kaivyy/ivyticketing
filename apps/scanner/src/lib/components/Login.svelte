<script lang="ts">
  import { login } from '../api';
  import { ApiError } from '../api';
  import { clearSessionToken } from '../session';

  // Credentials form. On submit it authenticates against the platform auth
  // endpoint (api.login), which persists the bearer token in Local_Store
  // (session.ts). On invalid credentials it surfaces an auth error. `onLoggedIn`
  // lets the parent advance the flow once a token is stored.
  interface Props {
    onLoggedIn?: () => void;
  }

  let { onLoggedIn }: Props = $props();

  let email = $state('');
  let password = $state('');
  let submitting = $state(false);
  let error = $state<string | null>(null);

  const canSubmit = $derived(email.trim().length > 0 && password.length > 0 && !submitting);

  async function handleSubmit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }
    submitting = true;
    error = null;
    try {
      await login({ email: email.trim(), password });
      password = '';
      onLoggedIn?.();
    } catch (err) {
      // Any non-2xx (typically 401) means the credentials were rejected.
      if (err instanceof ApiError && err.status === 401) {
        error = 'Email atau kata sandi salah.';
      } else if (err instanceof ApiError) {
        error = err.message || 'Gagal masuk. Coba lagi.';
      } else {
        error = 'Tidak dapat terhubung ke server.';
      }
      // Ensure no stale token lingers after a failed attempt.
      clearSessionToken();
    } finally {
      submitting = false;
    }
  }
</script>

<form class="login" onsubmit={handleSubmit}>
  <h1 class="login__title">Masuk</h1>

  <label class="login__field">
    <span>Email</span>
    <input
      type="email"
      autocomplete="username"
      bind:value={email}
      required
      disabled={submitting}
    />
  </label>

  <label class="login__field">
    <span>Kata sandi</span>
    <input
      type="password"
      autocomplete="current-password"
      bind:value={password}
      required
      disabled={submitting}
    />
  </label>

  {#if error}
    <p class="login__error" role="alert">{error}</p>
  {/if}

  <button class="login__submit" type="submit" disabled={!canSubmit}>
    {submitting ? 'Memproses…' : 'Masuk'}
  </button>
</form>

<style>
  .login {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    max-width: 22rem;
    width: 100%;
    margin: auto;
  }

  .login__title {
    font-size: 1.25rem;
    margin: 0 0 0.5rem;
  }

  .login__field {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.875rem;
  }

  .login__field input {
    padding: 0.625rem 0.75rem;
    border-radius: 0.5rem;
    border: 1px solid rgba(148, 163, 184, 0.35);
    background: rgba(15, 23, 42, 0.6);
    color: inherit;
    font-size: 1rem;
  }

  .login__error {
    color: #f87171;
    font-size: 0.875rem;
    margin: 0;
  }

  .login__submit {
    margin-top: 0.5rem;
    padding: 0.75rem 1rem;
    border-radius: 0.5rem;
    border: none;
    background: #2563eb;
    color: white;
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
  }

  .login__submit:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
