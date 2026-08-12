---
baseline_schema: "2.0"
pack: "penguin-space"
document: "roadmap"
status: "active"
updated: "2026-08-12"
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
| Phase 0 — Research and decision closure | Complete with scoped deferrals | Nine decisions were resolved or explicitly deferred in the Phase 0 register; Docker-only toolchain spike passed without a host SDK |
| Product implementation | M1 complete with two scoped environment deferrals; M2 in progress | Bun 1.x, npm 10/11, and uv 0.12.x cover bounded complete provider lifecycles; pnpm 11/12 adds an explicit-store-only lifecycle and truthful unavailable estimate, while drive-aware default-store discovery and the remaining providers are not implemented |

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

**Completion decision (approved 2026-08-12):** M1 is complete with two explicit, non-blocking deferrals. Interactive UAC refusal moves to M3 and becomes blocking before any real privileged provider or VHDX action is accepted. Clean-host Docker verification moves to M6 and becomes blocking before packaging or release. Timing separation, regression tests, Docker verification/build on the current host, auto-consent success, cancellation, timeout, and final fixture/UI smoke are complete. These deferrals do not constitute runtime evidence and do not authorize a real cleanup provider, installer/package, or physical-reclaim claim.

**Elevation continuation checkpoint (2026-08-11):** commit `6ee788d` adds `internal/elevation`: a versioned, fixed `m1.elevation.probe` contract, JSON request/result files below the per-user application directory, allow-list validation, cancellation marker, terminal statuses, and controller tests for success, cancellation, timeout, expired contracts, and an unknown action. The Windows-only launcher uses `ShellExecute` with the `runas` verb to re-run PenguinSpace in helper mode; the helper accepts only a request ID and executes a no-op probe. The Vue shell exposes start, status polling, and cancellation. No cleanup command, target path, provider command, or physical storage mutation can cross this bridge. Docker `verify` passed Bun `1.3.14`, Go `1.25.12`, Wails bindings for 7 service methods, Vue type-check/build, gofmt, vet, all internal tests, and a Windows cross-build. Docker `build` produced `out/penguinspace.exe` (13,544,960 bytes); a hidden five-second Windows process-start smoke test passed and the exact test process was stopped. The user then ran the visible application and supplied UI evidence of `Succeeded` for the no-op elevation probe. A repeated start request displayed `already in progress` before that first operation rendered its terminal status; the guard behaved safely, but the UI needs immediate pending-state feedback. UAC refusal, cancellation, timeout, and the remaining visible UI acceptance are unverified.

**M1 interaction follow-up checkpoint (2026-08-12):** commit `8fc559a` contains the fixed three-value `ProbeMode` allow-list: immediate `consent`, delayed `cancellation-test`, and `timeout-test` whose delay equals the controller's fixed 30-second deadline. Vue sets `elevationStarting` before its bridge call resolves, disables all start buttons while pending/active, and uses the generated Wails enum, so it cannot initiate a duplicate request or drift from the backend contract. Docker `verify` passed Wails bindings (5 enums), Vue type-check/build, gofmt, vet, every internal test, and Windows cross-build. Docker `build` produced `out/penguinspace.exe` (13,545,984 bytes). No new Windows runtime evidence exists yet. Code inspection on 2026-08-12 found that `NewRequest` sets `ExpiresAt` before `Launcher.Launch` waits for Windows consent; the controller creates its timer only afterward but uses the already-running absolute expiry. This can turn a slow consent response into a false timeout and must be corrected before acceptance. The operator explicitly allows long UAC response time and will assist when prompted; automation must wait rather than cancel the acceptance run.

**M1 timing and Windows acceptance checkpoint (2026-08-12):** commit `d0bc468` replaces pre-consent absolute expiry with a two-phase request. `Controller.run` activates and atomically persists `ExecutionStartedAt` plus `ExecutionDeadline` only after `Launcher.Launch` succeeds; an early helper waits for activation. The timeout probe deliberately outlives the execution window by 250 ms, removing the equality race, and the controller rechecks terminal helper status at its deadline. Focused elevation tests passed 20 consecutive runs, including simulated 80 ms consent waits against 40 ms execution windows, timeout-after-consent, refusal-to-`failed`, cancellation, and allow-list validation. Full Docker `verify` passed Bun `1.3.14`, Wails binding generation, Vue type-check/build, gofmt, vet, all internal tests, and Windows cross-build. Docker `build` produced `out/penguinspace.exe` (13,556,736 bytes; SHA-256 `f606c7a33c6ee13856db965cea2193c2e43197018fee30b6bbf078921125c4f1`). Computer-controlled Windows acceptance observed `Succeeded`, `Cancelled`, and `Timed-Out`, then a visible fixture run reported 3.00 MiB estimated and verified with no filesystem command. This host uses UAC Never notify, so `runas` auto-approved and no refusal prompt could be produced; refusal remains the only open M1 runtime gate.

