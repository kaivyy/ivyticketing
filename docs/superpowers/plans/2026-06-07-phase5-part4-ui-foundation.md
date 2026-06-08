# Phase 5 Plan — Part 4: UI Foundation (Tasks 10-11)

> Part of the Phase 5 implementation plan. Index: [2026-06-07-phase5-orders-inventory-checkout.md](2026-06-07-phase5-orders-inventory-checkout.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **INDEPENDENT of backend parts** — may run anytime. Do NOT build final pages/landing/dashboard. This is a presentational design-system foundation only.

---

## Task 10: packages/ui scaffold, theme, and primitive components

**Files:**
- Create: `packages/ui/package.json`
- Create: `packages/ui/tsconfig.json`
- Create: `packages/ui/tailwind.config.cjs`
- Create: `packages/ui/src/theme.css`
- Create: `packages/ui/src/cn.ts`
- Create: `packages/ui/src/components/Button.tsx`
- Create: `packages/ui/src/components/Input.tsx`
- Create: `packages/ui/src/components/Textarea.tsx`
- Create: `packages/ui/src/components/Select.tsx`
- Create: `packages/ui/src/components/Checkbox.tsx`
- Create: `packages/ui/src/components/Radio.tsx`
- Create: `packages/ui/src/components/Badge.tsx`
- Create: `packages/ui/src/components/Alert.tsx`
- Create: `packages/ui/src/components/Card.tsx`

These are React+TS presentational components styled with Tailwind, using Radix primitives
where accessibility matters (Select, Checkbox, Radio in Task 11's Modal/Dialog). They are
framework-agnostic enough to be consumed by Astro islands later. No data fetching, no API.

- [ ] **Step 1: package.json**

Create `packages/ui/package.json`:
```json
{
  "name": "@ivyticketing/ui",
  "version": "0.1.0",
  "type": "module",
  "main": "src/index.ts",
  "types": "src/index.ts",
  "scripts": {
    "typecheck": "tsc --noEmit",
    "build": "tsc --noEmit"
  },
  "peerDependencies": {
    "react": "^18.0.0",
    "react-dom": "^18.0.0"
  },
  "dependencies": {
    "@radix-ui/react-dialog": "^1.1.1",
    "@radix-ui/react-select": "^2.1.1",
    "@radix-ui/react-checkbox": "^1.1.1",
    "@radix-ui/react-radio-group": "^1.2.0",
    "clsx": "^2.1.1",
    "tailwind-merge": "^2.5.2"
  },
  "devDependencies": {
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "tailwindcss": "^3.4.0",
    "typescript": "^5.5.0"
  }
}
```

- [ ] **Step 2: tsconfig.json**

Create `packages/ui/tsconfig.json`:
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "noEmit": true
  },
  "include": ["src"]
}
```

- [ ] **Step 3: Tailwind config + theme tokens**

Create `packages/ui/tailwind.config.cjs`:
```js
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        primary: "#0B3D2E",
        secondary: "#111827",
        accent: "#D6A84F",
        background: "#F8F7F2",
        success: "#16A34A",
        warning: "#F59E0B",
        danger: "#DC2626",
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
      },
    },
  },
  plugins: [],
};
```

Create `packages/ui/src/theme.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --color-primary: #0B3D2E;
  --color-secondary: #111827;
  --color-accent: #D6A84F;
  --color-background: #F8F7F2;
  --color-success: #16A34A;
  --color-warning: #F59E0B;
  --color-danger: #DC2626;
  --font-sans: "Inter", system-ui, sans-serif;
}

body {
  font-family: var(--font-sans);
  background-color: var(--color-background);
  color: var(--color-secondary);
}
```

- [ ] **Step 4: cn helper**

Create `packages/ui/src/cn.ts`:
```ts
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/** cn merges class names with Tailwind conflict resolution. */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
```

- [ ] **Step 5: Button**

Create `packages/ui/src/components/Button.tsx`:
```tsx
import * as React from "react";
import { cn } from "../cn";

export type ButtonVariant = "primary" | "secondary" | "accent" | "danger" | "ghost";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
}

const variants: Record<ButtonVariant, string> = {
  primary: "bg-primary text-white hover:bg-primary/90",
  secondary: "bg-secondary text-white hover:bg-secondary/90",
  accent: "bg-accent text-secondary hover:bg-accent/90",
  danger: "bg-danger text-white hover:bg-danger/90",
  ghost: "bg-transparent text-secondary hover:bg-black/5",
};

