---
baseline_schema: "2.0"
pack: "penguin-space"
document: "roadmap"
status: "active"
updated: "2026-08-10"
code_ref: "uncommitted"
---

# Product roadmap and verification state

## Baseline conversion status

| Item | Status | Evidence |
|---|---|---|
| Preserve original proposal | Complete | `sources/PROPOSAL.md` retained a byte-identical copy before root `PROPOSAL.md` was removed |
| Core adaptive pack | Complete | Introduction, roadmap, and decision/unknowns documents created |
| Architecture and usage contract | Complete | Source-code and use-guide documents created because the proposal defines both materially |
| Root proposal deletion | Complete | Root source removed after SHA-256 equality and pack-coverage verification |
| Product implementation | Not started / unverified | Repository has no commits or implementation files |

## Milestone 1 — Core platform

**Depends on:** no product milestone.
**Target:** establish the final architecture, not a throwaway MVP.

- Wails 3 app shell and Vue shell
- WinUI-inspired design system and navigation shell
- domain models, provider interface, detection/scanning, planning, execution, verification
- storage measurement, risk and recovery-cost models, history persistence
- elevation, logging, cancellation, timeouts, and error model

**Acceptance criteria:** the app can scan a test provider through the full scan → plan → confirm → execute → verify pipeline without frontend-owned shell commands; results expose risk, recovery cost, estimates, status, and actual reclaimed bytes where measurable.

**Verification gate:** provider and plan tests plus an end-to-end Windows run. A build alone is insufficient.

## Milestone 2 — Developer tool ecosystems

**Depends on:** Milestone 1 provider contract.

Implement detection, inspection, estimate, plan, cleanup, and verification for Node.js (npm, pnpm, Yarn, Bun), Python (uv), .NET (NuGet), Rust (Cargo), Java/JVM (Gradle, Maven), and Cypress/Playwright.

**Acceptance criteria:** each provider has version-aware semantics, a chosen official source/API or justified fallback, risk/recovery classification, user consequence text, and a verification strategy.

**Verification gate:** provider tests and a Windows test environment containing representative tools. Never infer support from command strings alone.

## Milestone 3 — Containers and WSL

**Depends on:** Milestone 1; Docker and Windows/WSL test environments.

Implement Docker/Docker Desktop/Rancher awareness; independent resource inspection and cleanup for images, containers, build cache, networks, and volumes; WSL distro and VHDX discovery; physical/logical measurement; compactability estimation; elevation, shutdown warnings, compaction, and post-operation verification.

**Acceptance criteria:** volumes are Danger by default; no blanket `docker system prune -a --volumes` action is offered as routine clean; VHDX compaction reports before/after physical size separately from logical deletion.

**Verification gate:** real Docker and WSL/VHDX scenarios, including daemon unavailable, a running distro, elevation refusal, and interrupted compaction.

## Milestone 4 — Project storage

**Depends on:** Milestone 1 scanner and workspace configuration.

Implement workspace discovery, ecosystem detection, recursive measurement, configured exclusions, reclaim estimates, project detail, cleanup planning, and reliable last-used heuristics where available.

**Acceptance criteria:** scanner identifies the approved initial generated folders without silently deleting unknown paths.

**Verification gate:** fixture workspaces covering excluded paths, symbolic/locked/error cases, and classification tests.

## Milestone 5 — Complete desktop experience

**Depends on:** useful data from Milestones 2–4.

Complete Home metrics and breakdown, largest consumers versus reclaim opportunities, virtual-disk card, filters/search, review/progress/result/history experiences, tray/background/startup/notifications/theme, resizing, empty/error/loading states, master-detail behavior, command bars, and context menus.

**Acceptance criteria:** a wide desktop viewport communicates storage, reclaimability, safety, and virtual-disk state without a giant checkbox list or excessive scrolling.

**Verification gate:** Windows visual and resize QA, keyboard/accessibility checks, and real cleanup-result presentation based on verified bytes.

## Milestone 6 — Production hardening

**Depends on:** Milestones 1–5.

Cover failure recovery, locked files, permission failures, malformed output, partially completed cleanup, concurrent scans, restart recovery, Docker/WSL edge cases, tests, Windows distribution, signing, updating, CI/CD, diagnostics, troubleshooting, and release documentation.

**Acceptance criteria:** destructive-operation failure modes are recoverable or clearly reported; release and diagnostic paths are documented; Windows-specific test coverage is established.

**Verification gate:** release candidate exercised on Windows with representative Docker and WSL environments. Static checks do not prove production readiness.

## Risks and dependencies

| Risk or dependency | Effect | Required handling |
|---|---|---|
| Tool CLI semantics change by version | Incorrect or unsafe cleanup | Verify current official docs and tool versions per provider |
| Docker volumes contain state | Irrecoverable loss | Danger classification, no preselection, strengthened confirmation |
| Logical cleanup does not shrink VHDX | Misleading reclaimed-space claim | Separate logical, estimated compactable, and actual physical results |
| UAC or environment shutdown fails | Cleanup/compaction stops partway | Plan-time privilege/shutdown indication, cancellation and recovery handling |
| No code exists yet | Direction may be mistaken for implemented reality | Keep all status claims explicitly planned or unverified |

## Exact next action

Create the Wails 3/Go/Vue application shell and a single fixture-backed provider-contract vertical slice that produces a reviewable cleanup plan but performs no destructive cleanup.

## Pack migration verification

Completed before root-proposal removal on 2026-08-10:

- Root and preserved source had matching SHA-256 values: `61777954f327b1157bf2c0268a3f283cda614e21ec71e70b728eb74657d4d97c`.
- Core, architecture, and use-guide pack documents were present and linked.
- The preserved source is retained unchanged; its pre-existing six Markdown line-break spaces were intentionally not normalized because lossless preservation took precedence.