## Milestone 2 — Developer tool ecosystems

**Depends on:** Phase 0 provider research and Milestone 1 provider contract.

Implement detection, inspection, estimate, plan, cleanup, and verification for Node.js (npm, pnpm, Yarn, Bun), Python (uv), .NET (NuGet), Rust (Cargo), Java/JVM (Gradle, Maven), and Cypress/Playwright.

**Acceptance criteria:** each provider has version-aware semantics, a chosen official source/API or justified fallback, risk/recovery classification, user consequence text, and a verification strategy.

**Verification gate:** provider tests and a Windows test environment containing representative tools. Never infer support from command strings alone.

**Start decision (approved 2026-08-12):** M2 may proceed because its provider semantics, detection, inspection, estimates, plans, and fixture verification do not depend on either deferred environment gate. Begin with the Bun cache provider as the first complete slice, then reuse the proven contract for npm, pnpm, Yarn, uv, NuGet, Cargo, Gradle, Maven, Cypress, and Playwright. A provider must not expose real cleanup until its current official semantics and representative runtime behavior have evidence.

**Bun provider checkpoint (working tree, 2026-08-12):** `internal/core` now has a generic provider contract, measurement kinds, backend inspection, and cleanup-outcome models. `internal/providers/bun` supports Bun major 1, treats unknown majors as detected but unsupported, resolves the global cache using Bun's official command from an isolated temporary manifest context, rejects relative/root/home targets, skips symlinks during exact logical measurement, classifies the action Safe with Download recovery cost, and explains that hardlinks can reduce physical reclaim. `AppService` retains the issued plan; execution cannot accept a frontend target, requires explicit confirmation, detects Bun again, rejects a changed cache path, invokes `bun pm cache rm`, remeasures logical bytes, and records history. Final Docker `verify` passed binding generation for 9 service methods and 14 models, Vue type-check/build, all Go tests, and Windows cross-build; focused Bun/core tests passed 20 repeated runs. Final Docker `build` produced `out/penguinspace.exe` (13,599,744 bytes; SHA-256 `6261a9697cbdc3a1179447e655adc62da9e5a9f6747c040b7963dd6b36121243`). A disposable Windows cache forced through `BUN_INSTALL_CACHE_DIR` reported the exact 4 KiB fixture path, was cleared by Bun 1.3.13, and left no payload or fixture directories. The rebuilt UI detected Bun 1.3.13, measured 127 MiB, displayed Safe/Download consequence text, opened the explicit confirmation gate, and cancelled with the 127 MiB measurement intact. The real user cache was not deleted. This completes only the Bun slice, not M2.

**npm provider checkpoint (working tree, 2026-08-12):** `internal/providers/common` now centralizes safe storage-root validation, symlink avoidance, overflow checks, measurement, and system command execution; Bun was migrated onto it. `internal/providers/npm` supports npm majors 10 and 11, resolves configured scope with `npm config get cache`, measures only `_cacache`, and creates a Review/Download action for `npm cache clean --force`. A first Windows fixture deliberately placed data at cache root and proved npm did not delete it while `_logs` remained; this material failure changed the implementation from misleading whole-root measurement to `_cacache` only. A corrected 4 KiB `_cacache` fixture was removed by npm 11.8.0, while 2,039 unmeasured log bytes remained exactly out of scope. Docker `verify` passed all providers, Vue component extraction, generated bindings, and Windows cross-build. Docker `build` produced `out/penguinspace.exe` (13,619,712 bytes; SHA-256 `fc757aa9bdba423f2f3023081cee4079b58b94561f9b79bcfac77266a5d05d6e`). Windows UI acceptance detected npm 11.8.0, measured 87.3 MiB, rendered Review/Download and the explicit `--force` consequence, then cancelled with the measurement intact. The reusable `ProviderCard` and generic backend registry now avoid one-off Bun service/UI paths. The real npm cache was not deleted. M2 remains in progress.

