import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import ModeToggle from './ModeToggle.svelte';

// Example/unit tests for ModeToggle.svelte (task 13.4): the toggle presents the
// correct scanning mode, switches between PICKUP and CHECKIN, and emits the new
// mode via onChange only when it actually changes.
//
// _Requirements: 5.4_

describe('ModeToggle.svelte', () => {
  it('defaults to PICKUP and marks it as the active mode', () => {
    render(ModeToggle, {});

    const pickup = screen.getByRole('button', { name: 'Pengambilan' });
    const checkin = screen.getByRole('button', { name: 'Check-In' });

    expect(pickup).toHaveAttribute('aria-pressed', 'true');
    expect(checkin).toHaveAttribute('aria-pressed', 'false');
  });

  it('toggles to CHECKIN and emits onChange with the new mode', async () => {
    const onChange = vi.fn();
    render(ModeToggle, { props: { onChange } });

    const checkin = screen.getByRole('button', { name: 'Check-In' });
    await fireEvent.click(checkin);

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith('CHECKIN');
    expect(checkin).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Pengambilan' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('toggles back to PICKUP and emits onChange each way', async () => {
    const onChange = vi.fn();
    render(ModeToggle, { props: { onChange } });

    await fireEvent.click(screen.getByRole('button', { name: 'Check-In' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Pengambilan' }));

    expect(onChange).toHaveBeenCalledTimes(2);
    expect(onChange).toHaveBeenNthCalledWith(1, 'CHECKIN');
    expect(onChange).toHaveBeenNthCalledWith(2, 'PICKUP');
  });

  it('does not re-emit when the already-active mode is clicked', async () => {
    const onChange = vi.fn();
    render(ModeToggle, { props: { mode: 'PICKUP', onChange } });

    await fireEvent.click(screen.getByRole('button', { name: 'Pengambilan' }));

    expect(onChange).not.toHaveBeenCalled();
  });
});
