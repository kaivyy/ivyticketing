import * as React from "react";
import { cn } from "../cn";

export type AlertTone = "info" | "success" | "warning" | "danger";

const tones: Record<AlertTone, string> = {
  info: "border-secondary/20 bg-secondary/5 text-secondary",
  success: "border-success/30 bg-success/10 text-success",
  warning: "border-warning/30 bg-warning/10 text-warning",
  danger: "border-danger/30 bg-danger/10 text-danger",
};

export interface AlertProps extends React.HTMLAttributes<HTMLDivElement> {
  tone?: AlertTone;
  title?: string;
}

export function Alert({ tone = "info", title, className, children, ...props }: AlertProps) {
  return (
    <div role="alert" className={cn("rounded-md border p-4 text-sm", tones[tone], className)} {...props}>
      {title && <p className="mb-1 font-semibold">{title}</p>}
      {children}
    </div>
  );
}
