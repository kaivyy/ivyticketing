# Documentation Index

All project docs, grouped by purpose. Start with [`masterplan.md`](masterplan.md) for the
27-phase build plan and [`../README.md`](../README.md) for setup.

## Product & planning

- [prd.md](prd.md) — product requirements.
- [masterplan.md](masterplan.md) — the 27-phase development plan.
- [masterprompt.md](masterprompt.md) — build methodology / working agreement.
- [struktur.md](struktur.md) — system structure & data model.

## Core flows

- [REGISTRATION_MODES.md](REGISTRATION_MODES.md) — registration modes.
- [INVENTORY.md](INVENTORY.md) — inventory & oversell prevention.
- [RESERVATION_SYSTEM.md](RESERVATION_SYSTEM.md) — reservation holds.
- [ORDER_FLOW.md](ORDER_FLOW.md) / [CHECKOUT_FLOW.md](CHECKOUT_FLOW.md) — order & checkout.
- [TICKET_FLOW.md](TICKET_FLOW.md) / [QR_TICKET.md](QR_TICKET.md) — tickets & QR signing.
- [PARTICIPANT_DASHBOARD.md](PARTICIPANT_DASHBOARD.md) — participant dashboard.

## Payments

- [PAYMENT_FLOW.md](PAYMENT_FLOW.md) — payment lifecycle.
- [GATEWAY_INTEGRATION.md](GATEWAY_INTEGRATION.md) — Duitku / Xendit integration.
- [WEBHOOK_PROCESSING.md](WEBHOOK_PROCESSING.md) — callback handling.
- [PAYMENT_RECONCILIATION.md](PAYMENT_RECONCILIATION.md) — reconciliation job.
- [payment/](payment/) — gateway-specific notes.

## Queue, anti-bot & ballot

- [QUEUE_MODES.md](QUEUE_MODES.md) / [QUEUE_OPERATIONS.md](QUEUE_OPERATIONS.md) — war queue.
- [ANTIBOT.md](ANTIBOT.md) / [ABUSE_OPERATIONS.md](ABUSE_OPERATIONS.md) / [RATE_LIMITING.md](RATE_LIMITING.md) — abuse protection.
- [BALLOT_ENGINE.md](BALLOT_ENGINE.md) — ballot / lottery.
- [ACCESS_ENGINE.md](ACCESS_ENGINE.md) — coupon / invitation / community slot.

## Enterprise & operations

- [ENTERPRISE_API.md](ENTERPRISE_API.md) — API keys & webhooks.
- [SCALE_SPLIT.md](SCALE_SPLIT.md) — when to extract a service from the monolith.
- [INCIDENT_RUNBOOK.md](INCIDENT_RUNBOOK.md) — incident response.
- [LAUNCH_CHECKLIST.md](LAUNCH_CHECKLIST.md) — war-day go/no-go.
- [POST_EVENT_REPORT.md](POST_EVENT_REPORT.md) — post-event analysis template.

## Phase decision records

- [PHASE5_DECISIONS.md](PHASE5_DECISIONS.md), [PHASE6_DECISIONS.md](PHASE6_DECISIONS.md),
  [PHASE7_DECISIONS.md](PHASE7_DECISIONS.md), [PHASE8_DECISIONS.md](PHASE8_DECISIONS.md),
  [PHASE9_DECISIONS.md](PHASE9_DECISIONS.md).
- [PHASE11_UIUX_AUDIT.MD](PHASE11_UIUX_AUDIT.MD) — UI/UX audit.
- [PHASE_15_READINESS.md](PHASE_15_READINESS.md) — scanner PWA readiness.
