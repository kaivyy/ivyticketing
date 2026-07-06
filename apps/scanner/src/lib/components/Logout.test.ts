import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import Logout from './Logout.svelte';
import { getSessionToken, setSessionToken } from '../session';

// Example/unit test for Logout.svelte (task 13.4): clicking logout clears the
// session token from Local_Store and notifies the parent via onLoggedOut.
//
// _Requirements: 1.6_

describe('Logout.svelte', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('clears the session token and calls onLoggedOut when clicked', async () => {
    setSessionToken('tok-live');
    expect(getSessionToken()).toBe('tok-live');

    const onLoggedOut = vi.fn();
    render(Logout, { props: { onLoggedOut } });

    await fireEvent.click(screen.getByRole('button', { name: 'Keluar' }));

    expect(getSessionToken()).toBeNull();
    expect(onLoggedOut).toHaveBeenCalledTimes(1);
  });
});
