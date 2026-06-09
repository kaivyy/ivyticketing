# Access Engine — Operations Guide

## Overview

The Access Engine controls registration access via typed pools and codes. All access-controlled registration modes (INVITATION_ONLY, PRIORITY_ACCESS, WAITLIST_ONLY) funnel through this engine.

## Pool Types

| Type | Description | Members |
|---|---|---|
| RESERVED | Used by ballot winners — direct grant issuance | None |
| INVITATION | Code-gated access, single or multi-use | Optional |
| PRIORITY | Auto-granted to eligible users during priority window | Auto (eligibility rule) |
| COMMUNITY | Self-apply with eligibility check | Self-apply or bulk |
| CORPORATE | Bulk-issued to corporate account members | CSV upload |
| VIP | Manually assigned by organizer | Manual |
| ELITE | Manually assigned by organizer | Manual |
| SPONSOR | Event sponsor access | Manual |
| PARTNER | Cross-org partner access | Partner upload |

## Redemption Flow

1. Participant enters code at event page
2. `POST /api/v1/events/{eventId}/access/redeem` body: `{code, categoryId}`
3. Server: sha256 hash lookup → expiry check → eligibility check → `ReserveSlot` → `CreateGrant`
4. Response: `{token: grant.id}` — participant passes this as `admissionToken` at checkout

**Code values are never stored in plain text. Only sha256 hashes are stored.**

## Priority Window

1. Organizer creates LifecyclePhase with `registration_mode=PRIORITY_ACCESS`
2. Organizer creates PRIORITY pool with `eligibility_rule`
3. Eligible participant visits event page → `GET /api/v1/events/{eventId}/access/priority-window`
4. Server auto-issues AccessGrant if eligible and window open
5. Participant proceeds to checkout with grant token

## Corporate Registration

1. Organizer creates corporate account (`POST /org/{orgId}/access/corporate`)
2. Organizer approves account (`POST /org/{orgId}/access/corporate/{id}/approve`)
3. Organizer creates CORPORATE pool, uploads member CSV
4. Each member receives access code via email (external — platform issues codes, delivery is organizer responsibility)
5. Member redeems code via normal redemption flow

## Waitlist

When category mode is WAITLIST_ONLY:
- Participant joins waitlist (`POST /events/{id}/categories/{id}/waitlist/join`)
- When a slot opens (cancellation/refund): `WaitlistEngine.PromoteBatch` fires
- Promoted participant receives AccessGrant notification
- Participant uses grant token at checkout

## Security

- Code values: sha256-hashed at rest, never logged
- Failed redemption (wrong code): +2 IP reputation bump
- 3 failed redemptions from same IP within 60s: IP auto-blocked for 10 min (configurable)
- Rate limits: 10/IP/min, 5/user/min on redemption endpoint

### Brute-Force Block Settings (platform_settings)

| Key | Default | Description |
|---|---|---|
| `code_brute_force_block` | `true` | Enable/disable brute-force blocking |
| `code_brute_force_window` | `60` | Window in seconds for counting failures |
| `code_brute_force_max_tries` | `3` | Max failures before block |
| `code_brute_force_block_dur` | `600` | Block duration in seconds |

## Troubleshooting

| Error | Meaning | Resolution |
|---|---|---|
| `CODE_NOT_FOUND` | Code hash not in DB or wrong event | Verify code and event |
| `CODE_EXHAUSTED` | use_count >= max_uses or pool full | Organizer must add slots or issue new code |
| `CODE_EXPIRED` | now() outside valid_from..valid_until | Organizer extends valid_until |
| `NOT_ELIGIBLE` | Eligibility rule check failed | User doesn't meet criteria |
| `POOL_EXHAUSTED` | No available slots | Organizer increases total_slots |
| `PRIORITY_WINDOW_CLOSED` | Lifecycle phase not active | Wait for window or contact organizer |
| `GRANT_EXPIRED` | Grant issued but not used in time | Re-redeem code (if slots remain) |
| `REGISTRATION_CLOSED` | Category mode is CLOSED | Organizer must re-open category |
| `REGISTRATION_WINDOW_CLOSED` | Lifecycle phase not active for mode | Wait for window to open |
