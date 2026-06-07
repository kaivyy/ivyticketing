import { Modal } from "./Modal";
import { Button } from "./Button";

export interface DialogProps {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  onConfirm?: () => void;
  destructive?: boolean;
}

export function Dialog({ open, onOpenChange, title, description, confirmLabel = "Confirm", cancelLabel = "Cancel", onConfirm, destructive }: DialogProps) {
  return (
    <Modal
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange?.(false)}>{cancelLabel}</Button>
          <Button variant={destructive ? "danger" : "primary"} onClick={onConfirm}>{confirmLabel}</Button>
        </>
      }
    />
  );
}
