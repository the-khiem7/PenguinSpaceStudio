---
baseline_schema: "2.0"
pack: "penguin-space"
document: "introduction"
status: "active"
updated: "2026-08-14"
code_ref: "f7189e1 (M4.3 project detail, M4.3 complete)"
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
- Product decisions are approved direction, not runtime proof. Implemented evidence includes the M1 fixture/elevation, all bounded M2 provider slices, M3.3 read-only Docker ownership presentation, M3.5 exact Compose custom-network removal, and M3.6 read-only WSL/VHDX discovery. M3.6 requires complete exact registration identity plus affirmative WSL 2 evidence and reports only handle allocation bytes as physical measurement; logical usage and compactability remain unavailable. Images, containers, BuildKit, volumes, blanket prune, WSL/VHDX mutation, packaging, and release behaviour remain blocked or planned as recorded in the roadmap.
- M4 is refined into four fixture-bounded phases. M4.1 read-only project discovery is complete with Windows acceptance, and M4.2 exact logical measurement with per-request exclusions is implemented and Docker-verified; reclaim estimates, last-used heuristics, and every project-artifact deletion remain unauthorized.
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

## Save checkpoint — M3.3 read-only ownership presentation

**Code reference:** `3141ad4` (`feat(docker): present read-only ownership scope`); Windows acceptance recorded in `f499785`.

- `DockerAwareness` now keeps daemon-wide totals separate from ID-backed ownership observations. Exact valid `com.docker.compose.project` labels form project groups; absent, malformed, or resource-type-conflicting canonical labels remain in an explicit `unscoped` group without name/tag/path inference.
- Images expose current all-container reference counts; stopped containers expose image ID plus network and mount counts; custom networks expose attachment counts; volumes expose mount counts and remain Stateful/Danger even at zero. Every relationship count has an availability flag, and a partial list/inspect result marks the ownership snapshot incomplete rather than silently converting missing data to zero or unscoped.
- BuildKit is rendered separately as `selected-builder`, with ID-backed shared/mutable/reclaimable record metadata and no project attribution. The UI provides observation tags only; no Docker cleanup, prune, delete, review, plan, or executor was added.
- Final `docker compose run --rm verify` passed Wails binding generation (12 methods, 7 enums, 25 models), Vue type-check/build, formatting, vet, all internal tests, and Windows production cross-build. A live read-only check against Docker Engine 29.5.3 observed projects `docker` and `penguinspacestudio`, one unscoped image, zero containers, two unattached custom networks, ten unmounted volumes, and 40 selected-builder records including 22 shared.
- Windows UAT on `out/penguinspace-v0.1.3.exe` (13,830,656 bytes; SHA-256 `1eec64324e6c6eb757ee19e4ae698b922998dcaafeff4d9051fbb7d27e37d0a8`) completes M3.3 acceptance. With Docker 29.5.3 available, screenshots confirmed separate daemon totals, Compose projects `docker` (8 resources) and `penguinspacestudio` (6), one explicit unscoped Rancher image, ID-backed zero relationships, ten Stateful/Danger volumes, and selected-builder metrics of 40 records/22 shared/5 mutable/40 reported reclaimable with no project attribution or cleanup controls. After Rancher stopped, Refresh reported daemon unavailable and removed every totals, ownership, unscoped, and BuildKit card without stale observations.
- M3.4 closes the next cleanup boundary: M3.5 may remove only one exact empty Compose custom network after fresh complete inspection, explicit Review confirmation, action-time revalidation, and post-removal ID verification. Images, containers, BuildKit, volumes, broad prune, and byte-reclaim claims remain blocked. M3.6 is read-only WSL/VHDX discovery only.
- This acceptance authorizes the read-only presentation and the narrowly bounded future network action only. Shared-layer reclaim, container writable-layer recovery, volume recovery, WSL/VHDX mutation, and every other destructive flow remain open.

## Save checkpoint — M3.5 exact Compose network removal

**Code reference:** `e90ceda` (`feat(docker): implement exact Compose network removal with M3.5 checkpoint`).

