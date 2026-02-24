# CRE CLI Testing Framework Design

> A comprehensive design for AI-augmented testing of the CRE CLI, focused on catching template breakage and cross-component integration failures before they reach developers.

---

## Context

The CRE CLI currently ships embedded templates that are the primary entry point for developers. A branch-gated dynamic template pull model is also planned. Both modes depend on Go and TypeScript SDKs, GraphQL APIs, on-chain contracts, and third-party packages -- all evolving independently. The current CI validates these in isolation, so cross-component breakage goes undetected until users report it.

## Template Source Modes

- Embedded mode (current baseline): templates bundled into CLI via `go:embed`.
- Dynamic pull mode (upcoming, branch-gated): templates fetched from the external template repository at runtime.
- All dynamic-pull guidance in this folder is preparatory until upstream branch/repo links are active.

This documentation package designs a three-tier testing framework that combines deterministic scripts, AI-driven validation, and targeted manual checks.

## Policy Snapshot

- **Merge-gating checks (required by default):** deterministic compatibility/smoke/negative-path checks.
- **Diagnostic checks (advisory by default):** broader AI/nightly exploratory runs unless explicitly promoted.
- **Manual/browser checks (non-gating by default):** subjective UX and browser-only flows.
- **Playwright credential bootstrap:** proposal-only local primitive; optional and non-baseline for this framework.

---

## Documents

| # | Document | Purpose |
|---|----------|---------|
| **START HERE** | [Implementation Plan](implementation-plan.md) | Concrete 2-week plan with 5 deliverables: test file, SDK matrix, PTY wrapper, macOS CI, AI skill. Includes YAML snippets, test pseudocode, and timeline. |
| 1 | [Testing Framework Architecture](01-testing-framework-architecture.md) | Overall framework design: three-tier model, component interactions, failure detection matrix, environment requirements |
| 2 | [Test Classification Matrix](02-test-classification-matrix.md) | Every test from the QA runbook classified as Script / AI / Manual, with rationale. Revised aggregate: 85% script, 8% AI, 7% manual |
| 3 | [PoC Specification](03-poc-specification.md) | Detailed spec for a proof-of-concept: two-track template validation (deterministic script + AI agent), agent prompt design, report format, implementation phases |
| 4 | [CI/CD Integration Design](04-ci-cd-integration-design.md) | GitHub Actions workflow designs: template compatibility job, SDK version matrix, AI validation job, cross-repo triggers, cost analysis, rollout plan |

---

## Key Numbers

| Metric | Current State | With Framework |
|--------|--------------|----------------|
| Templates tested in CI | 3 of 5 | 5 of 5 |
| Tests automated | ~45 (unit + partial E2E) | 109 (script + AI) |
| Tests requiring human | ~103 (full runbook) | 8 |
| SDK version matrix | None | Go + TS, pinned + latest |
| Platforms in CI | 2 (Ubuntu, Windows) | 3 (+ macOS) |
| Detection time for SDK breakage | Days to weeks (user report) | < 24 hours (nightly) or immediate (cross-repo trigger) |
| Estimated monthly cost | $0 (manual QA is engineer time) | ~$65 (CI + AI) |

---

## Reading Order

1. Start with **[implementation-plan.md](implementation-plan.md)** -- the actionable plan with specs and timeline
2. For background, read **01-testing-framework-architecture.md** for the big picture
3. For deep dive on automation boundaries, read **02-test-classification-matrix.md**
4. For PoC details, read **03-poc-specification.md**
5. For CI/CD details beyond the implementation plan, read **04-ci-cd-integration-design.md**
