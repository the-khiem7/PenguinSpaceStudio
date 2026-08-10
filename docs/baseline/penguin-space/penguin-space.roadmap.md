---
baseline_schema: "2.0"
pack: "penguin-space"
document: "roadmap"
status: "active"
updated: "2026-08-11"
code_ref: "6ee788d"
---

# Product roadmap and verification state

## Baseline conversion status

| Item | Status | Evidence |
|---|---|---|
| Preserve original proposal | Complete | `sources/PROPOSAL.md` retained a byte-identical copy before root `PROPOSAL.md` was removed |
| Core adaptive pack | Complete | Introduction, roadmap, and decision/unknowns documents created |
| Architecture and usage contract | Complete | Source-code and use-guide documents created because the proposal defines both materially |
| Root proposal deletion | Complete | Root source removed after SHA-256 equality and pack-coverage verification |
| Phase 0 — Research and decision closure | Complete with scoped deferrals | Nine decisions were resolved or explicitly deferred in the Phase 0 register; Docker-only toolchain spike passed without a host SDK |
| Product implementation | M1 bootstrap implemented; milestone remains in progress | Pinned Docker verification, frontend build, Go tests, Windows cross-build, and hidden process-start smoke test passed; real provider/elevation/package work remains |

## Phase 0 — Research and decision closure

**Depends on:** no product milestone. This phase converts the approved product direction and Docker-only build policy into evidence-backed implementation decisions. It does not implement a cleanup provider or perform destructive cleanup.

Close the following decisions with an evidence record for each: pinned container toolchain and Windows packaging feasibility; supported tool versions and provider command/API details; storage calculation/rounding; settings/history/diagnostic persistence; Wails-to-Windows elevation bridge; WSL/VHDX compaction; project-root discovery defaults; and Danger confirmation language.

**Required outputs:** a decision register containing owner, scope, current official/source evidence, test or spike result, chosen approach, rejected alternatives where safety-relevant, and follow-on implementation task. The Docker decision must also define the proposed `verify`, `build`, and `shell` Compose responsibilities and cache boundary from [the Docker build runbook](../../runbook/docker-build-environment.md).

**Acceptance criteria:** every current open decision is either closed with the required evidence or explicitly deferred with owner, reason, risk, and a blocking/non-blocking designation. The list in [penguin-space.hallucination.md](penguin-space.hallucination.md) is synchronized so it contains only genuinely open items.

**Verification gate:** no implementation milestone begins on an unverified assumption that Phase 0 identifies as blocking. A research note, a successful image build, or a static check alone is not sufficient evidence for a closed runtime decision.

**Completion record (2026-08-10):** the decision register is in [penguin-space.hallucination.md](penguin-space.hallucination.md#phase-0-decision-register). Docker Engine 29.5.3 was available through Rancher Desktop; disposable containers successfully ran Go `1.25.12`, Wails CLI `v3.0.0-beta.6` with `CGO_ENABLED=0`, and Bun `1.3.14`. Bun created a Vue 3 + TypeScript + Vite project and completed `vue-tsc -b && vite build` alongside the Go/Wails CLI checks. The generated project required an explicit `src/vite-env.d.ts` Vite/Vue declaration shim; M1 must create and retain that source file. An earlier Node/npm/Corepack probe is superseded and is not the M1 project build stack. The tested base-image digests are recorded in that register. The probe did not create an application, Dockerfile, Compose manifest, Wails app package, or Windows package; those remain Milestone 1 work.

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

**Verification gate:** a clean Windows host, without PenguinSpace Go/Wails/Bun/Windows build SDK toolchains, can run the containerized verification path; then provider and plan tests plus an end-to-end Windows run. A build alone is insufficient.

**Implementation checkpoint (2026-08-10):** M1 bootstrap is committed on `main` as `5ba3fe8`. It contains `Dockerfile`, `docker-compose.yml`, Docker-only PowerShell delegates, `go.mod`, Vue/Vite/Bun frontend sources, Wails service bindings, and a schema-versioned SQLite history store. `docker compose run --rm verify` passed Bun `1.3.14`, Go `1.25.12`, Wails CLI `v3.0.0-beta.6`, Vue type-check/build, Go vet/tests for `internal`, and a Windows `CGO_ENABLED=0` cross-compile. `docker compose run --rm build` produced `out/penguinspace.exe`; a hidden five-second Windows process-start smoke test passed and the test process was stopped. The only executable provider is an in-memory fixture: it covers scan → plan → confirmation gate → execute → verify → SQLite history and deliberately invokes no filesystem or tool command.

**Remaining M1 closure work:** fix duplicate-start UX, add controlled safe delay/timeout probe modes, then perform interactive Windows UAC refusal, cancellation, timeout, and final UI smoke tests. The UAC-success no-op probe was observed in the visible UI on 2026-08-11. Do not mark a real provider, cleanup command, installer/package, or physical reclaim outcome as implemented.

**Elevation continuation checkpoint (2026-08-11):** commit `6ee788d` adds `internal/elevation`: a versioned, fixed `m1.elevation.probe` contract, JSON request/result files below the per-user application directory, allow-list validation, cancellation marker, terminal statuses, and controller tests for success, cancellation, timeout, expired contracts, and an unknown action. The Windows-only launcher uses `ShellExecute` with the `runas` verb to re-run PenguinSpace in helper mode; the helper accepts only a request ID and executes a no-op probe. The Vue shell exposes start, status polling, and cancellation. No cleanup command, target path, provider command, or physical storage mutation can cross this bridge. Docker `verify` passed Bun `1.3.14`, Go `1.25.12`, Wails bindings for 7 service methods, Vue type-check/build, gofmt, vet, all internal tests, and a Windows cross-build. Docker `build` produced `out/penguinspace.exe` (13,544,960 bytes); a hidden five-second Windows process-start smoke test passed and the exact test process was stopped. The user then ran the visible application and supplied UI evidence of `Succeeded` for the no-op elevation probe. A repeated start request displayed `already in progress` before that first operation rendered its terminal status; the guard behaved safely, but the UI needs immediate pending-state feedback. UAC refusal, cancellation, timeout, and the remaining visible UI acceptance are unverified.

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

Fix the elevation probe UI's duplicate-start feedback and add controlled no-op delay/timeout modes; rebuild and verify in Docker, then interactively validate UAC refusal, cancellation, timeout, and the generated Windows shell. Keep the fixture non-destructive; begin M2 provider implementation only after those checks have evidence.

## Pack migration verification

Completed before root-proposal removal on 2026-08-10:

- Root and preserved source had matching SHA-256 values: `61777954f327b1157bf2c0268a3f283cda614e21ec71e70b728eb74657d4d97c`.
- Core, architecture, and use-guide pack documents were present and linked.
- The preserved source is retained unchanged; its pre-existing six Markdown line-break spaces were intentionally not normalized because lossless preservation took precedence.
