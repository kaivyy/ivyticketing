# @ivyticketing/ui

Design-system foundation for ivyticketing. React + TypeScript + Tailwind, Radix primitives
for accessible Select/Checkbox/Radio/Dialog. Presentational only — no data fetching.

## Install

```bash
pnpm add @ivyticketing/ui
```

Import the theme CSS once at the app root:

```ts
import "@ivyticketing/ui/src/theme.css";
```

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

<Button variant="primary" onClick={handleCheckout}>Checkout</Button>
<Button variant="danger" size="sm" loading>Deleting…</Button>
```

### Input

Props: `invalid` (boolean), plus all native input attrs.

```tsx
import { Input } from "@ivyticketing/ui";

<Input placeholder="Email address" invalid={hasError} />
```

### Textarea

Props: `invalid` (boolean), plus all native textarea attrs.

```tsx
import { Textarea } from "@ivyticketing/ui";

<Textarea rows={4} placeholder="Notes…" invalid={hasError} />
```

### Select

Props: `value`, `defaultValue`, `onValueChange`, `options` (array of `{ value, label }`), `placeholder`, `disabled`.
Uses Radix Select for accessibility.

```tsx
import { Select } from "@ivyticketing/ui";

<Select
  options={[{ value: "ga", label: "General Admission" }, { value: "vip", label: "VIP" }]}
  placeholder="Choose category"
  onValueChange={(v) => setCategory(v)}
/>
```

### Checkbox

Props: `checked`, `defaultChecked`, `onCheckedChange`, `disabled`, `id`.
Uses Radix Checkbox for accessibility.

```tsx
import { Checkbox } from "@ivyticketing/ui";

<label htmlFor="agree" className="flex items-center gap-2">
  <Checkbox id="agree" onCheckedChange={setAgreed} />
  I agree to the terms
</label>
```

### RadioGroup

Props: `value`, `defaultValue`, `onValueChange`, `options` (array of `{ value, label }`), `name`, `disabled`.
Uses Radix Radio Group for accessibility.

```tsx
import { RadioGroup } from "@ivyticketing/ui";

<RadioGroup
  options={[{ value: "card", label: "Card" }, { value: "bank", label: "Bank Transfer" }]}
  onValueChange={setPaymentMethod}
/>
```

### Badge

Props: `tone` (neutral|success|warning|danger|accent), plus all native span attrs.

```tsx
import { Badge } from "@ivyticketing/ui";

<Badge tone="success">Confirmed</Badge>
<Badge tone="warning">Pending</Badge>
<Badge tone="danger">Cancelled</Badge>
```

### Alert

Props: `tone` (info|success|warning|danger), `title`, plus all native div attrs.

```tsx
import { Alert } from "@ivyticketing/ui";

<Alert tone="warning" title="Queue full">
  Try again in a few minutes.
</Alert>
```

### Card, CardHeader, CardBody, CardFooter

Composable card primitives. All accept standard div attrs plus `className`.

```tsx
import { Card, CardHeader, CardBody, CardFooter } from "@ivyticketing/ui";

<Card>
  <CardHeader>Order #12345</CardHeader>
  <CardBody>2× General Admission</CardBody>
  <CardFooter>Total: $80.00</CardFooter>
</Card>
```

### Modal

Props: `open`, `onOpenChange`, `title`, `description`, `children`, `footer`.
Built on Radix Dialog for accessible focus trapping and aria attributes.

```tsx
import { Modal, Button } from "@ivyticketing/ui";

<Modal
  open={isOpen}
  onOpenChange={setIsOpen}
  title="Confirm order"
  description="You are about to purchase 2 tickets."
  footer={<Button onClick={() => setIsOpen(false)}>Close</Button>}
>
  <p>Order details here…</p>
</Modal>
```

### Dialog

A confirm-style dialog built on Modal. Props: `open`, `onOpenChange`, `title`, `description`,
`confirmLabel` (default "Confirm"), `cancelLabel` (default "Cancel"), `onConfirm`, `destructive`.

```tsx
import { Dialog } from "@ivyticketing/ui";

<Dialog
  open={showDelete}
  onOpenChange={setShowDelete}
  title="Delete order?"
  description="This cannot be undone."
  confirmLabel="Delete"
  destructive
  onConfirm={handleDelete}
/>
```

### Table, THead, TBody, TR, TH, TD

Composable table primitives. All accept standard HTML table element attrs plus `className`.

```tsx
import { Table, THead, TBody, TR, TH, TD } from "@ivyticketing/ui";

<Table>
  <THead>
    <TR>
      <TH>Order</TH>
      <TH>Event</TH>
      <TH>Status</TH>
    </TR>
  </THead>
  <TBody>
    <TR>
      <TD>#12345</TD>
      <TD>Summer Concert</TD>
      <TD>Confirmed</TD>
    </TR>
  </TBody>
</Table>
```

### EmptyState

Props: `title`, `description`, `action` (ReactNode).

```tsx
import { EmptyState, Button } from "@ivyticketing/ui";

<EmptyState
  title="No orders yet"
  description="Your purchased tickets will appear here."
  action={<Button variant="accent">Browse events</Button>}
/>
```

### LoadingState

Props: `label` (default "Loading…").

```tsx
import { LoadingState } from "@ivyticketing/ui";

<LoadingState label="Fetching your tickets…" />
```

### ErrorState

Props: `title` (default "Something went wrong"), `message`, `onRetry`.

```tsx
import { ErrorState } from "@ivyticketing/ui";

<ErrorState
  title="Failed to load orders"
  message="Check your connection and try again."
  onRetry={refetch}
/>
```

### QueueCard

Presentational domain shell. Props: `position` (number), `estimatedWait` (string, optional), `status` ("waiting"|"allowed"|"expired").
Not yet wired to any API.

```tsx
import { QueueCard } from "@ivyticketing/ui";

<QueueCard position={42} estimatedWait="5 min" status="waiting" />
```

### PaymentCard

Presentational domain shell. Props: `amount` (string), `method` (string, optional), `status` ("pending"|"paid"|"expired"|"failed"), `footer` (ReactNode, optional).
Not yet wired to any API.

```tsx
import { PaymentCard, Button } from "@ivyticketing/ui";

<PaymentCard
  amount="€80.00"
  method="Credit card"
  status="pending"
  footer={<Button variant="accent">Pay now</Button>}
/>
```

### TicketCard

Presentational domain shell. Props: `eventName`, `categoryName`, `orderNumber`, `status` ("pending"|"confirmed"|"cancelled").
Not yet wired to any API.

```tsx
import { TicketCard } from "@ivyticketing/ui";

<TicketCard
  eventName="Summer Concert"
  categoryName="General Admission"
  orderNumber="ORD-20240801-12345"
  status="confirmed"
/>
```
