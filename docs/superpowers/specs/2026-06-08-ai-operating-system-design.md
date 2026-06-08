# ivyticketing AI Operating System — Design

**Date:** 2026-06-08
**Status:** Approved (brainstorming)
**Scope:** AI skill ecosystem only. No application code.

---

## 1. Goal

Build a modular AI skill system so that any coding agent (Claude Code, Codex, Cursor, etc.)
working in this repo preserves architectural correctness from Phase 8 through Phase 27.

It must prevent: architectural drift, UI inconsistency, business-logic drift, phase
confusion, accidental rewrites, payment/queue/transaction regressions.

Core philosophy: **EXTEND, NEVER REWRITE.** Priority order is fixed:
**Correctness > Stability > Security > Transparency > Performance > UX polish.**

---

## 2. Decisions (locked in brainstorming)

| Decision | Choice |
|---|---|
| Location & format | Hybrid: `.claude/skills/<name>/SKILL.md` with Claude Code **native** frontmatter (`name`, `description`); custom field (`type`) and structured sections live in the body so non-CC agents read raw markdown. |
| Content depth | Skills hold durable **rules + decision criteria**; detailed implementation reference points to existing `docs/`. Prevents drift. |
| Always-loaded layer | New repo-root `CLAUDE.md` carries non-negotiables + skill index (only true always-on mechanism in Claude Code). Skills carry depth, invoked on demand. |
| Agent staffing | Generate **real** `.claude/agents/iv-*.md` reviewer subagents that the staffing skill dispatches. |
| Reviewer count | 7 reviewer subagents (implementer = main agent, no file). |

---

## 3. Directory layout

```
CLAUDE.md                                              # NEW — always loaded
.claude/
  skills/
    ivyticketing-global-constitution/SKILL.md          # always invoked
    ivyticketing-masterplan/SKILL.md                   # always invoked
    ivyticketing-current-phase/SKILL.md                # always invoked — REPLACEABLE per phase
    ivyticketing-transaction-safety/SKILL.md           # conditional
    ivyticketing-ui-consistency/SKILL.md               # conditional
    ivyticketing-agent-staffing/SKILL.md               # conditional
    ivyticketing-release-management/SKILL.md           # conditional
    ivyticketing-architecture-review/SKILL.md          # conditional
    ivyticketing-risk-assessment/SKILL.md              # conditional
    ivyticketing-testing-strategy/SKILL.md             # conditional
    ivyticketing-database-governance/SKILL.md          # conditional
    ivyticketing-security-review/SKILL.md              # conditional
  agents/
    iv-transaction-auditor.md
    iv-architecture-reviewer.md
    iv-ui-reviewer.md
    iv-security-reviewer.md
    iv-database-reviewer.md
    iv-test-writer.md
    iv-release-manager.md
```

## 4. Frontmatter convention

Native fields for Claude Code loading; custom `type` and structured sections in the body.

```markdown
---
name: ivyticketing-transaction-safety
description: >
  Money/inventory/voucher/callback safety rules. Use BEFORE writing or reviewing
  any code under services/api/internal/modules/{orders,payments,inventory,tickets},
  or anything touching reservations, vouchers, refunds, webhooks, or callbacks.
---
**Type:** principle

## Purpose
## Rules
## When To Use
## When Not To Use
## Required Outputs
## References
```

`description` is the load trigger, so each one names concrete paths + keywords an agent
matches against.

## 5. Loading model

- **Always loaded:** `CLAUDE.md`. Holds project identity, 6-priority order, extend-never-rewrite,
  the transaction-safety gate, the current-phase pointer, and the skill index (when to invoke each).
- **Always invoked at task start** (CLAUDE.md instructs): `global-constitution`, `masterplan`,
  `current-phase`.
- **Conditional** (invoked by trigger match):
  - `transaction-safety` → touching orders / payments / inventory / tickets / vouchers / callbacks
  - `database-governance` → any migration under `database/migrations/`
  - `ui-consistency` → any change under `apps/web/`
  - `security-review` → auth, RBAC, tenant isolation, tokens, secrets, callbacks
  - `architecture-review` → new module, cross-module dependency, service-split temptation
  - `testing-strategy` → adding/altering tests or a feature needing tests
  - `risk-assessment` → before any merge/deploy decision
  - `release-management` → merge / push / deploy / migration rollout
  - `agent-staffing` → high-risk change needing reviewer dispatch