const sizes: Record<ButtonSize, string> = {
  sm: "h-8 px-3 text-sm",
  md: "h-10 px-4 text-sm",
  lg: "h-12 px-6 text-base",
};

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "primary", size = "md", loading, disabled, className, children, ...props }, ref) => (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={cn(
        "inline-flex items-center justify-center rounded-md font-medium transition-colors",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
        "disabled:pointer-events-none disabled:opacity-50",
        variants[variant],
        sizes[size],
        className,
      )}
      {...props}
    >
      {loading ? "…" : children}
    </button>
  ),
);
Button.displayName = "Button";
```

- [ ] **Step 6: Input + Textarea**

Create `packages/ui/src/components/Input.tsx`:
```tsx
import * as React from "react";
import { cn } from "../cn";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ invalid, className, ...props }, ref) => (
    <input
      ref={ref}
      className={cn(
        "h-10 w-full rounded-md border bg-white px-3 text-sm",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
        "disabled:cursor-not-allowed disabled:opacity-50",
        invalid ? "border-danger" : "border-gray-300",
        className,
      )}
      {...props}
    />
  ),
);
Input.displayName = "Input";
```

Create `packages/ui/src/components/Textarea.tsx`:
```tsx
import * as React from "react";
import { cn } from "../cn";

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ invalid, className, ...props }, ref) => (
    <textarea
      ref={ref}
      className={cn(
        "min-h-[80px] w-full rounded-md border bg-white px-3 py-2 text-sm",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
        "disabled:cursor-not-allowed disabled:opacity-50",
        invalid ? "border-danger" : "border-gray-300",
        className,
      )}
      {...props}
    />
  ),
);
Textarea.displayName = "Textarea";
```

- [ ] **Step 7: Select (Radix), Checkbox (Radix), Radio (Radix)**

Create `packages/ui/src/components/Select.tsx`:
```tsx
import * as React from "react";
import * as RadixSelect from "@radix-ui/react-select";
import { cn } from "../cn";

export interface SelectOption {
  value: string;
  label: string;
}

export interface SelectProps {
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  options: SelectOption[];
  placeholder?: string;
  disabled?: boolean;
}

export function Select({ value, defaultValue, onValueChange, options, placeholder = "Select…", disabled }: SelectProps) {
  return (
    <RadixSelect.Root value={value} defaultValue={defaultValue} onValueChange={onValueChange} disabled={disabled}>
      <RadixSelect.Trigger
        className={cn(
          "inline-flex h-10 w-full items-center justify-between rounded-md border border-gray-300 bg-white px-3 text-sm",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent",
          "disabled:cursor-not-allowed disabled:opacity-50",
        )}
      >
        <RadixSelect.Value placeholder={placeholder} />
        <RadixSelect.Icon>▾</RadixSelect.Icon>
      </RadixSelect.Trigger>
      <RadixSelect.Portal>
        <RadixSelect.Content className="overflow-hidden rounded-md border border-gray-200 bg-white shadow-md">
          <RadixSelect.Viewport className="p-1">
            {options.map((opt) => (
              <RadixSelect.Item
                key={opt.value}
                value={opt.value}
                className="relative flex h-9 cursor-pointer select-none items-center rounded px-6 text-sm data-[highlighted]:bg-primary/10 data-[highlighted]:outline-none"
              >
                <RadixSelect.ItemText>{opt.label}</RadixSelect.ItemText>
              </RadixSelect.Item>
            ))}
          </RadixSelect.Viewport>
        </RadixSelect.Content>
      </RadixSelect.Portal>
    </RadixSelect.Root>
  );
}
```

Create `packages/ui/src/components/Checkbox.tsx`:
```tsx
import * as React from "react";
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
```

Create `packages/ui/src/components/Radio.tsx`:
```tsx
import * as React from "react";
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
```

- [ ] **Step 8: Badge, Alert, Card**

Create `packages/ui/src/components/Badge.tsx`:
```tsx
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
```

Create `packages/ui/src/components/Alert.tsx`:
```tsx
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
```

Create `packages/ui/src/components/Card.tsx`:
```tsx
import * as React from "react";
import { cn } from "../cn";

