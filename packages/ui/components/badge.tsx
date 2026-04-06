import * as React from "react";
import { cn } from "../lib/utils";

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: "default" | "success" | "warning" | "destructive" | "outline";
}

const variants = {
  default: "bg-gray-100 text-gray-800",
  success: "bg-green-100 text-green-800",
  warning: "bg-yellow-100 text-yellow-800",
  destructive: "bg-red-100 text-red-800",
  outline: "border border-gray-300 text-gray-700",
};

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium",
        variants[variant],
        className
      )}
      {...props}
    />
  );
}

export function StatusBadge({ status }: { status: string }) {
  const variant = {
    ACTIVE: "success" as const,
    PENDING: "warning" as const,
    REVOKED: "destructive" as const,
    EXPIRED: "destructive" as const,
    COMPLETED: "success" as const,
    FAILED: "destructive" as const,
    SUSPENDED: "warning" as const,
    DELETED: "destructive" as const,
  }[status] || "default" as const;

  return <Badge variant={variant}>{status}</Badge>;
}
