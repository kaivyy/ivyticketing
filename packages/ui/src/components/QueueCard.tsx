import { Card, CardBody } from "./Card";
import { Badge } from "./Badge";

export interface QueueCardProps {
  position: number;
  estimatedWait?: string;
  status: "waiting" | "allowed" | "expired";
}

const statusTone = { waiting: "warning", allowed: "success", expired: "danger" } as const;

export function QueueCard({ position, estimatedWait, status }: QueueCardProps) {
  return (
    <Card>
      <CardBody className="flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-600">Your position</p>
          <p className="text-2xl font-semibold text-primary">#{position}</p>
          {estimatedWait && <p className="mt-1 text-xs text-gray-500">~{estimatedWait}</p>}
        </div>
        <Badge tone={statusTone[status]}>{status}</Badge>
      </CardBody>
    </Card>
  );
}