- One controller-owned plan may target one exact custom-network ID selected from a fresh, complete ownership snapshot. Eligibility requires canonical Compose project/network labels and exactly one available zero-attachment observation; missing rows, duplicate/unexpected inspect identities, absent/null attachment metadata, changed labels, or new attachments fail closed.
- Execution accepts only the retained plan ID plus explicit confirmation, immediately re-inspects the exact network, and can invoke only `docker network rm <ID>` without force. Independent exact-ID reconciliation handles success, verification failure, and daemon-side mutation followed by a CLI error. The outcome preserves command-attempt/completion, verified absence, refreshed awareness, history status, and any follow-up failure; reclaimed bytes are always `unavailable`.
- SQLite history schema version 2 stores reclaimed measurement kind, migrates version-1 records to `measured-logical`, and refuses newer schema versions without downgrade. A verified network outcome records provider `docker.network` with unavailable reclaimed bytes.
- Vue exposes **Review exact removal** only for an eligible network, shows the retained project/network/ID boundary and no-byte claim, and requires a second explicit confirmation. Results remain visible even if the refreshed daemon is unavailable. Images, containers, BuildKit, volumes, force, prune, bulk removal, and user-supplied commands remain absent.
- Final `docker compose run --rm verify` passed binding generation (14 methods, 7 enums, 27 models), Vue type-check/build, gofmt, vet, all internal tests, and Windows production cross-build. Deterministic fake-runner tests cover incomplete identity sets, missing attachment metadata, changed attachments, wrong/unconfirmed/consumed plans, exact commands, absence failure, mutate-then-error reconciliation with available/unavailable daemons, history failure, and prohibited force/prune commands. Final semantic review reported no Critical, High, or Medium issue.
- No existing Docker resource was removed and no reclaimed-byte or live Windows UI claim is made. M3.5 is complete; shutdown, elevation, mount, optimize, compact, or any WSL/VHDX mutation remains unauthorized.

## Save checkpoint — M3.6 read-only WSL/VHDX discovery

**Code reference:** `339f67f` (`feat(wsl): add read-only VHDX awareness with M3.6 checkpoint`).

- `internal/wslinventory` performs three fixed list-only WSL calls and correlates exact CLI identities with complete current-user registration metadata. Any registry read error, duplicate identity, case mismatch, malformed encoding, missing affirmative WSL 2 version, or unsafe path disables VHDX measurement.
- On Windows, the candidate is opened without following a reparse point. Handle attributes reject reparse points/directories, and `FILE_STANDARD_INFO.AllocationSize` supplies the only physical-byte claim; EOF, distro logical usage, compactability, and reclaimable bytes remain unavailable.
- The Containers & WSL surface is observation-only. It displays a physical size only when path evidence is available and the measurement kind is exactly `measured-physical`; mismatched or unavailable measurements display **Unavailable**. There is no WSL or VHDX action control.
- Windows-native sparse-allocation and reparse tests passed. The final Docker gate passed 15 service methods, 8 enums, 30 models, Vue type-check/build, formatting, vet, internal tests, and Windows production cross-build. Final semantic review found no Critical, High, or Medium issue.
- No installed distribution or VHDX was mutated. Live installed-WSL layout/access and Windows visual behavior remain unverified; M4 project-storage phase refinement is the next implementation action.

## Save checkpoint — M4.1 read-only project discovery

**Code reference:** `f421412` (`feat(projects): add read-only project discovery with M4.1 checkpoint`).