**pnpm provider checkpoint (working tree, 2026-08-12):** `internal/providers/pnpm` supports majors 11 and 12, but the first Windows acceptance exposed two assumptions that had to be removed. Running pnpm in the process working directory failed with `EPERM` when the desktop inherited a protected WindowsApps directory, so every pnpm probe now uses `--dir` with an owned temporary directory. Resolving a default store from that unrelated directory then selected the C: store even though this D: project uses `D:\.pnpm-store\v11`; official pnpm settings confirm that default stores are per disk. M2 therefore detects pnpm without creating an action unless an explicit absolute `storeDir` is configured. For an explicit store, the provider measures total observed logical bytes but emits an `unavailable` reclaim estimate because `store prune` removes only unreferenced packages. It retains the plan, revalidates the store path, invokes `pnpm store prune`, remeasures, and records the actual before/after difference. A disposable pnpm 11.3.0 Windows store shrank from 21,903 to 12,288 bytes, verifying 9,615 bytes removed; the user's C: and D: stores were untouched. Windows UI acceptance detected pnpm 11.3.0 and exposed no review/confirm action for the implicit per-disk configuration. Focused pnpm/core tests passed 20 repeated runs. Final Docker `verify` passed binding generation, Vue type-check/build, all provider tests, and Windows cross-build; Docker `build` produced `out/penguinspace.exe` (13,637,632 bytes; SHA-256 `c28428ce56169f13175ab00faa541292c00e7dbbf0facb993db44031fc1be4f0`). Drive-aware default-store discovery belongs with M4 project roots rather than being guessed in M2. M2 remains in progress.

**uv provider checkpoint (uncommitted, 2026-08-12):** `internal/providers/uv` supports uv 0.12.x, runs probes from an owned temporary `--directory`, resolves the cache with `uv cache dir`, retains a backend-owned plan, revalidates the path, invokes only `uv cache prune`, and records verified before/after bytes. Initial recursive filesystem measurement exceeded the 30-second UI budget on the real cache; this material failure changed inspection to official `uv cache size --preview-features cache-size` raw bytes. Total cache size is therefore observed, while the pre-prune reclaim estimate remains unavailable. A disposable boundary fixture removed exactly 4,096 bytes inside the uv cache while preserving a 2,048-byte sentinel outside it. Windows UI acceptance detected uv 0.12.1, measured 3.45 GiB, rendered Safe/Download consequences including centralized project-environment recreation, opened review, and cancelled; the real user cache was not pruned. That run briefly exposed Windows Terminal, so the common command runner now applies `CREATE_NO_WINDOW` on Windows. Focused tests passed 20 repeated runs; Windows compile-only checks and full Docker `verify` passed binding generation for 9 service methods, 6 enums, and 14 models, Vue build, all internal provider tests, and Windows cross-build. Docker `build` produced `out/penguinspace.exe` (13,655,040 bytes; SHA-256 `ea6416d9a68f63d429c150d3903e556738ff945d36e1c85dcf2381bcfbf46e6b`). The final runtime recheck of console suppression was interrupted before launch, so this one UX observation remains unverified. M2 remains in progress.

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
| Only Bun, npm, conditional pnpm, and uv cleanup providers exist | Their evidence may be generalized to unimplemented ecosystems, pnpm/uv total size may be mistaken for reclaimable size, or logical bytes may be presented as physical reclaim | Keep every remaining provider planned until its own evidence exists; retain observed, unavailable-estimate, logical, and physical measurement distinctions |

## Exact next action

Launch the freshly rebuilt `out/penguinspace.exe`, open Developer Tools, and run **Inspect uv cache** once. Confirm that uv 0.12.1 still reports the expected 3.45 GiB-class measurement and, critically, that no Windows Terminal or console window flashes after the `CREATE_NO_WINDOW` patch. Open the uv review, cancel it, and confirm the measurement remains while no prune runs. Record that runtime result before selecting the next M2 provider; do not touch the real uv cache.

## Pack migration verification

Completed before root-proposal removal on 2026-08-10:

- Root and preserved source had matching SHA-256 values: `61777954f327b1157bf2c0268a3f283cda614e21ec71e70b728eb74657d4d97c`.
- Core, architecture, and use-guide pack documents were present and linked.
- The preserved source is retained unchanged; its pre-existing six Markdown line-break spaces were intentionally not normalized because lossless preservation took precedence.
