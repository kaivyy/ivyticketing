import * as RadixCheckbox from "@radix-ui/react-checkbox";
import { cn } from "../cn";

export interface CheckboxProps {
  checked?: boolean;
  defaultChecked?: boolean;
  onCheckedChange?: (checked: boolean) => void;
  disabled?: boolean;
  id?: string;
}

export function Checkbox({ checked, defaultChecked, onCheckedChange, disabled, id }: CheckboxProps) {
  return (
    <RadixCheckbox.Root
      id={id}
      checked={checked}
      defaultChecked={defaultChecked}
      onCheckedChange={(c) => onCheckedChange?.(c === true)}
      disabled={disabled}
      className={cn(
        "flex h-5 w-5 items-center justify-center rounded border border-gray-300 bg-white",
        "data-[state=checked]:bg-primary data-[state=checked]:border-primary",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
        "disabled:opacity-50",
      )}
    >
      <RadixCheckbox.Indicator className="text-white text-xs">✓</RadixCheckbox.Indicator>
    </RadixCheckbox.Root>
  );
}