- M4 is refined into four fixture-bounded phases: M4.1 read-only discovery, M4.2 exact logical measurement with exclusions, M4.3 project detail plus a last-used heuristic decision, and M4.4 one reviewed exact artifact removal. A phase is authorized only by the recorded acceptance of the phase before it.
- `internal/projectinventory` implements M4.1. Projects come only from exact marker files, and a generated directory is reported only when its allow-list name is claimed by a marker in the same directory. Every measurement is `unavailable`; there is no plan, confirmation, executor, estimate, or deletion path.
- Traversal is depth-, directory-, project-, and skip-bounded, never follows a reparse point, never traverses `.git`/`.hg`/`.svn`, never descends into a claimed artifact or an unclaimed allow-list name, and re-checks the resolved path with a no-follow `Lstat` before reading it. Read failures and exhausted bounds clear completeness instead of presenting a shorter list as authoritative.
- The Vue **Projects** surface is observation-only: **Unavailable** sizes, a recorded skip list with reasons, an authoritative badge only for an approved, complete, untruncated snapshot, and no cleanup control.
- `docker compose run --rm verify` passed binding generation (16 methods, 10 enums, 34 models), Vue type-check/build, gofmt, vet, all internal tests, and the Windows production cross-build. The `projectinventory` fixtures cover nested projects, claiming priority, unclaimed and differently cased allow-list names, VCS metadata, reparse rejection including read-time revalidation, depth/directory/project/skip bounds, cancellation, an injected permission denial, and root validation; they passed repeated runs with no skipped test.
- `scripts/build-windows.ps1 -Version 0.1.4` produced `out/penguinspace-v0.1.4.exe` (13,981,696 bytes; SHA-256 `22d916a30088e62a2f9c5eaaa6b462ae96ca8b9c30ebe386ba1c978c2ae0aec3`) without replacing any earlier versioned artifact or the unversioned compatibility path. `build/config.yml` and `frontend/package.json` now both carry `0.1.4`, removing the earlier disagreement between the packaged version and the frontend manifest.
- User-supplied Windows screenshots complete M4.1 acceptance on a real approved root (`AgentMindStudio`). Discovery reported 2 marker-backed projects and 2 claimed generated directories across 100 successfully read directories. The approved root rendered as a `node` project from `package.json` with `build` (Review · Rebuild) and `node_modules` (Review · Download), both Rebuildable artifact with **Size** **Unavailable**; the nested `docs/spikes/electrobun-foundation/prototype` project rendered from `package.json` with the explicit statement that no allow-listed generated directory is claimed by its markers. The badge read **Snapshot truncated**, the summary stated that a traversal bound was reached and that skipped or unreadable paths are not reported as empty, `.git` appeared as version-control metadata, `fixtures/clients/kilo/instruction/valid/.kilo/rules` appeared as the 6-level depth bound, exactly one depth warning was emitted, and no cleanup, review, plan, or delete control existed anywhere on the surface.
- No project file or directory was created, modified, or removed by discovery or by this acceptance run. M4.1 is complete; M4.2 measurement is authorized as the next phase but is not implemented, and reclaim estimates, last-used ranking, and every project-artifact deletion remain unauthorized.

## Save checkpoint — M4.2 project storage measurement

**Code reference:** `9a90be9` (`feat(projects): measure project artifacts with validated exclusions`).

- Exclusion sourcing is decided: rules are proposed per request, validated against the approved root, applied to that one measurement, and never persisted. A workspace ignore file and persisted settings are deferred to M5.
- `Inspector.MeasureProject` measures one discovered project's claimed artifacts as exact logical bytes. The project and its artifacts are re-derived from a fresh discovery pass, so an arbitrary path, an artifact path, a marker-less directory, and an unapproved root are all rejected, and an unfinished or incomplete discovery is reported as such rather than as a missing project.
- Safety precedes scope: a reparse point is recorded as one even when an exclusion covers it, and unreadable paths and non-regular entries keep their own kinds. An excluded or unreadable artifact reports `unavailable` instead of a measured zero and is left out of the project total with a warning. Any exclusion, safety skip, budget exhaustion, depth bound, or overflow clears completeness, and reclaimable bytes stay `unavailable` everywhere.
- Bounds are fixed: a 400,000-entry budget, a 64-level measurement depth bound, per-artifact skip caps, and overflow checks on both per-artifact and per-project sums.
- `docker compose run --rm verify` passed bindings (17 methods, 10 enums, 37 models), Vue type-check/build, gofmt, vet, all internal tests, and the Windows production cross-build. Fixtures cover exact counts, exclusion behaviour and reporting, unsafe and overlapping rules, safety-over-exclusion ordering, sparse-file logical size, non-regular and unreadable paths, the entry budget, byte overflow, cancellation, and incomplete-discovery attribution. A semantic review reported no Critical issue and its High findings were fixed before this checkpoint.
- `out/penguinspace-v0.1.5.exe` (14,018,048 bytes; SHA-256 `6fb663910384769bf49d9a7bb8a90bcb1057e42cf46e496ed0a36a903af14190`) was produced without replacing any prior artifact and reproduced bit-for-bit by an independent container build. It has not been launched, so Windows visual acceptance of measurement remains open, and M4.3 is not authorized by this checkpoint.

## Save checkpoint — M4.3 measurement cancellation

**Code reference:** `1ce9da1` (`feat(projects): support cancelling an in-flight measurement`).

