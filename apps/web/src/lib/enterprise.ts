import { authedFetch } from "./api";

// --- types (mirror services/api/internal/modules/enterprise/model.go) ---

export interface APIKey {
  id: string;
  name: string;
  keyPrefix: string;
  scopes: string[];
  rateLimitPerMin: number;
  lastUsedAt?: string | null;
  revokedAt?: string | null;
  createdAt: string;
  // rawKey is present ONLY on the create response — shown once, never again.
  rawKey?: string;
}

export interface CreateAPIKeyBody {
  name: string;
  scopes: string[];
  rateLimitPerMin: number;
}

export interface Webhook {
  id: string;
  url: string;
  events: string[];
  isActive: boolean;
  createdAt: string;
  // secret is present ONLY on the create response — shown once, never again.
  secret?: string;
}

export interface CreateWebhookBody {
  url: string;
  events: string[];
}

export interface WebhookDelivery {
  id: string;
  endpointId: string;
  eventType: string;
  eventKey: string;
  status: string;
  attempts: number;
  lastError?: string;
  nextAttemptAt: string;
  deliveredAt?: string | null;
  createdAt: string;
}

// The scopes an API key can hold (read-only public API surface).
export const API_SCOPES: { value: string; label: string }[] = [
  { value: "events:read", label: "Baca Event" },
  { value: "orders:read", label: "Baca Order" },
  { value: "payments:read", label: "Baca Pembayaran" },
];

// The business events a webhook can subscribe to.
export const WEBHOOK_EVENTS: { value: string; label: string }[] = [
  { value: "order.paid", label: "Order Dibayar" },
  { value: "order.expired", label: "Order Kedaluwarsa" },
  { value: "ticket.issued", label: "Tiket Terbit" },
  { value: "ticket.checked_in", label: "Tiket Check-in" },
];

export const DELIVERY_STATUS_LABELS: Record<string, string> = {
  PENDING: "Menunggu",
  DELIVERED: "Terkirim",
  FAILED: "Gagal",
  DEAD: "Berhenti",
};

// --- organizer endpoints (gated on apikey.manage) ---

const base = (orgId: string) => `/organizations/${orgId}/enterprise`;

export function listAPIKeys(orgId: string): Promise<APIKey[]> {
  return authedFetch<APIKey[]>(`${base(orgId)}/api-keys`);
}

export function createAPIKey(
  orgId: string,
  body: CreateAPIKeyBody
): Promise<APIKey> {
  return authedFetch<APIKey>(`${base(orgId)}/api-keys`, {
    method: "POST",
    body,
  });
}

export function revokeAPIKey(orgId: string, keyId: string): Promise<void> {
  return authedFetch<void>(`${base(orgId)}/api-keys/${keyId}`, {
    method: "DELETE",
  });
}

export function listWebhooks(orgId: string): Promise<Webhook[]> {
  return authedFetch<Webhook[]>(`${base(orgId)}/webhooks`);
}

export function createWebhook(
  orgId: string,
  body: CreateWebhookBody
): Promise<Webhook> {
  return authedFetch<Webhook>(`${base(orgId)}/webhooks`, {
    method: "POST",
    body,
  });
}

export function deleteWebhook(orgId: string, webhookId: string): Promise<void> {
  return authedFetch<void>(`${base(orgId)}/webhooks/${webhookId}`, {
    method: "DELETE",
  });
}

export function listDeliveries(
  orgId: string,
  limit = 50,
  offset = 0
): Promise<WebhookDelivery[]> {
  return authedFetch<WebhookDelivery[]>(
    `${base(orgId)}/webhooks/deliveries?limit=${limit}&offset=${offset}`
  );
}
