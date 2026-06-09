import { authedFetch } from "./api";

export interface OrderSummary {
  id: string;
  orderNumber: string;
  eventId: string;
  categoryId: string;
  status: string;
  subtotal: number;
  fee: number;
  discount: number;
  total: number;
  expiredAt: string | null;
  createdAt: string;
}

export interface PaymentResult {
  id: string;
  orderId: string;
  gateway: string;
  method: string;
  channel: string;
  status: string;
  amount: number;
  currency: string;
  merchantReference: string;
  gatewayReference: string;
  payUrl: string;
  qrString: string;
  vaNumber: string;
  expiresAt: string | null;
  paidAt: string | null;
  createdAt: string;
}

export interface CreateOrderParams {
  eventId: string;
  categoryId: string;
  /** admission token — from queue (admissionToken), grant token, or ballot draw admission */
  admissionToken?: string;
}

export interface InitiatePaymentParams {
  gateway: string;
  method: string;
  channel?: string;
}

/**
 * Create an order via POST /events/{eventId}/categories/{categoryId}/checkout.
 * The admission token (queue/grant/ballot) is sent as X-Queue-Token header.
 */
export function createOrder(params: CreateOrderParams): Promise<OrderSummary> {
  const headers: Record<string, string> = {};
  if (params.admissionToken) {
    headers["X-Queue-Token"] = params.admissionToken;
  }
  return authedFetch<OrderSummary>(
    `/events/${params.eventId}/categories/${params.categoryId}/checkout`,
    { method: "POST", headers }
  );
}

/** Fetch an existing order by ID: GET /orders/{orderId} */
export function fetchOrder(orderId: string): Promise<OrderSummary> {
  return authedFetch<OrderSummary>(`/orders/${orderId}`);
}

/**
 * Initiate payment for an order: POST /orders/{orderId}/payments.
 * Uses default gateway/method if not specified — callers should supply these
 * or expose a gateway selector UI.
 */
export function initiatePayment(
  orderId: string,
  params: InitiatePaymentParams = { gateway: "midtrans", method: "SNAP" }
): Promise<PaymentResult> {
  return authedFetch<PaymentResult>(`/orders/${orderId}/payments`, {
    method: "POST",
    body: params,
  });
}
