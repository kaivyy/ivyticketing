# Organizer Quickstart and Event Launch Guide

This guide walks event organizers through setting up their workspace, inviting staff members, connecting payment gateways, and launching their first sporting event.

---

## 1. Accessing Your Organization Workspace

1. Log in to IvyTicketing with an account assigned the `Owner` or `Manager` role.
2. Select your organization from the workspace switcher or navigate directly to `/organizations/{orgSlug}`.
3. Review your organization profile, legal name, contact email, and default currency settings.

---

## 2. Inviting Team Members and Assigning Roles

Maintain security and separation of duties by assigning team members role-specific permissions:

1. Navigate to **Team & Permissions** (`/organizations/{orgSlug}/members`).
2. Click **Invite Member**.
3. Enter the team member's email address and assign one or more pre-configured system roles:
   - **Manager**: Complete event operations and ticket oversight.
   - **Finance**: Access to financial reconciliation, refunds, and invoices.
   - **Customer Service**: Participant lookup, order review, and problem desk assistance.
   - **Racepack Staff**: Expo scanner and racepack check-in permissions.
   - **Timing Operator**: Access to timing split feeds and race result imports.
4. The member will receive an invitation to access the workspace.

---

## 3. Connecting Payment Gateways

IvyTicketing supports Duitku and Xendit payment gateways:

1. Navigate to **Finance & Payments** -> **Gateway Settings**.
2. Select your gateway:
   - For **Duitku**: Input your `Merchant Code` and `Merchant API Key`.
   - For **Xendit**: Input your `Secret API Key` and `Callback Verification Token`.
3. Set environment to `Sandbox` for initial testing.
4. Ensure your gateway provider dashboard is configured with your callback URL:
   `https://<your-domain>/api/v1/payments/<gateway>/callback`.

---

## 4. Creating Your First Event

1. Navigate to **Events** -> **Create Event**.
2. Enter the event metadata:
   - **Title**: e.g., "Jakarta City Marathon 2026"
   - **Sport**: Running / Cycling / Triathlon / Multi-Sport
   - **Location**: Venue name, address, and city
   - **Event Dates**: Race day and expo dates
   - **Description**: Banner image, overview, and schedule
3. Save as `DRAFT`.

---

## 5. Adding Ticket Categories and Quotas

1. In your event dashboard, open the **Categories** tab.
2. Click **Add Category** for each race distance:
   - **Name**: e.g. "Full Marathon 42K"
   - **Price**: Ticket base price (e.g. `850,000 IDR`)
   - **Total Quota**: Authoritative capacity (e.g. `2,500`)
   - **Min/Max Age**: Eligibility restrictions
   - **Max Tickets Per User**: Order limits (default: `1`)
3. Save each category.

---

## 6. Configuring Registration Modes and Windows

1. Navigate to the **Registration Settings** tab.
2. Set the **Registration Window**: Opening timestamp and closing timestamp.
3. Select the **Registration Mode**:
   - For expected high-demand sales, choose `WAR_QUEUE` with a release rate of 100 users per 10 seconds.
   - For oversubscribed elite events, choose `BALLOT` and configure draw dates.
   - For community events, choose `NORMAL`.

---

## 7. Pre-Launch Verification and Publishing

1. **Test Registration**: Run a test registration using sandbox payment credentials to verify inventory decrements, email notifications, and ticket QR code generation.
2. **Review Forms & Waivers**: Ensure required participant questions and liability waivers are properly attached.
3. **Publish**: Click **Publish Event**. The event is now live in the public catalog and ready to accept registrations at the scheduled opening time.
