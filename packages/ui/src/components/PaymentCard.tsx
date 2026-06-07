import * as React from "react";
import { Card, CardBody, CardFooter } from "./Card";
import { Badge } from "./Badge";

export interface PaymentCardProps {
  amount: string;
  method?: string;
  status: "pending" | "paid" | "expired" | "failed";
  footer?: React.ReactNode;
}

const statusTone = { pending: "warning", paid: "success", expired: "neutral", failed: "danger" } as const;

export function PaymentCard({ amount, method, status, footer }: PaymentCardProps) {
  return (
    <Card>
      <CardBody className="flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-600">Amount due</p>
          <p className="text-2xl font-semibold text-secondary">{amount}</p>
          {method && <p className="mt-1 text-xs text-gray-500">{method}</p>}
        </div>
        <Badge tone={statusTone[status]}>{status}</Badge>
      </CardBody>
      {footer && <CardFooter>{footer}</CardFooter>}
    </Card>
  );
}