export function Card({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("rounded-lg border border-gray-200 bg-white shadow-sm", className)} {...props} />;
}
export function CardHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("border-b border-gray-100 p-4", className)} {...props} />;
}
export function CardBody({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("p-4", className)} {...props} />;
}
export function CardFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("border-t border-gray-100 p-4", className)} {...props} />;
}
```

- [ ] **Step 9: Install deps and typecheck**

Run:
```bash
cd packages/ui && pnpm install && pnpm typecheck; cd ../..
```
Expected: install succeeds; `tsc --noEmit` passes (no type errors). If pnpm workspace isn't set up, add `packages/*` to `pnpm-workspace.yaml` at repo root (create if absent):
```yaml
packages:
  - "apps/*"
  - "packages/*"
  - "services/*"
```
Note: only create/modify `pnpm-workspace.yaml` if it doesn't already list these — additive, don't remove existing entries.

- [ ] **Step 10: Commit**

```bash
git add packages/ui pnpm-workspace.yaml
git commit -m "feat(ui): add design-system scaffold, theme, and primitive components"
```

---

## Task 11: Modal/Dialog, Table, state components, domain shells, barrel + README

**Files:**
- Create: `packages/ui/src/components/Modal.tsx`
- Create: `packages/ui/src/components/Dialog.tsx`
- Create: `packages/ui/src/components/Table.tsx`
- Create: `packages/ui/src/components/EmptyState.tsx`
- Create: `packages/ui/src/components/LoadingState.tsx`
- Create: `packages/ui/src/components/ErrorState.tsx`
- Create: `packages/ui/src/components/QueueCard.tsx`
- Create: `packages/ui/src/components/PaymentCard.tsx`
- Create: `packages/ui/src/components/TicketCard.tsx`
- Create: `packages/ui/src/index.ts`
- Create: `packages/ui/README.md`

- [ ] **Step 1: Modal + Dialog (Radix Dialog)**

Create `packages/ui/src/components/Modal.tsx`:
```tsx
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
```

Create `packages/ui/src/components/Dialog.tsx` (a confirm-style dialog built on Modal):
```tsx
import * as React from "react";
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
```

- [ ] **Step 2: Table**

Create `packages/ui/src/components/Table.tsx`:
```tsx
import * as React from "react";
import { cn } from "../cn";

export function Table({ className, ...props }: React.TableHTMLAttributes<HTMLTableElement>) {
  return <table className={cn("w-full border-collapse text-sm", className)} {...props} />;
}
export function THead(props: React.HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className="border-b border-gray-200 text-left text-gray-600" {...props} />;
}
export function TBody(props: React.HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody {...props} />;
}
export function TR({ className, ...props }: React.HTMLAttributes<HTMLTableRowElement>) {
  return <tr className={cn("border-b border-gray-100", className)} {...props} />;
}
export function TH({ className, ...props }: React.ThHTMLAttributes<HTMLTableCellElement>) {
  return <th className={cn("px-3 py-2 font-medium", className)} {...props} />;
}
export function TD({ className, ...props }: React.TdHTMLAttributes<HTMLTableCellElement>) {
  return <td className={cn("px-3 py-2", className)} {...props} />;
}
```

- [ ] **Step 3: Empty / Loading / Error states**

Create `packages/ui/src/components/EmptyState.tsx`:
```tsx
import * as React from "react";

export interface EmptyStateProps {
  title: string;
  description?: string;
  action?: React.ReactNode;
}

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-gray-300 p-10 text-center">
      <p className="font-medium text-secondary">{title}</p>
      {description && <p className="mt-1 text-sm text-gray-600">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
```

Create `packages/ui/src/components/LoadingState.tsx`:
```tsx
import * as React from "react";

export interface LoadingStateProps {
  label?: string;
}

export function LoadingState({ label = "Loading…" }: LoadingStateProps) {
  return (
    <div role="status" aria-live="polite" className="flex items-center justify-center gap-2 p-10 text-sm text-gray-600">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-primary" />
      {label}
    </div>
  );
}
```

Create `packages/ui/src/components/ErrorState.tsx`:
```tsx
import * as React from "react";
import { Button } from "./Button";

export interface ErrorStateProps {
  title?: string;
  message?: string;
  onRetry?: () => void;
}

export function ErrorState({ title = "Something went wrong", message, onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-danger/30 bg-danger/5 p-10 text-center">
      <p className="font-medium text-danger">{title}</p>
      {message && <p className="mt-1 text-sm text-gray-600">{message}</p>}
      {onRetry && <Button className="mt-4" variant="danger" onClick={onRetry}>Retry</Button>}
    </div>
  );
}
```

- [ ] **Step 4: Domain shells (presentational only)**

Create `packages/ui/src/components/QueueCard.tsx`:
```tsx
import * as React from "react";
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
```

Create `packages/ui/src/components/PaymentCard.tsx`:
```tsx
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
```

Create `packages/ui/src/components/TicketCard.tsx`:
```tsx
import * as React from "react";
import { Card, CardBody } from "./Card";
import { Badge } from "./Badge";

export interface TicketCardProps {
  eventName: string;
  categoryName: string;
  orderNumber: string;
  status: "pending" | "confirmed" | "cancelled";
}

const statusTone = { pending: "warning", confirmed: "success", cancelled: "neutral" } as const;

export function TicketCard({ eventName, categoryName, orderNumber, status }: TicketCardProps) {
  return (
    <Card>
      <CardBody>
        <div className="flex items-start justify-between">
          <div>
            <p className="font-semibold text-secondary">{eventName}</p>
            <p className="text-sm text-gray-600">{categoryName}</p>
          </div>
          <Badge tone={statusTone[status]}>{status}</Badge>
        </div>
        <p className="mt-3 font-mono text-xs text-gray-500">{orderNumber}</p>
      </CardBody>
    </Card>
  );
}
```

- [ ] **Step 5: Barrel export**

Create `packages/ui/src/index.ts`:
```ts
export { Button } from "./components/Button";
export type { ButtonProps, ButtonVariant, ButtonSize } from "./components/Button";
export { Input } from "./components/Input";
export type { InputProps } from "./components/Input";
export { Textarea } from "./components/Textarea";
export type { TextareaProps } from "./components/Textarea";
export { Select } from "./components/Select";
export type { SelectProps, SelectOption } from "./components/Select";
export { Checkbox } from "./components/Checkbox";
export type { CheckboxProps } from "./components/Checkbox";
export { RadioGroup } from "./components/Radio";
export type { RadioGroupProps, RadioOption } from "./components/Radio";
export { Badge } from "./components/Badge";
export type { BadgeProps, BadgeTone } from "./components/Badge";
export { Alert } from "./components/Alert";
export type { AlertProps, AlertTone } from "./components/Alert";
export { Card, CardHeader, CardBody, CardFooter } from "./components/Card";
export { Modal } from "./components/Modal";
export type { ModalProps } from "./components/Modal";
export { Dialog } from "./components/Dialog";
export type { DialogProps } from "./components/Dialog";
export { Table, THead, TBody, TR, TH, TD } from "./components/Table";
export { EmptyState } from "./components/EmptyState";
export type { EmptyStateProps } from "./components/EmptyState";
export { LoadingState } from "./components/LoadingState";
export type { LoadingStateProps } from "./components/LoadingState";
export { ErrorState } from "./components/ErrorState";
export type { ErrorStateProps } from "./components/ErrorState";
export { QueueCard } from "./components/QueueCard";
export type { QueueCardProps } from "./components/QueueCard";
export { PaymentCard } from "./components/PaymentCard";
export type { PaymentCardProps } from "./components/PaymentCard";
export { TicketCard } from "./components/TicketCard";
export type { TicketCardProps } from "./components/TicketCard";
```

- [ ] **Step 6: README**

Create `packages/ui/README.md` documenting: install, theme tokens, and each component's
props + a usage example. Structure:
```markdown
# @ivyticketing/ui

Design-system foundation for ivyticketing. React + TypeScript + Tailwind, Radix primitives
for accessible Select/Checkbox/Radio/Dialog. Presentational only — no data fetching.

## Theme

| Token | Value |
|---|---|
| primary | #0B3D2E |
| secondary | #111827 |
| accent | #D6A84F |
| background | #F8F7F2 |
| success | #16A34A |
| warning | #F59E0B |
| danger | #DC2626 |

Font: Inter. Import `src/theme.css` once at the app root.

## Components

### Button
Props: `variant` (primary|secondary|accent|danger|ghost), `size` (sm|md|lg), `loading`, plus native button attrs.
```tsx
import { Button } from "@ivyticketing/ui";
<Button variant="primary" onClick={...}>Checkout</Button>
```

(Document every component the same way: Input, Textarea, Select, Checkbox, RadioGroup,
Badge, Alert, Card+sub, Modal, Dialog, Table+sub, EmptyState, LoadingState, ErrorState,
QueueCard, PaymentCard, TicketCard — each with props list + a short example. The domain
cards QueueCard/PaymentCard/TicketCard are presentational shells; note they take plain
props and are not yet wired to any API.)
```
Write the full README covering all components.

- [ ] **Step 7: Typecheck**

Run:
```bash
cd packages/ui && pnpm typecheck; cd ../..
```
Expected: `tsc --noEmit` passes — no type errors, no unused locals/params.

- [ ] **Step 8: Commit**

```bash
git add packages/ui
git commit -m "feat(ui): add modal/dialog/table/state components, domain shells, barrel, and README"
```

---
