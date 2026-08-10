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
| Phase 0 — Research and decision closure | Not started | All nine implementation decisions are assigned here; no research result is yet accepted as product fact |
| Product implementation | Not started / unverified | Repository has no commits or implementation files |

## Phase 0 — Research and decision closure

**Depends on:** no product milestone. This phase converts the approved product direction and Docker-only build policy into evidence-backed implementation decisions. It does not implement a cleanup provider or perform destructive cleanup.

Close the following decisions with an evidence record for each: pinned container toolchain and Windows packaging feasibility; supported tool versions and provider command/API details; storage calculation/rounding; settings/history/diagnostic persistence; Wails-to-Windows elevation bridge; WSL/VHDX compaction; project-root discovery defaults; and Danger confirmation language.

**Required outputs:** a decision register containing owner, scope, current official/source evidence, test or spike result, chosen approach, rejected alternatives where safety-relevant, and follow-on implementation task. The Docker decision must also define the proposed `verify`, `build`, and `shell` Compose responsibilities and cache boundary from [the Docker build runbook](../../runbook/docker-build-environment.md).

**Acceptance criteria:** every current open decision is either closed with the required evidence or explicitly deferred with owner, reason, risk, and a blocking/non-blocking designation. The list in [penguin-space.hallucination.md](penguin-space.hallucination.md) is synchronized so it contains only genuinely open items.

**Verification gate:** no implementation milestone begins on an unverified assumption that Phase 0 identifies as blocking. A research note, a successful image build, or a static check alone is not sufficient evidence for a closed runtime decision.

## Milestone 1 — Core platform

**Depends on:** Phase 0 decisions marked blocking for the Docker build environment, measurement model, persistence, elevation, and initial provider contract.
**Target:** establish the final architecture, not a throwaway MVP.

- Wails 3 app shell and Vue shell
- Dockerfile and Compose manifest with pinned, container-owned toolchains and `verify`, `build`, and `shell` responsibilities
- WinUI-inspired design system and navigation shell
- domain models, provider interface, detection/scanning, planning, execution, verification
- storage measurement, risk and recovery-cost models, history persistence
- elevation, logging, cancellation, timeouts, and error model

**Acceptance criteria:** the app can scan a test provider through the full scan → plan → confirm → execute → verify pipeline without frontend-owned shell commands; results expose risk, recovery cost, estimates, status, and actual reclaimed bytes where measurable.

**Verification gate:** a clean Windows host, without PenguinSpace Go/Wails/Node/Windows build SDK toolchains, can run the containerized verification path; then provider and plan tests plus an end-to-end Windows run. A build alone is insufficient.

## Milestone 2 — Developer tool ecosystems

**Depends on:** Phase 0 provider research and Milestone 1 provider contract.

Implement detection, inspection, estimate, plan, cleanup, and verification for Node.js (npm, pnpm, Yarn, Bun), Python (uv), .NET (NuGet), Rust (Cargo), Java/JVM (Gradle, Maven), and Cypress/Playwright.

**Acceptance criteria:** each provider has version-aware semantics, a chosen official source/API or justified fallback, risk/recovery classification, user consequence text, and a verification strategy.

**Verification gate:** provider tests and a Windows test environment containing representative tools. Never infer support from command strings alone.

## Milestone 3 — Containers and WSL

**Depends on:** Phase 0 Docker/WSL findings; Milestone 1; Docker and Windows/WSL test environments.

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
| Host toolchain drift or cache pollution | Non-reproducible builds and WSL/Docker storage pressure | Keep all project toolchains and caches in pinned container images or named volumes; do not bind-mount host SDK/cache directories |
| No code exists yet | Direction may be mistaken for implemented reality | Keep all status claims explicitly planned or unverified |

## Exact next action

Start Phase 0 by creating the decision register and resolving the Docker build-image contract through a non-destructive container spike; record the result before creating the Wails 3/Go/Vue application shell.

## Pack migration verification

Completed before root-proposal removal on 2026-08-10:

- Root and preserved source had matching SHA-256 values: `61777954f327b1157bf2c0268a3f283cda614e21ec71e70b728eb74657d4d97c`.
- Core, architecture, and use-guide pack documents were present and linked.
- The preserved source is retained unchanged; its pre-existing six Markdown line-break spaces were intentionally not normalized because lossless preservation took precedence.
