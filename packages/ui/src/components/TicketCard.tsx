import { Card, CardBody } from "./Card";
import { Badge } from "./Badge";

export interface TicketCardProps {
  eventName: string;
  categoryName: string;
  orderNumber: string;
  status: "pending" | "confirmed" | "cancelled";
}

const statusTone = { pending: "warning", confirmed: "success", cancelled: "neutral" } as const;

export function TicketCard({ eventName, categoryName, orderNumber, status }: TicketCardProps) {
  return (
    <Card>
      <CardBody>
        <div className="flex items-start justify-between">
          <div>
            <p className="font-semibold text-secondary">{eventName}</p>
            <p className="text-sm text-gray-600">{categoryName}</p>
          </div>
          <Badge tone={statusTone[status]}>{status}</Badge>
        </div>
        <p className="mt-3 font-mono text-xs text-gray-500">{orderNumber}</p>
      </CardBody>
    </Card>
  );
}
