---
baseline_schema: "2.0"
pack: "penguin-space"
document: "introduction"
status: "active"
updated: "2026-08-13"
code_ref: "114a64b + M3.2 runtime evidence (no code change)"
---

# PenguinSpace baseline

## Purpose and authority

This is the active, durable documentation pack for the approved PenguinSpace product direction. It is the implementation-facing source of truth and replaces the removed root-level proposal.

The unabridged source was preserved byte-for-byte at [sources/PROPOSAL.md](sources/PROPOSAL.md). It retains every original section, example, command, table, reference, and wording; it is a migration record, not a second competing direction. The five documents in this pack turn that approved direction into resumable implementation guidance. When a summary and the preserved record appear to differ, the preserved record is authoritative until an explicit decision updates this pack.

## Product outcome

PenguinSpace is a personal, Windows-first developer storage manager. It detects developer tooling and environments, measures their storage, classifies reclaim opportunities, explains deletion consequences, plans and executes reviewed cleanup, then verifies physical bytes reclaimed.

It is not a generic PC cleaner or merely a GUI for cache-clearing commands. Its differentiators are developer-specific storage semantics, risk plus recovery-cost explanations, project-artifact discovery, and first-class Docker, WSL, and VHDX treatment.

## Decided product constraints

| Area | Decision | Status |
|---|---|---|
| Platform and stack | Windows desktop; Wails 3; Go backend; Vue 3 + TypeScript + Vite frontend | Decided / locked |
| Frontend toolchain | Bun `1.3.14` runs dependency installation and frontend scripts inside Docker; Node/Corepack are not project build-toolchain dependencies | Decided / locked; Phase 0 container spike passed |
| Delivery strategy | Build one complete Product Goal through milestones, not a disposable MVP architecture | Decided / locked |
| Interaction design | Horizontal-first, WinUI 3-inspired, dense desktop utility with a left sidebar | Decided / locked |
| Primary domains | Developer Tools, Containers & WSL, Projects, History, Settings | Decided / locked |
| Cleanup safety | Scan, measure, classify, review, confirm, execute, verify | Decided / locked |
| Cleanup implementation | Prefer official/tool-native commands; filesystem deletion only as an understood, classified fallback | Decided / locked |
| Privileges | Normal execution without Administrator rights; elevate only for operations that require it | Decided / locked |
| Build environment | Docker-only toolchain boundary: the Windows host may edit, use Git, and orchestrate Docker, but must not run or install PenguinSpace build SDKs/toolchains | Decided / locked; [runbook](../../runbook/docker-build-environment.md) |

## Current truth

