---
baseline_schema: "2.0"
pack: "penguin-space"
document: "sourcecode"
status: "draft"
updated: "2026-08-10"
code_ref: "uncommitted"
---

# Intended architecture

No source code exists at this baseline. This document describes the **planned** topology drawn from the approved proposal; it is not code-inspected architecture.

```mermaid
flowchart LR
  UI[Vue desktop UI] --> Domain[Go domain models]
  Domain --> Detect[Detection and scanner engine]
  Domain --> Provider[Provider implementations]
  Provider --> Plan[Cleanup planning engine]
  Plan --> Review[Review and confirmation]
  Review --> Execute[Cleanup executor]
  Execute --> Verify[Verification engine]
  Verify --> History[History and diagnostics]
  Provider --> Native[Official CLIs or APIs]
  Detect --> Files[Windows filesystems, projects, Docker, WSL]
```

## Boundary rules

- Vue renders provider, category, action, risk, recovery-cost, size, progress, and history domain models. It does not own raw shell commands.
- A provider owns detection, inspection, estimate, plan, execution, verification, version-aware semantics, and user-facing consequence data for one ecosystem.
- The planning engine creates explicit reviewable actions before any destructive operation begins.
- The executor handles cancellation, timeout, logs, errors, elevated operations, and partial completion.
- The verification engine remeasures provider state and physical disk state when meaningful, then records actual outcomes.
- Docker resource classes and virtual disks are not treated as interchangeable cache locations.

## Build-environment boundary

All project dependency installation, code generation, verification, tests, builds, and packaging are planned to execute inside Docker. The Windows host may run the editor, Git, Rancher Desktop/Docker, and `docker compose` only as the orchestrator; it must not provide a Go, Wails, Node/Vue, or Windows packaging toolchain for PenguinSpace.

The planned Compose contract has three responsibilities: `verify` for locked dependency installation and static/test checks, `build` for the Windows package, and `shell` for disposable diagnostics. Toolchain and dependency caches belong to container-managed paths or Docker named volumes, never host SDK/cache bind mounts. A deliberately exported installer may use `out/` as a release artifact location.

## Provider lifecycle

```text
Detect → Inspect → Measure → Classify → Estimate → Plan → Confirm → Execute → Verify → Record history
```

The UI can display a plan only after classification. A plan must expose provider/item, current and estimated reclaimable size, risk, recovery cost, consequence, privilege requirement, shutdown prerequisite, and verification approach.

## Required core models

- Provider and detected environment
- Storage item and storage class
- Cleanup action and cleanup plan
- Risk and recovery-cost enums
- Measurement, estimate, logical deletion result, and physical reclamation result
- Execution status, progress, cancellation/error outcome, log/diagnostic context
- Scan result and cleanup-history record
- Workspace root, exclusions, and scanner rule

## Docker and WSL design constraints

Docker inspection and cleanup must distinguish build cache, images, stopped containers, networks, and volumes. Volumes are persistent-state candidates and therefore Danger.

WSL/VHDX needs a separate measurement path: distro/filesystem logical usage, backing VHDX path, physical size, estimated compactability, and physical size after compaction. Logical deletion is never evidence of host-disk reclaim until post-operation verification.

## Proposed UI architecture

The app shell contains the left navigation and compact/responsive behavior. Domain pages use metrics, horizontal breakdowns, dense tables/lists, master-detail interactions, detail panes, review dialogs, progress, and contextual warning/confirmation surfaces. Design details are in [penguin-space.useguide.md](penguin-space.useguide.md).
