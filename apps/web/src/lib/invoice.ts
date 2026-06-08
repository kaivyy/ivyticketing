import { authedFetch } from "./api";

export interface Order {
  id: string;
  orderNumber: string;
  status: string;
  total: number;
  createdAt: string;
}

export interface Invoice {
  orderId: string;
  orderNumber: string;
  status: string;
  eventTitle: string;
  categoryName: string;
  holderName: string;
  holderEmail: string;
  subtotal: number;
  fee: number;
  discount: number;
  total: number;
  currency: string;
  issuedAt: string;
}

export function listMyOrders(): Promise<Order[]> {
  return authedFetch<Order[]>("/orders");
}

export function getInvoice(orderId: string): Promise<Invoice> {
  return authedFetch<Invoice>(`/orders/${orderId}/invoice`);
}
