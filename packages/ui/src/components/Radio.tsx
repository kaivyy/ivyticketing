import * as RadixRadio from "@radix-ui/react-radio-group";
import { cn } from "../cn";

export interface RadioOption {
  value: string;
  label: string;
}

export interface RadioGroupProps {
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  options: RadioOption[];
  name?: string;
  disabled?: boolean;
}

export function RadioGroup({ value, defaultValue, onValueChange, options, name, disabled }: RadioGroupProps) {
  return (
    <RadixRadio.Root
      value={value}
      defaultValue={defaultValue}
      onValueChange={onValueChange}
      name={name}
      disabled={disabled}
      className="flex flex-col gap-2"
    >
      {options.map((opt) => (
        <label key={opt.value} className="flex items-center gap-2 text-sm">
          <RadixRadio.Item
            value={opt.value}
            className={cn(
              "h-5 w-5 rounded-full border border-gray-300 bg-white",
              "data-[state=checked]:border-primary",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
            )}
          >
            <RadixRadio.Indicator className="flex h-full w-full items-center justify-center after:block after:h-2.5 after:w-2.5 after:rounded-full after:bg-primary" />
          </RadixRadio.Item>
          {opt.label}
        </label>
      ))}
    </RadixRadio.Root>
  );
}
