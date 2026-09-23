# Platform Administrator Onboarding Guide

This guide introduces platform administrators to the global governance, system telemetry, multi-tenant provisioning, and abuse control tools in IvyTicketing.

---

## 1. Platform Admin Authentication and Privileges

Platform administrators are identified by `users.is_platform_admin = true`.

- **Access Level**: Full cross-tenant read/write authority across the `/api/v1/admin` route tree.
- **Bypass Capabilities**: Bypasses organization membership checks in [`RequirePermission`](file:///root/ivyticketing/services/api/internal/platform/middleware/authz.go#L115).
- **Audit Logging**: All administrative actions are permanently logged in the PostgreSQL `audit_logs` table with actor ID, IP address, and changed payload diffs.

---

## 2. Navigating the Admin Console

When logged in as a platform administrator, the navigation bar displays the **Platform Admin** portal:

- **War Room Telemetry**: Live operational metrics, database pool status, queue depths, and gateway latency.
- **Organizations**: Tenant directory, onboarding, subscription packages, and domain management.
- **Abuse & Bot Guard**: Global reputation rules, IP blocklists, and CAPTCHA challenge thresholds.
- **Platform Invoicing**: Fee ledger calculations, monthly invoices, and payout reconciliations.
- **System Status**: Incident management and public component status indicators.

---

## 3. Provisioning a New Organizer Tenant

1. Navigate to **Organizations** -> **New Organization**.
2. Input the organization details:
   - **Organization Name**: e.g., "Mandalika Sports Club"
   - **Slug**: Unique URL slug (e.g. `mandalika`)
   - **Contact Email**: Billing and primary contact address
3. Assign a **Subscription Package**:
   - `Starter`: Standard road events, direct checkout.
   - `Professional`: War queue enabled, automated BIB assignment.
   - `Enterprise`: White-labeling, custom domain, full competition engine, dedicated API keys.
4. Invite the initial organization owner by providing their email address.

---

## 4. Monitoring the War Room Telemetry

During major event registration openings, open the **War Room** dashboard (`/admin/warroom` or `GET /api/v1/admin/warroom`):

- **Database Connection Saturation**: Monitors active versus idle connections in `pgxpool`.
- **Redis Queue Depth**: Real-time count of waiting tokens across active `WAR_QUEUE` events.
- **Admission Release Velocity**: Number of participants admitted per 10-second interval.
- **Webhook Delivery Throughput**: Inbound transaction callbacks processed per second.
- **Order Conversion Ratio**: Percentage of admitted queue participants who successfully complete payment.

---

## 5. Configuring Global Bot Protection

To shield the platform from automated scalper bots and DDoS attacks:

1. Navigate to **Abuse & Bot Guard** -> **Security Settings**.
2. Configure **Cloudflare Turnstile**: Ensure valid `Turnstile Site Key` and `Turnstile Secret Key` are loaded.
3. Review **Reputation Thresholds**:
   - `Challenge Threshold` (default: `10`): Requests exceeding this score must solve an interactive Turnstile captcha.
   - `Deny Threshold` (default: `25`): Requests exceeding this score receive an immediate `403 FORBIDDEN`.
4. Inspect the **Live Blocklist** to manually ban offending ASN ranges or suspicious IP subnets.
