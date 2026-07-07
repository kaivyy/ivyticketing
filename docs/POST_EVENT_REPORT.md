# Post-Event Report & Continuous Improvement

The event is not "done" when the queue drains. It is done when you have turned what
happened into a report, a bug list, and a prioritized backlog. This document is the
template for that, plus where the data lives so the report is evidence-based, not
anecdotal.

Run this within 48 hours of the event, while memory and logs are fresh.

---

## Where the data comes from

Every section below has a source you can query — do not rely on impressions.

| What to analyze        | Source of truth |
|------------------------|-----------------|
| Queue complaints       | Support channel + `QUEUE_OPERATIONS.md` metrics; queue depth over time in Grafana |
| Payment failed reasons | `payments` failed-order rows grouped by gateway reason code; reconciliation log |
| Checkout drop-off      | Order funnel: `PENDING` created vs `PAID`; abandonment by step (see reporting export) |
| Racepack bottleneck    | Scanner throughput + slot utilization (Phase 14 racepack dashboards) |
| Support tickets        | Support channel export, tagged by category |
| Error logs             | Sentry (grouped by release) + `/metrics` 5xx by route in Grafana |
| Organizer feedback     | Direct debrief with the EO |
| Participant feedback   | Post-event survey + social/support sentiment |

---

## Report template

Copy this section per event. Fill every field; "N/A" is an answer, blank is not.

### 1. Event summary
- Event name / date / organizer: `____`
- Peak concurrency (RPS, in-flight): `____`
- Total orders: created `____` / paid `____` (conversion `__%`)
- Total tickets issued: `____`
- Revenue processed: `____`

### 2. Reliability
- Uptime during event window: `__%`
- Error rate (5xx share) peak / average: `__%` / `__%`
- p95 latency peak / average: `__ms` / `__ms`
- Incidents opened: `____` (link each to the status-page incident)
- Time-to-detect / time-to-mitigate per incident: `____`

### 3. Payments
- Failed payment count and top-3 reason codes: `____`
- Reconciliation discrepancies found/resolved: `____`
- Double-payment or oversell incidents: **must be 0** — if not, root-cause immediately.

### 4. Queue & anti-bot
- Max queue depth / average wait: `____`
- Release rate profile (what was set, when): `____`
- Bot/abuse rejections (Phase 9 abuse guard): `____`
- Complaints attributable to the queue: `____`

### 5. Operations
- Racepack scan throughput / bottleneck observed: `____`
- Support ticket volume by category (top 5): `____`
- Runbook steps that were unclear or wrong (feed back into the runbook): `____`

### 6. Feedback
- Organizer: what worked, what hurt: `____`
- Participant: top 3 themes from survey/support: `____`

---

## Output artifacts (Phase 27 acceptance)

The report is not finished until these four artifacts exist:

- [ ] **Post-event report** — this template, filled, committed to the repo or the
      team's docs, linked from the event record.
- [ ] **Bug list** — every defect observed, filed as issues with severity and a link
      to evidence (error group, log line, support ticket). Distinguish *bugs* (broken)
      from *improvements* (works, could be better).
- [ ] **Improvement backlog** — prioritized. For each item: expected impact
      (reliability / conversion / cost / operator-effort), rough size, and the trigger
      that would make it urgent. Cross-reference [`SCALE_SPLIT.md`](./SCALE_SPLIT.md)
      if any item is a service-extraction trigger.
- [ ] **Performance tuning notes** — concrete: slow queries seen (with `EXPLAIN`),
      pool/timeout settings that needed adjusting, cache hit rates. Turn the biggest
      finding into a backlog item.

### Pricing review (from the masterplan)
- [ ] Revisit gateway fee vs. platform fee against actual volume; note whether the
      current pricing held margin at this event's scale. This is a business output of
      the same data, kept here so it is not forgotten.

---

## Turning the report into the next release

1. Triage the bug list: anything that risked an invariant (oversell, double payment,
   secret handling) is P0 regardless of how rare.
2. Pull the top backlog items into the next planning cycle with their impact tags.
3. Re-run the affected k6 scenario after each reliability fix so the improvement is
   proven, not assumed — then update the load-test baseline in
   [`LAUNCH_CHECKLIST.md`](./LAUNCH_CHECKLIST.md).
