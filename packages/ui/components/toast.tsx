"use client";
import * as React from "react";
import { cn } from "../lib/utils";

export interface ToastProps {
  message: string;
  type?: "success" | "error" | "info";
  onClose?: () => void;
}

const typeStyles = {
  success: "bg-green-50 text-green-800 border-green-200",
  error: "bg-red-50 text-red-800 border-red-200",
  info: "bg-blue-50 text-blue-800 border-blue-200",
};

export function Toast({ message, type = "info", onClose }: ToastProps) {
  return (
    <div
      className={cn(
        "fixed bottom-4 right-4 z-50 flex items-center gap-2 rounded-lg border px-4 py-3 shadow-lg",
        typeStyles[type]
      )}
      role="alert"
    >
      <p className="text-sm font-medium">{message}</p>
      {onClose && (
        <button onClick={onClose} className="ml-2 text-current opacity-70 hover:opacity-100">
          ✕
        </button>
      )}
    </div>
  );
}
