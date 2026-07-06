import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import Login from './Login.svelte';
import { getSessionToken } from '../session';

// Example/unit tests for Login.svelte UI behavior (task 13.4).
//
// These exercise the REAL login() path (api.ts -> session.setSessionToken) by
// mocking only the network boundary (global fetch). That makes the "stores the
// session token" assertion faithful: on success the component runs the actual
// login(), which persists the accessToken in Local_Store, and we read it back
// via getSessionToken().
//
// _Requirements: 1.1, 1.3_

/** Stub global fetch with a single response for the auth/login endpoint. */
function mockFetchOnce(status: number, body: unknown): ReturnType<typeof vi.fn> {
  const fn = vi.fn(
    async () =>
      new Response(body === undefined ? '' : JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      }),
  );
  vi.stubGlobal('fetch', fn);
  return fn;
}

async function fillCredentials(email: string, password: string): Promise<void> {
  const emailInput = screen.getByLabelText('Email');
  const passwordInput = screen.getByLabelText('Kata sandi');
  await fireEvent.input(emailInput, { target: { value: email } });
  await fireEvent.input(passwordInput, { target: { value: password } });
}

describe('Login.svelte', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('stores the session token and calls onLoggedIn on successful login', async () => {
    mockFetchOnce(200, { accessToken: 'tok-success' });
    const onLoggedIn = vi.fn();

    render(Login, { props: { onLoggedIn } });

    await fillCredentials('staff@example.com', 'correct-horse');
    await fireEvent.click(screen.getByRole('button', { name: 'Masuk' }));

    await waitFor(() => {
      expect(onLoggedIn).toHaveBeenCalledTimes(1);
    });
    // Faithful assertion: the real login() persisted the token in Local_Store.
    expect(getSessionToken()).toBe('tok-success');
    // No auth error surfaced.
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('rejects invalid credentials, shows an auth error, and stores no token', async () => {
    mockFetchOnce(401, { code: 'INVALID_CREDENTIALS', message: 'nope' });
    const onLoggedIn = vi.fn();

    render(Login, { props: { onLoggedIn } });

    await fillCredentials('staff@example.com', 'wrong-password');
    await fireEvent.click(screen.getByRole('button', { name: 'Masuk' }));

    // An authentication error message is displayed.
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Email atau kata sandi salah.');
    });
    // Login is NOT advanced and no token is stored.
    expect(onLoggedIn).not.toHaveBeenCalled();
    expect(getSessionToken()).toBeNull();
  });
});