- Milestone 1 bootstrap implementation is committed in `5ba3fe8`: a pinned Docker toolchain, Wails/Go/Vue shell, SQLite history store, and a fixture-only provider lifecycle.
- The M1 elevation probe is committed in `6ee788d`: it has a fixed action allow-list, per-operation Windows `runas` launcher, cancellation and timeout states, and a UI control. Docker verification, Windows cross-build, and a hidden process-start smoke test passed. On 2026-08-11 the user exercised the visible UI and observed the no-op probe reach `Succeeded`; it still has no cleanup provider or arbitrary shell-command channel.
- The root `PROPOSAL.md`, which was an already staged newly added file, was removed after byte-identical preservation in this pack.
- Product decisions are approved direction, not runtime proof. Implemented evidence includes the M1 fixture/elevation, all bounded M2 provider slices, and M3.1 read-only Docker awareness. M3.1 detects the daemon and independently reports images, stopped containers, BuildKit cache, custom networks, and volumes; it exposes no cleanup plan or mutation. Docker cleanup, WSL/VHDX operations, packaging, and release behaviour remain planned or deferred as recorded in the roadmap.
- Docker-only verification and Windows cross-build pass through Compose. The generated Windows executable launched successfully for five seconds on the host in a hidden smoke test, then was stopped; this is process-start evidence, not interactive UI acceptance.
- The fixture lifecycle scans, plans, requires confirmation, executes only an in-memory change, verifies exact reclaimed bytes, and persists a history record. It is not an implementation of any real cleanup provider or command.
- Commit `d0bc468` upgrades the no-op elevation request to contract version 2 and activates its bounded 30-second execution window only after the Windows launcher returns successfully. The helper waits for that activation, so operator consent time no longer consumes execution time; repeated regression tests cover slow consent, timeout after consent, refusal mapping, cancellation, and allow-list validation. Docker `verify` and `build` passed and produced `out/penguinspace.exe` (13,556,736 bytes; SHA-256 `f606c7a33c6ee13856db965cea2193c2e43197018fee30b6bbf078921125c4f1`). On the Windows host with UAC set to Never notify, the rebuilt executable reached `Succeeded`, `Cancelled`, and `Timed-Out`; its visible fixture lifecycle completed 3.00 MiB through scan, plan, backend confirmation, no-filesystem execution, and verification. Interactive refusal remains unverified because this host configuration auto-approves `runas` without presenting a rejectable prompt.
- Commit `1f7098c` adds three bounded M2 providers to the existing Bun, npm, conditional pnpm, and uv slices: Yarn Classic 1.x global-cache cleanup, .NET SDK 6+ NuGet HTTP-cache cleanup, and Cypress 13–15 binary-cache pruning. Each uses an inspected backend-owned plan, mandatory confirmation, tool re-detection and path revalidation, tool-native execution, logical remeasurement, history recording, and deterministic fixture coverage. Modern Yarn, NuGet global-packages/temp/plugins caches, and Cypress App Data remain outside these actions. Cargo, Gradle, Maven, and Playwright remain deferred because their official cleanup semantics need an explicit project/workspace scope, which is an M4 dependency; no broad home-cache deletion is authorized.
- Commands and external-documentation behaviour are time-sensitive and must be verified against current official sources before a provider is implemented.

## Save checkpoint — discovery-first acceptance and Docker-first boundary

**Code reference:** `d97fa27` (`feat(core,build): add provider discovery and versioned artifact output`). The statements below supersede earlier M2 status language where it conflicts.

- The discovery-first Developer Tools surface is implemented and user-confirmed on Windows through `out/penguinspace-v0.1.1.exe`: supported host providers render as full cards; pnpm without an explicit `storeDir` is a configuration notice; unavailable host tools are collapsed diagnostics; project providers are not rendered before a workspace root is approved.
- Docker verification passed Wails binding generation (11 methods/16 models), Vue type-check/build, gofmt, vet, internal tests, and the Windows production cross-build. Packaging is versioned and non-overwriting: `scripts/build-windows.ps1 -Version 0.1.1` created `out/penguinspace-v0.1.1.exe` without replacing the older artifact or unversioned compatibility executable.
- IRYS is a Docker-first Rust project. Its Cargo toolchain, manifest/build context, and build artifacts are outside the current host-workspace Cargo provider, so the absence of a Cargo card is expected and must not be interpreted as the project being non-Rust.
- The next implementation scope is M3.1 Docker awareness in read-only mode: discover daemon availability and separately report images, stopped containers, BuildKit cache, networks, and volumes. It must not execute cleanup, invoke global prune, or treat volumes as ordinary cache. Docker cleanup semantics and ownership/scoping evidence remain open.
- A host-native disposable workspace still provides optional follow-up acceptance for Cargo, Gradle, Maven, and Playwright, but it does not resolve the Docker-first IRYS case.

## Save checkpoint — M3.1 read-only Docker awareness

**Code reference:** `31bd100`; Windows acceptance artifact `out/penguinspace-v0.1.2.exe`.

