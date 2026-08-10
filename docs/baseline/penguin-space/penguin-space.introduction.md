---
baseline_schema: "2.0"
pack: "penguin-space"
document: "introduction"
status: "active"
updated: "2026-08-10"
code_ref: "worktree-m1"
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

- Milestone 1 bootstrap implementation exists in the uncommitted worktree: a pinned Docker toolchain, Wails/Go/Vue shell, SQLite history store, and a fixture-only provider lifecycle.
- The root `PROPOSAL.md`, which was an already staged newly added file, was removed after byte-identical preservation in this pack.
- The product decisions recorded in this pack are approved direction, not proof that a provider, UI, command, integration, cleanup, or VHDX operation currently works.
- Docker-only verification and Windows cross-build pass through Compose. The generated Windows executable launched successfully for five seconds on the host in a hidden smoke test, then was stopped; this is process-start evidence, not interactive UI acceptance.
- The fixture lifecycle scans, plans, requires confirmation, executes only an in-memory change, verifies exact reclaimed bytes, and persists a history record. It is not an implementation of any real cleanup provider or command.
- Real provider integrations, Windows elevation/runtime behavior, package/installer creation, and full interactive UI QA remain unverified.
- Commands and external-documentation behaviour are time-sensitive and must be verified against current official sources before a provider is implemented.

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