- `MeasureProject` accepts an optional cancel signal. Cancelling from another goroutine stops the walk before the next directory descent or within 2,000 entries inside one large listing, and the check runs before the deadline and budget checks so a simultaneous cancel is never misreported as a timeout or a budget exhaustion.
- A cancelled measurement returns `Cancelled: true` with no error and both `Complete` and `Truncated` false; bytes gathered before the stop are exact, and anything never read stays `unavailable`.
- `AppService` serializes measurement and tracks the active cancel signal under a separate mutex so `CancelProjectMeasurement` never blocks on the in-flight call and never reaches a stale signal from a just-finished measurement.
- Vue adds **Cancel measurement** beside the measure control and a distinct **Cancelled, partial** label.
- `docker compose run --rm verify` passed bindings (18 methods, 10 enums, 37 models), Vue type-check/build, gofmt, vet, and all internal tests; cancellation fixtures passed under `-race`. A semantic review reported no High or Critical issue; its Medium tie-break findings and Low dead-code findings were fixed before this checkpoint.
- Project detail remains open for M4.3. No filesystem path was created, modified, or removed, and this change has not been exercised through the Windows UI.

## Save checkpoint — M4.3 last-used heuristic decision

**Code reference:** `eddf959` (no code changed by this decision; documentation only).

- Access time is never shown. NTFS disables last-access-time updates by default since Windows Vista, and re-enabling it needs an elevated, rebooted registry change PenguinSpace does not perform, so atime cannot be trusted on a real host.
- The "last used" concept is renamed to **Last modified** and scoped to the artifact root directory's own modification time only, already available from the `Lstat` performed during discovery/measurement; no additional traversal is added. A max-mtime scan across artifact contents was rejected because it reintroduces the recursive cost M4.2 already separated out, for a value that would still mean modification, not usage.
- The value must always be shown with a disclosure that it reflects modification, not reading or use, and it must never drive sorting, ranking, "abandoned" labeling, preselection, or any cleanup plan.
- A tool-log-derived usage signal and an in-app "last inspected" signal are explicitly deferred, each pending its own decision record.
- This is a documentation-only decision; no code changed. Project detail implementation, which will consume this decision, remains open for M4.3.

## Save checkpoint — M4.3 Last modified implementation

**Code reference:** `2cb0989` (`feat(projects): show Last modified time per decided heuristic`).

- `LastModified` is added to project and artifact observations in both discovery and measurement, sourced only from stat metadata already obtained during the existing walk; measurement carries the value over rather than re-deriving it.
- `TimeObservation.Value` is a pointer so an unavailable observation's JSON omits the field entirely rather than serializing a zero date that would render as a real, wrong value.
- The fixed modification-not-usage disclosure sentence accompanies every rendering of the value in Vue; nothing sorts, ranks, badges, or preselects on it, honoring the decision above.
- `docker compose run --rm verify` passed bindings (18 methods, 10 enums, 38 models), Vue type-check/build, gofmt, vet, and all internal tests. A semantic review found no Critical issue; its High and Medium findings were fixed before this checkpoint.
- No filesystem path was created, modified, or removed, and this has not been exercised through the Windows UI. Project detail remains the one open part of M4.3.

## Save checkpoint — M4.3 project detail (M4.3 complete)

**Code reference:** `f7189e1` (`feat(projects): add shared project detail panel for M4.3`).

- The Projects surface now shows a compact summary per project plus one shared detail panel for whichever project is selected; the exclusion textarea, Measure/Cancel controls, and measurement result live only in that panel, scoped to the selected project's path.
- Switching projects mid-measurement can no longer leak a stale result or error into the wrong panel; a background measurement for another project stays cancellable and visibly explained from wherever the user is looking.
- Frontend-only change; no backend, model, or binding was touched. `docker compose run --rm verify` passed unchanged bindings (18 methods, 10 enums, 38 models) plus Vue type-check/build. A semantic review's one High and two Medium findings were fixed before this commit.
- This completes M4.3. M4.4 — one reviewed exact project-artifact removal — is the next milestone phase; it remains unauthorized until its own implementation and evidence.
- `scripts/build-windows.ps1 -Version 0.1.6` produced `out/penguinspace-v0.1.6.exe` (14,029,312 bytes; SHA-256 `c26bc5b0a792881e319f70574df78cab6f7c969c55ff732fc07dbaee99e9030b`) without replacing any prior artifact; `build/config.yml` and `frontend/package.json` are aligned at `0.1.6`. This build packages the full M4.2 measurement surface and the complete M4.3 work (cancellation, Last modified, project detail) for the pending Windows UAT; the executable has not yet been launched by the operator.

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