- `internal/dockerinventory` uses a fixed CLI allow-list to detect the Docker daemon and inspect five separate categories: images, stopped containers, BuildKit cache, custom networks, and volumes. Disk-usage summaries are daemon-wide; counts come from category-specific list commands, and no project ownership is inferred.
- The Containers & WSL UI reports count, daemon size, and daemon-reported reclaimable size where available. Volumes are visually Stateful and remain Danger scope. No Docker cleanup button, plan, prune command, or volume mutation exists.
- Docker verification passed bindings (12 methods/19 models), frontend type-check/build, formatting, vet, internal tests, and Windows cross-build. `v0.1.2` is 13,789,696 bytes with SHA-256 `ffbe4ded8f938d9cf9ca09f3cbba883487c30a3a7712f30a75410392a53923ce`.
- User-supplied Windows screenshots complete M3.1 acceptance. With Docker 29.5.3 available, the UI showed exactly five categories, volumes as Stateful, and no cleanup control. After Rancher Desktop was stopped, refresh showed daemon unavailable and no stale resource cards. The operator later restarted Rancher for M3.2 evidence collection.
- M3.2 read-only evidence on the restarted daemon found canonical Compose project/service labels on the two toolchain images, project/resource labels on both custom networks and all ten volumes, and no labels on the Rancher proxy image. No containers existed; networks had no attachments and volumes had no mounts. The selected builder reported 40 cache records, 22 shared, without project identity.
- Decision: exact canonical Compose labels may group images, containers, networks, and volumes for observation only; missing or malformed labels are explicitly unscoped and names are never inferred as ownership. BuildKit stays selected-builder scope. Volumes remain Stateful/Danger even when labeled and unmounted.
- The next ready scope is M3.3 read-only ownership presentation. Docker cleanup semantics, image shared-layer reclaim, a stopped-container runtime fixture, and volume recovery remain open and block destructive Docker work.

## Storage model

Every discovered storage item must be represented as one of these domains, never collapsed into a generic “cache” label:

| Storage class | Meaning | Default risk / recovery cost |
|---|---|---|
| Disposable cache | Regenerable cache or metadata | Safe; usually Download |
| Rebuildable artifact | Source remains but removal causes generation, rebuild, or download | Review; Rebuild or Download |
| Stateful data | Data that may not be reproducible | Danger; State Loss |
| Virtual disk / container | Logical deletion may not reduce host allocation until compaction | Separately measured and verified |

Risk labels are **Safe**, **Review**, and **Danger**. Recovery-cost labels are **Instant**, **Download**, **Rebuild**, and **State Loss**. Safe never means cost-free; Danger actions are never preselected.

## Scope at Product Goal

- Tool/cache providers: uv, npm, pnpm, Yarn, Bun, NuGet, Cargo, Gradle, Maven, Cypress, and Playwright.
- Containers and virtualization: Docker, Docker Desktop, Rancher Desktop, WSL, and VHDX compaction.
- Project artifacts: at least `node_modules`, `target`, `.venv`, `.next`, `dist`, `build`, `.gradle`, and `.turbo`.
- Complete desktop surfaces: Home, Developer Tools, Containers & WSL, Projects, History, Settings, cleanup review/execution, and detail panes.

Detailed source material and examples are retained in [sources/PROPOSAL.md](sources/PROPOSAL.md); planned architecture is in [penguin-space.sourcecode.md](penguin-space.sourcecode.md), and the user-facing flow is in [penguin-space.useguide.md](penguin-space.useguide.md).

## Non-goals

- Browser history, Windows temporary files, registry cleaning, thumbnail cleanup, recycle-bin cleanup, and generic “PC optimization”.
- Silent deletion, automatic selection of dangerous data, or automatic installation of third-party cleanup tools.
- Vue components that contain or directly execute raw cleanup commands.

## Completed replacement record

The root `PROPOSAL.md` was removed on 2026-08-10 after the following evidence was recorded:

1. Its SHA-256 exactly matched `sources/PROPOSAL.md`: `61777954f327b1157bf2c0268a3f283cda614e21ec71e70b728eb74657d4d97c`.
2. This pack's core, architecture, and use-guide documents were present and linked.
3. The source record remains the lossless migration artifact and must be retained.
