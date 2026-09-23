# Contribution Guidelines and Engineering Standards

This document establishes the code quality, testing requirements, and contribution workflow for developers working on IvyTicketing.

---

## 1. Code Quality Standards

### Go Backend Standards
- **Formatting**: All Go code must be formatted using `gofmt -s -w .`.
- **Linting**: Run `golangci-lint run` prior to submitting pull requests.
- **Context Propagation**: Always pass `context.Context` as the first argument to database and network operations. Never use `context.Background()` inside HTTP handlers or service methods.
- **Error Wrapping**: Wrap errors with meaningful context using `fmt.Errorf("orders: failed to reserve quota: %w", err)`. Never discard returned errors.
- **No Masked Leaks**: Internal infrastructure errors must be converted to standard domain error envelopes via `apperr.WriteError`.

### Frontend TypeScript Standards
- **Strict Typing**: Zero `any` types permitted. Define explicit interfaces or types for all component props, API payloads, and state stores.
- **Module Format**: Use standard ES module `import` syntax. Never use CommonJS `require()`.
- **Anti-Slop Hygiene**: Avoid repetitive, redundant code comments. Document the *why* (non-obvious rationale, concurrency constraints), not the *what*.

---

## 2. Pull Request Workflow

1. **Branch Naming**: Use descriptive branch names:
   - `feat/ballot-auto-promotion`
   - `fix/queue-status-cache-ttl`
   - `perf/inventory-row-locking`
2. **Pre-Submission Checklist**:
   - [ ] All unit and integration tests pass: `go test ./internal/modules/...`
   - [ ] TypeScript checks pass: `pnpm check`
   - [ ] No regression in concurrency benchmarks.
   - [ ] Knowledge graph updated: `graphify update .`
3. **Commit Messages**: Write clear, imperative commit messages:
   - `fix(ballot): prevent false lapse during concurrent payment`
   - `feat(queue): implement 2s Redis status caching`