## 6. The 12 skills (one-line purpose each)

1. **global-constitution** — project identity, priorities, extend-never-rewrite, forbidden behavior, AI decision hierarchy. Always on.
2. **masterplan** — all 27 phases (name, purpose, dependencies, exit criteria) + MVP/launch/enterprise scope. Lets AI place any work in "Phase X".
3. **current-phase** — Phase 8 Queue/War: goals, allowed/forbidden scope, acceptance criteria, refresh/reconnect/mobile-sleep safety, one-queue-entry rule, admission control. **Replaceable.**
4. **transaction-safety** — payment/inventory/voucher/callback/refund/bulk/cleanup rules; idempotency, atomic update, `SELECT FOR UPDATE`, affectedRows guards, unique constraints, race/rollback/failure-path analysis. Mandatory audit checklist.
5. **ui-consistency** — spacing/typography/cards/forms/tables/colors/loading/error states; prohibits random patterns/libraries; Indonesian UX copy guidelines.
6. **agent-staffing** — reviewer roles + when each is required + the high-risk-change workflow. Dispatches the `iv-*` subagents.
7. **release-management** — pre-merge / pre-push / pre-deploy / post-deploy checklists, rollback, migration rollout, smoke tests.
8. **architecture-review** — review scalability/maintainability/coupling/complexity/future-phase compatibility; prevent premature microservices.
9. **risk-assessment** — P0–P3 classification; block-deploy vs safe-deploy criteria.
10. **testing-strategy** — required tests per subsystem (payment/voucher/queue/inventory/callback/invoice/RBAC/bulk) + chaos/race/concurrency tests.
11. **database-governance** — migration/index/partition/unique-constraint/pre-check/rollback rules.
12. **security-review** — authn/authz/tenant isolation/callback verification/IDOR/token/secret handling/logging policy.

## 7. The 7 reviewer subagents

Each is read-only-by-default, scoped to its domain, and instructed to invoke its companion skill.

| Agent file | Backs skill | Dispatched when |
|---|---|---|
| `iv-transaction-auditor.md` | transaction-safety | money/inventory/voucher/callback change |
| `iv-architecture-reviewer.md` | architecture-review | new module / cross-module coupling |
| `iv-ui-reviewer.md` | ui-consistency | `apps/web/` change |
| `iv-security-reviewer.md` | security-review | auth/RBAC/token/secret/callback change |
| `iv-database-reviewer.md` | database-governance | migration added |
| `iv-test-writer.md` | testing-strategy | feature/bugfix needing tests |
| `iv-release-manager.md` | release-management | merge/deploy gate |

Implementer = the main agent (no file).

High-risk workflow (in agent-staffing): implementer writes → dispatch relevant reviewer(s)
→ release-manager gate → human confirm for irreversible actions.

## 8. Phase evolution (Phase 8 → 27)

Only `current-phase/SKILL.md` changes when a phase ships. Procedure (documented inside that skill):

1. Archive the outgoing phase's content to a dated note.
2. Rewrite goals / allowed-scope / forbidden-scope / acceptance-criteria for the new phase
   (sourced from `masterplan`).
3. Update the one-line current-phase pointer in `CLAUDE.md`.

`masterplan` and the other 10 skills are phase-agnostic and stay untouched. This is what keeps
the system useful and low-maintenance across 20 phases.

## 9. Doc references (skills point here, don't duplicate)

`docs/masterplan.md`, `docs/prd.md`, `docs/INVENTORY.md`, `docs/RESERVATION_SYSTEM.md`,
`docs/PAYMENT_FLOW.md`, `docs/WEBHOOK_PROCESSING.md`, `docs/PAYMENT_RECONCILIATION.md`,
`docs/ORDER_FLOW.md`, `docs/CHECKOUT_FLOW.md`, `docs/QR_TICKET.md`, `docs/TICKET_FLOW.md`,
`docs/GATEWAY_INTEGRATION.md`, `docs/payment/CALLBACK_SECURITY.md`, `docs/PHASE{5,6,7}_DECISIONS.md`,
`CHANGELOG.md`, `database/migrations/`.

## 10. Out of scope

- No application code.
- No changes to existing `.qoder/skills` (workflow skills, separate concern).
- No new docs/ reference files (skills reference existing ones).
```
