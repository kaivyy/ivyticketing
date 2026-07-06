import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, checkIn, listPermittedEvents, verify } from './api';
import * as session from './session';

// Minimal unit coverage for the api.ts fetch wrapper: header construction
// (Bearer token, Idempotency-Key), URL shape, and error mapping. Full scanner
// behavior is covered once the flow is wired up (tasks 13-14).
//
// The token source is mocked at the module boundary (getSessionToken) so the
// test verifies header construction independently of the Local_Store backend.

function mockFetchOnce(status: number, body: unknown): typeof fetch {
  const fn = vi.fn(async () =>
    new Response(body === undefined ? '' : JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  vi.stubGlobal('fetch', fn);
  return fn as unknown as typeof fetch;
}

function stubToken(token: string | null): void {
  vi.spyOn(session, 'getSessionToken').mockReturnValue(token);
}

describe('api.ts fetch wrapper', () => {
  beforeEach(() => {
    stubToken(null);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('attaches the Bearer token from the session accessor', async () => {
    stubToken('tok-123');
    const fetchMock = mockFetchOnce(200, { events: [] });

    await listPermittedEvents();

    const [url, init] = (fetchMock as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(String(url)).toContain('/api/v1/scan/events');
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer tok-123');
  });

  it('omits Authorization when signed out', async () => {
    const fetchMock = mockFetchOnce(200, { events: [] });

    await listPermittedEvents();

    const [, init] = (fetchMock as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect((init.headers as Record<string, string>).Authorization).toBeUndefined();
  });

  it('sends the Idempotency-Key header on check-in', async () => {
    const fetchMock = mockFetchOnce(200, {
      ticketId: 't1',
      status: 'USED',
      checkedInAt: '2024-01-01T00:00:00Z',
      duplicate: false,
    });

    await checkIn('org1', 'ev1', { ticketId: 't1' }, 'idem-key-1');

    const [url, init] = (fetchMock as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(String(url)).toContain('/organizations/org1/events/ev1/scan/check-in');
    expect((init.headers as Record<string, string>)['Idempotency-Key']).toBe('idem-key-1');
    expect(init.method).toBe('POST');
  });

  it('maps a non-2xx response onto ApiError with the server code', async () => {
    mockFetchOnce(409, { code: 'ALREADY_CHECKED_IN', message: 'already used' });

    await expect(verify('org1', 'ev1', 'token')).rejects.toMatchObject({
      name: 'ApiError',
      status: 409,
      code: 'ALREADY_CHECKED_IN',
    });
  });

  it('exposes ApiError as an Error subclass', async () => {
    mockFetchOnce(422, { code: 'QR_SIGNATURE_INVALID' });
    const err = await verify('o', 'e', 't').catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
  });
});
