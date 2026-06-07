import * as React from "react";
import * as RadixDialog from "@radix-ui/react-dialog";
import { cn } from "../cn";

export interface ModalProps {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  title?: string;
  description?: string;
  children?: React.ReactNode;
  footer?: React.ReactNode;
}

export function Modal({ open, onOpenChange, title, description, children, footer }: ModalProps) {
  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
      <RadixDialog.Portal>
        <RadixDialog.Overlay className="fixed inset-0 bg-black/40" />
        <RadixDialog.Content
          className={cn(
            "fixed left-1/2 top-1/2 w-[90vw] max-w-md -translate-x-1/2 -translate-y-1/2",
            "rounded-lg bg-white p-6 shadow-lg focus:outline-none",
          )}
        >
          {title && <RadixDialog.Title className="text-lg font-semibold text-secondary">{title}</RadixDialog.Title>}
          {description && <RadixDialog.Description className="mt-1 text-sm text-gray-600">{description}</RadixDialog.Description>}
          <div className="mt-4">{children}</div>
          {footer && <div className="mt-6 flex justify-end gap-2">{footer}</div>}
        </RadixDialog.Content>
      </RadixDialog.Portal>
    </RadixDialog.Root>
  );
}
