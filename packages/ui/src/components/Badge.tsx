import * as React from "react";
import { cn } from "../cn";

export type BadgeTone = "neutral" | "success" | "warning" | "danger" | "accent";

const tones: Record<BadgeTone, string> = {
  neutral: "bg-gray-100 text-gray-700",
  success: "bg-success/10 text-success",
  warning: "bg-warning/10 text-warning",
  danger: "bg-danger/10 text-danger",
  accent: "bg-accent/15 text-primary",
};

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone;
}

export function Badge({ tone = "neutral", className, ...props }: BadgeProps) {
  return <span className={cn("inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium", tones[tone], className)} {...props} />;
}
