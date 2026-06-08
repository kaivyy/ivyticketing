import { authedFetch } from "./api";

export interface Ticket {
  id: string;
  ticketNumber: string;
  status: string;
  orderId: string;
  eventId: string;
  categoryId: string;
  holderName: string;
  holderEmail: string;
  eventTitle: string;
  categoryName: string;
  issuedAt: string;
  usedAt?: string;
}

export interface TicketWithQR extends Ticket {
  qrToken: string;
}

export function listMyTickets(): Promise<Ticket[]> {
  return authedFetch<Ticket[]>("/tickets");
}

export function getTicket(id: string): Promise<TicketWithQR> {
  return authedFetch<TicketWithQR>(`/tickets/${id}`);
}

export function getTicketByOrder(orderId: string): Promise<TicketWithQR> {
  return authedFetch<TicketWithQR>(`/orders/${orderId}/ticket`);
}
