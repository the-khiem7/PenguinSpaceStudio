# PenguinSpace — Product Proposal & Source of Truth

> **Status:** Approved product direction  
> **Purpose:** This document is the canonical source of truth for humans and coding agents implementing PenguinSpace.  
> **Target platform:** Windows desktop  
> **Primary stack:** Wails 3 + Go + Vue  
> **Product type:** Personal developer-experience (DX) project  
> **Design direction:** Horizontal-first, WinUI 3-inspired desktop UI  
> **Development strategy:** Build toward the complete Product Goal through explicit milestones. This is **not** an MVP-first product plan.

---

## 1. Executive Summary

**PenguinSpace** is a Windows-first developer storage manager.

It helps developers understand and reclaim disk space consumed by development environments such as package-manager caches, build artifacts, browser-testing binaries, Docker, WSL virtual disks, and project-local generated data.

The product exists because developer machines accumulate many different kinds of storage over time, while every ecosystem exposes different cleanup commands, storage locations, retention behavior, and risk levels.

The core problem is **not merely that developers forget cleanup commands**.

The deeper problem is:

> Developers can see disk space disappearing, but determining **what is consuming it**, **what is actually reclaimable**, **what is safe to delete**, **what will need to be downloaded or rebuilt afterward**, and **how to reclaim the physical disk space** requires tool-specific knowledge.

PenguinSpace provides one Windows-native-style GUI that:

1. Detects installed developer tools and environments.
2. Measures their storage footprint.
3. Distinguishes caches, rebuildable artifacts, stateful data, and virtual disks.
4. Estimates reclaimable space.
5. Explains the consequence and risk of each cleanup action.
6. Uses official/tool-native cleanup mechanisms where possible.
7. Executes cleanup only after review.
8. Verifies how much physical space was actually reclaimed.
9. Handles Docker/WSL/VHDX storage as a first-class domain rather than treating it as generic cache.
10. Scans project-local artifacts such as `node_modules`, `.venv`, `target`, `.next`, and build outputs.

A useful one-line product definition is:

> **A Windows-first storage manager that understands developer toolchains, Docker, WSL, and project artifacts.**

Potential tagline:

> **PenguinSpace — Reclaim your dev space.**

---

# 2. Locked Product Decisions

The following decisions have been explicitly approved and should be treated as **LOCKED** unless the owner intentionally changes them.

## 2.1 Technology

**LOCKED**

- Desktop framework: **Wails 3**
- Backend language: **Go**
- Frontend framework: **Vue**
- Target OS: **Windows**
- UI visual direction: **WinUI 3-inspired**
- This is a **personal project**
- Do not optimize the product around a temporary MVP architecture.

Wails 3 is intentionally selected even though the project began from a Wails/Go + Vue concept. The final implementation target is Wails 3.

---

## 2.2 Product Development Strategy

**LOCKED**

PenguinSpace will be designed for the **complete Product Goal from the beginning**.

There is no conceptual split where:

- MVP has a disposable architecture.
- Production is redesigned later.

Instead:

- The final domain model and architecture should be established early.
- Development happens through milestones.
- Each milestone incrementally completes the final product.
- The application is considered to have reached the Product Goal only after all production milestones are completed.

---

## 2.3 UI Direction

**LOCKED**

PenguinSpace is a desktop Windows utility and therefore must be designed **horizontal-first / landscape-first**.

The UI must **not** look like:

- a mobile layout stretched onto desktop,
- a vertically stacked SaaS dashboard,
- an antivirus/cleaner product full of aggressive CTA banners,
- a generic web admin dashboard.

The UI should feel visually familiar to Windows users.

The desired visual language is inspired by:

- Windows 11 Settings,
- WinUI 3,
- Microsoft Dev Home,
- PowerToys.

PenguinSpace does **not** need to mimic WinUI 3 exactly. The requirement is stylistic familiarity with modern Windows apps.

---

# 3. Original Pain Points

Developer tools frequently accumulate significant storage after being used for some time.

Examples identified for PenguinSpace include:

## 3.1 Python / AI Tooling

### uv

Possible stored data:

- package cache,
- wheels,
- downloaded package artifacts,
- metadata.

Originally identified cleanup command:

```bash
uv cache clean
```

Preferred routine behavior should consider:

```bash
uv cache prune
```

because routine pruning is less aggressive than completely resetting the cache.

---

## 3.2 Node.js Ecosystem

### npm

Originally identified:

```bash
npm cache clean --force
```

PenguinSpace should **not** present this as a normal Safe Clean action because npm treats its cache as self-healing and full cache resets are usually unnecessary.

### pnpm

Preferred command:

```bash
pnpm store prune
```

This is a good example of the type of cleanup PenguinSpace should favor: allow the package manager itself to decide what is unused.

### Yarn

Originally identified:

```bash
yarn cache clean
```

Exact behavior may differ by Yarn major version and must be handled through the provider implementation rather than assuming one static command forever.

### Bun

Originally identified:

```bash
bun pm cache rm
```

### Cypress

Originally identified:

```bash
npx cypress cache clear
```

Preferred routine behavior should use pruning when possible:

```bash
cypress cache prune
```

Full clear should exist only as a more aggressive action.

### Playwright

Originally identified:

```bash
npx playwright uninstall --all
```

This is **not equivalent to ordinary cache cleanup**.

It removes Playwright browser binaries and therefore must be classified as a rebuild/download-cost action rather than Safe Cache Clean.

---

## 3.3 .NET

### NuGet

Originally identified:

```bash
dotnet nuget locals all --clear
```

PenguinSpace should expose NuGet storage categories separately when possible rather than treating `all` as one generic safe action.

Potential categories:

- HTTP cache,
- temp,
- plugin cache,
- global packages.

Clearing global packages causes dependencies to be restored later and therefore has a higher recovery cost.

---

## 3.4 Rust

### Cargo

Originally identified cleanup flow:

```bash
cargo install cargo-cache
cargo-cache --remove-dir git-repos,registry-sources,registry-crate-cache
cargo clean
```

Important product decision:

- Do **not** automatically install third-party `cargo-cache`.
- Modern Cargo already provides garbage collection behavior for global caches.
- `cargo clean` mainly targets project build artifacts such as `target/`.
- Global Cargo cache management and project `target/` cleanup are different domains and should not be conflated.

---

## 3.5 Java / JVM

### Gradle

Originally identified:

```bash
rm -rf ~/.gradle/caches/
```

PenguinSpace must not treat deleting the entire Gradle cache directory as routine Safe Clean.

Modern Gradle has its own cache-cleanup behavior.

A complete reset may still exist as an advanced action.

### Maven

Originally identified:

```bash
rm -rf ~/.m2/repository/
```

This should **not** be a default Safe Clean action.

The Maven local repository may contain a large dependency set and potentially locally installed artifacts.

Where possible, prefer scoped/tool-native purge mechanisms such as Maven dependency-plugin workflows rather than blindly deleting the entire repository.

---

## 3.6 Containers / WSL

### Docker

Originally identified:

```bash
docker system prune -a --volumes
```

This command is **too aggressive to expose as a default cleanup button**.

It can remove:

- unused images,
- stopped containers,
- build cache,
- unused networks,
- unused volumes when `--volumes` is included.

Docker volumes may contain local databases or other stateful development data.

PenguinSpace must therefore represent Docker sub-resources independently.

### WSL / VHDX

Originally identified manual flow:

```text
wsl --shutdown
diskpart
attach vdisk readonly
compact vdisk
detach vdisk
exit
```

This is one of the strongest reasons for PenguinSpace to exist.

A developer may clean data *inside* Docker or WSL and still see little or no physical disk space returned to the Windows host because the dynamically expanding virtual disk does not necessarily shrink automatically.

PenguinSpace should make that distinction visible and provide an explicit VHDX compaction workflow.

---

# 4. Refined Problem Statement

The original pain point was:

> Developers must constantly ask AI for the correct command to clean caches.

That remains true, but PenguinSpace should solve the deeper issue.

## Final Problem Statement

Developer environments accumulate:

- caches,
- downloaded dependencies,
- browser binaries,
- build artifacts,
- project-local generated folders,
- container images,
- build cache,
- container volumes,
- WSL filesystems,
- dynamically expanding VHDX files.

These resources are spread across unrelated tools, have different cleanup semantics, and carry different risks.

Developers often do not know:

- where the data is located,
- how large it is,
- whether it is reclaimable,
- whether it is safe to remove,
- whether removing it requires re-download,
- whether removing it requires a costly rebuild,
- whether removing it can destroy state,
- whether cleanup will actually reduce physical disk usage,
- which command should be used,
- whether that command changed between tool versions.

PenguinSpace centralizes this knowledge into one desktop utility.

---

# 5. Product Positioning

PenguinSpace should **not** position itself as:

> A GUI wrapper around cache-cleaning commands.

That positioning is too narrow and easily copied.

The preferred positioning is:

> **Windows-first developer storage manager.**

It is specifically designed for developer storage rather than general consumer PC cleaning.

PenguinSpace is **not** primarily concerned with:

- browser history,
- Windows temporary files,
- registry cleaning,
- thumbnails,
- recycle bin cleaning,
- generic “PC optimization”.

It is concerned with questions such as:

> “What developer environment consumed 80 GB of my disk?”

> “How much of Docker is actually reclaimable?”

> “Why did I delete 30 GB inside WSL but my Windows free space barely change?”

> “Which old projects contain huge build outputs or dependency folders?”

---

# 6. Competitive / Market Research Notes

The pain point is validated by existing developer-oriented cleanup tools.

The market already contains utilities that scan developer caches and artifacts across ecosystems.

Examples discussed during research include:

- **ClearDisk** — developer cache/storage cleaner, especially macOS-oriented.
- **DevCleaner** — developer storage cleanup with explicit Safe / Warning / Danger concepts.
- **Mac Dev Cleaner** — particularly relevant because it uses a Go + Wails architecture and scans developer ecosystems.
- **WinDiskCleanup** and other Windows cleanup utilities — provide some package-manager or WSL/Docker-related cleanup but are often CLI/TUI-oriented or broader system-cleaner tools.

This means PenguinSpace should not compete purely on:

> “I support npm, Cargo, Docker, etc.”

The strongest differentiation should be:

1. Windows-first experience.
2. Native-looking desktop UI.
3. Developer-specific storage model.
4. Risk and recovery-cost explanations.
5. Docker + WSL + VHDX awareness.
6. Project artifact discovery.
7. Physical-space verification after cleanup.

---

# 7. Storage Domain Model

A major product insight is that the application must not treat every item as “cache”.

At minimum, PenguinSpace must distinguish the following storage classes.

## 7.1 Disposable Cache

Examples:

- package-manager caches,
- unused package-store entries,
- old Cypress versions,
- temporary metadata caches.

Characteristics:

- generally safe to regenerate,
- low risk,
- may require later network download.

Typical classification:

> **Risk: Safe**

---

## 7.2 Rebuildable Artifact

Examples:

- Cargo `target`,
- `.next`,
- `dist`,
- build outputs,
- Playwright browser binaries,
- some project-local generated files.

Characteristics:

- source code is not lost,
- cleanup can cause expensive rebuilds,
- may require network or CPU time.

Typical classification:

> **Risk: Review**

---

## 7.3 Stateful Data

Examples:

- Docker volumes,
- development databases,
- important WSL files,
- certain locally installed Maven artifacts.

Characteristics:

- may not be reproducible,
- cleanup can cause data loss.

Typical classification:

> **Risk: Danger**

Dangerous cleanup must never be automatically selected.

---

## 7.4 Virtual Disk / Storage Container

Examples:

- WSL `.vhdx`,
- Docker Desktop virtual disk,
- Rancher Desktop virtual disk.

Characteristics:

- physical host storage may remain allocated after logical data is deleted,
- compaction may be required to reclaim host disk space.

This domain needs separate metrics:

- logical used space,
- physical VHDX size,
- estimated compactable space,
- actual space reclaimed after compaction.

---

# 8. Risk Model

Every cleanup action should have a risk classification.

## 8.1 Safe

Expected to remove disposable data with little or no destructive impact.

Examples may include:

- unused pnpm store entries,
- old Cypress versions,
- Docker build cache under safe conditions,
- temporary caches.

Safe does **not** mean zero recovery cost.

---

## 8.2 Review

The action is non-destructive to source/state but may create meaningful inconvenience.

Examples:

- Cargo `target`,
- Playwright browser binaries,
- NuGet global packages,
- unused Docker images,
- project-local `node_modules`,
- `.venv`,
- `.next`.

---

## 8.3 Danger

May destroy data that is not automatically recoverable.

Examples:

- Docker volumes,
- important WSL filesystem content,
- other persistent state.

Danger actions must:

- never be preselected,
- show explicit consequences,
- require deliberate confirmation.

---

# 9. Recovery Cost Model

Risk alone is insufficient.

PenguinSpace should also show what happens **after** the user deletes something.

Recommended recovery-cost categories:

## Instant

Regenerated immediately or nearly immediately.

## Download

Requires fetching data from the network again.

## Rebuild

Requires meaningful CPU/build time.

## State Loss

May be irrecoverable.

Examples:

| Item | Risk | Recovery Cost |
|---|---|---|
| pnpm unused store entries | Safe | Download |
| uv prunable cache | Safe | Download |
| Cargo target | Review | Rebuild |
| Playwright browser binaries | Review | Download |
| `node_modules` | Review | Download |
| `.venv` | Review | Download / Rebuild |
| Docker volume | Danger | State Loss |

The UI should answer:

> **“If I delete this, what happens next?”**

not merely:

> “Can I delete this?”

---

# 10. Product Information Architecture

Because PenguinSpace covers very different technical domains, UI categories must follow the **developer mental model**, not internal implementation details.

## Primary Navigation

**LOCKED direction**

```text
Home
Developer Tools
Containers & WSL
Projects
History
Settings
```

Navigation should be placed in a **left sidebar**, consistent with modern Windows desktop applications.

At larger window widths:

- show icons + text labels.

At compact widths:

- collapse to icon-focused navigation where appropriate.

Do not use a web-style top navigation bar as the primary navigation model.

---

# 11. Home Page

## 11.1 Purpose

Home is an **observability/dashboard page**.

It should answer, within a few seconds:

- How much developer storage exists?
- How much can be reclaimed?
- How much is safe to clean?
- What requires review?
- Which category consumes the most?
- What are the largest reclaim opportunities?
- Are virtual disks consuming significantly more physical space than their logical contents?

Home should **not** be a giant list of cleaner checkboxes.

Primary cleanup action:

> **Review & Clean**

This leads into a dedicated review workflow.

---

## 11.2 Home Metrics

The primary metric row should be horizontal.

Recommended metrics:

### Developer Storage

Total storage footprint detected and attributed to developer environments.

Example:

```text
184.6 GB
Developer Storage
```

This may aggregate:

- developer tool caches,
- build artifacts,
- Docker,
- WSL,
- project artifacts.

---

### Reclaimable

The hero metric.

Example:

```text
42.8 GB
Reclaimable
```

This represents the estimated amount PenguinSpace believes can potentially be reclaimed through available cleanup actions.

---

### Safe to Clean

Example:

```text
31.2 GB
Safe to clean
```

Represents cleanup opportunities classified as Safe.

---

### Needs Review

Example:

```text
11.6 GB
Review recommended
```

Represents reclaimable items that have rebuild/download/consequence considerations.

Dangerous items should **not** be promoted as a hero metric.

---

## 11.3 Storage Breakdown

Home should display a horizontal breakdown such as:

```text
Containers & WSL         92.4 GB   50%
Projects                 48.7 GB   26%
Developer Tools          37.8 GB   20%
Other Dev Data            5.7 GB    4%
```

Prefer horizontal bars over large pie charts.

This is easier to scan in a desktop utility.

---

## 11.4 Largest Consumers

Example:

```text
Docker Desktop                   47.8 GB
WSL Ubuntu                       31.7 GB
D:\Projects\SignalScout           9.4 GB
pnpm Store                        8.2 GB
Cargo Targets                     7.1 GB
```

Important:

> Largest Consumer is not the same as Largest Reclaim Opportunity.

A 40 GB WSL distro may contain only 2 GB of safely reclaimable space.

---

## 11.5 Top Reclaim Opportunities

Example:

```text
Docker Build Cache          +14.8 GB
Old Project Artifacts        +8.2 GB
pnpm Store                  +4.6 GB
Cypress Versions            +2.1 GB
Cargo Targets               +1.8 GB
```

This section should be action-oriented.

---

## 11.6 Virtual Disk Metrics

When WSL / Docker / Rancher virtual disks are detected, show a dedicated card.

Example:

```text
Virtual Disks

Physical size              87.4 GB
Used filesystem            53.2 GB

Potentially compactable
~29.8 GB
```

The compactable amount must be marked as an **estimate** until compaction actually occurs.

Do not blindly include estimated VHDX compaction in ordinary reclaimable cache metrics.

---

## 11.7 Cleanup History Metric

A satisfying but restrained metric may be shown:

```text
Space reclaimed

This month
68.4 GB

All time
214.7 GB
```

Avoid excessive gamification.

---

## 11.8 Last Scan Metadata

Example:

```text
Last scanned
2 minutes ago

18 tools detected
5 environments
12 projects
```

This is supporting information, not a hero metric.

---

# 12. Home Horizontal Layout

**LOCKED**

The Home page must be designed primarily for a wide desktop viewport.

A conceptual layout:

```text
┌───────────────────────────────────────────────────────────────────────────────┐
│ PenguinSpace                                                      ─ □ ×      │
├───────────────┬───────────────────────────────────────────────────────────────┤
│               │                                                               │
│  Home         │  Home                                      Last scan: 2m ago │
│               │                                                               │
│  Dev Tools    │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ │
│               │  │ 184.6 GB   │ │ 42.8 GB    │ │ 31.2 GB    │ │ 11.6 GB    │ │
│  Containers   │  │ Dev Storage│ │ Reclaimable│ │ Safe Clean │ │ Review     │ │
│  & WSL        │  └────────────┘ └────────────┘ └────────────┘ └────────────┘ │
│               │                                                               │
│  Projects     │  ┌───────────────────────────────┬───────────────────────────┐ │
│               │  │ Storage Breakdown             │ Top Opportunities         │ │
│  History      │  │ Containers █████████ 92 GB    │ Docker Cache    14.8 GB  │ │
│               │  │ Projects   █████     48 GB    │ Old Projects     8.2 GB  │ │
│  Settings     │  │ Dev Tools  ████      37 GB    │ pnpm Store       4.6 GB  │ │
│               │  └───────────────────────────────┴───────────────────────────┘ │
│               │                                                               │
│               │  ┌──────────────────────────────────┬────────────────────────┐ │
│               │  │ Largest Consumers                │ Virtual Disks          │ │
│               │  │ Docker Desktop          47.8 GB  │ VHDX       87.4 GB    │ │
│               │  │ Ubuntu                  31.7 GB  │ Compactable ~29.8 GB  │ │
│               │  └──────────────────────────────────┴────────────────────────┘ │
└───────────────┴───────────────────────────────────────────────────────────────┘
```

Design goal:

> At common desktop sizes such as 1440×900, the user should understand most Home state without excessive scrolling.

---

# 13. WinUI 3-Inspired Visual Language

PenguinSpace should feel like it belongs on Windows 11.

Desired characteristics:

- Windows system typography / Segoe UI-style typography.
- Fluent-style icons.
- Subtle panel/background contrast.
- Mica/acrylic-inspired surface treatment where practical.
- Light borders.
- Moderate corner radius, approximately 6–10 px.
- Low-shadow / restrained elevation.
- Windows-like hover states.
- Windows-like context menus.
- Windows-like toggles, checkboxes, progress indicators.
- Strong support for light/dark appearance.
- Accent color may use Windows accent where practical, with a PenguinSpace fallback.
- Avoid excessive gradients.
- Avoid SaaS-style glassmorphism.
- Avoid excessive card nesting.
- Avoid neon “clean now” marketing UI.

A metric card should feel understated:

```text
┌─────────────────────────────┐
│ Reclaimable                 │
│                             │
│ 42.8 GB                     │
│                             │
│ 23% of developer storage    │
└─────────────────────────────┘
```

---

# 14. Desktop Layout Principles

The following principles are approved.

## 14.1 Horizontal-first

Use horizontal space intentionally.

Avoid vertical page designs that simply stack every element.

---

## 14.2 Windows-native familiarity

The UI should visually and behaviorally resemble a Windows desktop utility.

---

## 14.3 Master-detail over unnecessary navigation

When appropriate:

- select an item on the left,
- inspect details on the right.

Do not create a separate route/page for every tiny detail.

---

## 14.4 Tables for dense data, cards for metrics

This is a key design rule.

Use cards for:

- summary metrics,
- category summaries,
- high-level actionable insights.

Use tables/list views for:

- many tools,
- many projects,
- many cleanup records,
- many storage items.

Do **not** card-ify every row.

---

## 14.5 One viewport, one understanding

Each major dashboard should communicate the current state with minimal scrolling.

---

# 15. Desktop Responsive Strategy

Responsive behavior is for **desktop window resizing**, not mobile web.

Suggested states:

```text
≥ 1400 px
Full dashboard

1100–1399 px
Compact dashboard

800–1099 px
Collapsed sidebar + 2-column/condensed layout

< 800 px
Minimum supported window / simplified layout
```

PenguinSpace does not need to be optimized for 375 px smartphone layouts.

---

# 16. Developer Tools Page

## Purpose

Manage developer-tool storage by ecosystem.

The page must be grouped by the user's mental model.

Recommended ecosystems:

### Node.js

- npm
- pnpm
- Yarn
- Bun

### Python

- uv

Potential future additions are allowed if they fit the same provider model.

### .NET

- NuGet

### Rust

- Cargo

### Java / JVM

- Gradle
- Maven

### Testing / Browser Tooling

- Cypress
- Playwright

---

## Horizontal Layout Direction

Do not display one long vertical list.

Use ecosystem grouping with horizontal space.

Example:

```text
Developer Tools

┌──────────────────────────────┬──────────────────────────────┐
│ Node.js                      │ Python                       │
│                              │                              │
│ npm          1.8 GB          │ uv             2.1 GB       │
│ pnpm         5.4 GB          │                              │
│ Yarn         0.9 GB          │                              │
│ Bun          1.2 GB          │                              │
└──────────────────────────────┴──────────────────────────────┘

┌──────────────────────────────┬──────────────────────────────┐
│ Rust                         │ Java / JVM                   │
│ Cargo        7.2 GB          │ Gradle         4.8 GB       │
│                              │ Maven          3.9 GB       │
└──────────────────────────────┴──────────────────────────────┘
```

---

## Master-detail Interaction

Selecting a tool should ideally show detailed information beside the list rather than always navigating to a new page.

Example:

```text
┌──────────────────────────────┬────────────────────────────────────┐
│ Node.js                      │ pnpm                               │
│                              │                                    │
│ npm          1.8 GB          │ Store size           5.4 GB       │
│ pnpm         5.4 GB   ←      │ Reclaimable          2.2 GB       │
│ Yarn         0.9 GB          │ Risk                 Safe         │
│ Bun          1.2 GB          │ Recovery             Download     │
│                              │                                    │
│                              │ [ Inspect ]      [ Clean ]         │
└──────────────────────────────┴────────────────────────────────────┘
```

---

# 17. Containers & WSL Page

This domain is deliberately separate from Developer Tools.

Reasons:

- container storage is stateful,
- Docker has its own resource model,
- WSL storage is virtualized,
- VHDX compaction affects host physical storage,
- some actions require process shutdown and/or elevation.

---

## 17.1 Docker Representation

PenguinSpace should break Docker usage into at least:

- Images
- Dangling images
- Unused images
- Build cache
- Stopped containers
- Volumes
- Unused volumes
- Virtual disk / backing storage when applicable

Example:

```text
Docker Desktop                         31.4 GB

Images                   14.3 GB
 ├─ Dangling              3.1 GB   Safe
 └─ Unused               11.2 GB   Review

Build Cache               9.8 GB   Safe

Stopped Containers        1.1 GB   Review

Volumes                    6.2 GB
 └─ Unused                 4.7 GB   Danger
```

Docker volume cleanup must never be preselected.

---

## 17.2 Docker Data vs VHDX

Logical Docker data and the physical Windows VHDX file must be reported separately.

Example:

```text
Docker data
23.4 GB

Docker VHDX
57.8 GB

Potential compactable space
~31.6 GB
```

This distinction is a flagship PenguinSpace capability.

---

## 17.3 WSL Distributions

Display distributions such as:

- Ubuntu
- Debian
- docker-desktop
- Rancher-related distributions where present

Example:

```text
┌─────────────────────────────────────┬──────────────────────────────┐
│ WSL Distributions                   │ Selected: Ubuntu             │
│                                     │                              │
│ Ubuntu          31.7 GB             │ Filesystem        21.2 GB   │
│ docker-desktop  47.3 GB             │ VHDX physical     31.7 GB   │
│ Rancher          8.2 GB             │ Compactable       ~8.6 GB   │
└─────────────────────────────────────┴──────────────────────────────┘
```

---

## 17.4 VHDX Compaction Workflow

Conceptual workflow:

```text
Clean internal data
        ↓
wsl --shutdown
        ↓
Locate VHDX
        ↓
Attach / make available in compactable state
        ↓
Compact VHDX
        ↓
Detach
        ↓
Measure reclaimed physical space
```

Important implementation principles:

- discover distro/VHDX information through Windows-supported mechanisms when available,
- do not hard-code AppData paths if a more reliable discovery path exists,
- request elevation only for operations that require it,
- avoid running the entire application permanently elevated,
- show impact before shutting down WSL,
- report both estimated and actual reclaimed physical space.

---

# 18. Projects Page

Project-local storage is a separate product domain.

It must not be confused with global package caches.

Examples of project-local generated data:

```text
node_modules/
target/
.venv/
.next/
dist/
build/
.gradle/
.turbo/
```

Additional generated directories may be added through the provider/scanner model.

---

## 18.1 Project Discovery

User should be able to configure one or more workspace roots, for example:

```text
D:\Projects
C:\Users\<user>\source\repos
```

PenguinSpace scans those roots and identifies projects.

Potential signals include:

- `package.json`
- `pnpm-workspace.yaml`
- `Cargo.toml`
- `pyproject.toml`
- `.sln`
- `.csproj`
- `pom.xml`
- `build.gradle`
- `settings.gradle`
- other ecosystem manifests.

---

## 18.2 Project Overview UI

Use a dense Windows-style table/list rather than one card per project.

Example:

```text
Projects

D:\Projects                                  Scan folders…

┌──────────────────────────────────────────────────────────────────┐
│ Project          Ecosystem     Size      Reclaimable   Last used │
├──────────────────────────────────────────────────────────────────┤
│ SignalScout      Node/Python   9.4 GB    4.8 GB        Today     │
│ PenguinSpace     Go/Vue        2.1 GB    820 MB        Today     │
│ SnakeAid         .NET/React    5.6 GB    3.2 GB        2 weeks   │
│ Old Hackathon    Node          11.8 GB   10.1 GB       8 months  │
└──────────────────────────────────────────────────────────────────┘
```

Selecting a project should expose its storage breakdown.

Example:

```text
SignalScout

node_modules       4.2 GB
.next              1.8 GB
.venv              1.4 GB
dist              340 MB
.git              710 MB
```

Note:

- `.git` may be displayed as storage information,
- but cleanup behavior must be separately designed and must never imply `.git` can be casually deleted.

---

# 19. History Page

PenguinSpace should record cleanup history.

Useful fields:

- timestamp,
- provider/domain,
- action,
- target,
- risk level,
- estimated reclaimable bytes,
- actual reclaimed bytes,
- result,
- duration,
- error if any,
- whether elevation was required.

Possible summaries:

```text
This month reclaimed: 68.4 GB
All-time reclaimed: 214.7 GB
```

History must help with transparency and debugging, not just gamification.

---

# 20. Settings Page

Settings should eventually include areas such as:

## General

- theme / system theme
- startup behavior
- tray/background behavior
- scan behavior

## Workspace Scanning

- project roots
- excluded folders
- filesystem scan limits
- optional ignore rules

## Cleanup Safety

- confirmation preferences for Review actions
- strong confirmation policy for Danger actions

Danger actions should not become silently executable merely because a user disables ordinary confirmations.

## Advanced

- diagnostics/logs
- provider status
- detected binary paths
- command timeouts
- experimental integrations if ever required

---

# 21. Core Backend Architecture

The backend must **not** become a collection of scattered `exec.Command()` calls.

PenguinSpace should use a provider-based cleanup architecture.

Conceptual model:

```text
                    PenguinSpace
                         │
                 Cleanup Engine
                         │
        ┌────────────────┼────────────────┐
        │                │                │
     Scanner          Planner         Executor
        │                │                │
        └──────── Cleaner Providers ──────┘
                         │
       ┌───────────┬─────┴─────┬───────────┐
       uv         pnpm       Docker       WSL
       npm        Cargo      Gradle       NuGet
       Bun        Maven      Cypress      ...
```

---

# 22. Provider Responsibilities

Every cleaner/provider should expose a consistent conceptual contract.

## Detect

Questions:

- Is the tool installed?
- Which version?
- Where is the executable?
- Which environment/context is active?

---

## Inspect

Questions:

- Which storage locations/resources exist?
- How large are they?
- Which subsets are reclaimable?
- Which information can the tool itself report?

---

## Plan

Produces cleanup actions.

Each action should include enough information for UI review.

Conceptual shape:

```text
CleanerAction {
    id
    provider
    title
    description
    target
    estimatedReclaimableBytes
    risk
    recoveryCost
    requiresAdmin
    requiresShutdown
    commandOrStrategy
}
```

The actual Go type may differ, but the frontend-facing model must preserve these concepts.

---

## Clean / Execute

Executes the approved action.

Requirements:

- cancellable where practical,
- timeout-aware,
- stdout/stderr captured for diagnostics,
- clear errors,
- no hidden unrelated cleanup.

---

## Verify

After execution:

- re-measure storage,
- calculate actual reclaimed bytes,
- refresh provider state,
- record history.

This enables the core UX:

```text
Before: 42.8 GB
After:  24.1 GB

Reclaimed: 18.7 GB
```

---

# 23. CLI-First / Tool-Native Cleanup Principle

**Strong implementation rule**

If a tool provides an official cleanup command/API, PenguinSpace should prefer that over directly deleting internal cache directories.

Examples:

```text
pnpm      → pnpm store prune
uv        → uv cache prune
Cypress   → cypress cache prune
NuGet     → dotnet nuget locals ...
Docker    → Docker CLI / API semantics
Cargo     → cargo clean for project build artifacts
```

Reasons:

- official commands understand the tool's internal state,
- cache layout may change between versions,
- direct filesystem deletion can violate tool assumptions,
- tool-native cleanup is easier to maintain.

Filesystem deletion should be a fallback only when:

- no supported lifecycle mechanism exists,
- behavior is understood,
- risk is clearly classified.

---

# 24. Scan and Clean Must Be Separate

PenguinSpace must not open and immediately execute cleanup commands.

Primary flow:

```text
Launch
  ↓
Scan
  ↓
Measure
  ↓
Classify
  ↓
Show Dashboard
  ↓
User chooses Review & Clean
  ↓
Cleanup Plan
  ↓
User confirms
  ↓
Execute
  ↓
Verify
  ↓
Show reclaimed space
```

A scan result may look like:

```text
uv               1.2 GB
pnpm             4.8 GB
Cypress          2.7 GB
Docker          23.4 GB
WSL             41.7 GB

Estimated reclaimable
~28.9 GB
```

---

# 25. Cleanup Review Workflow

The review screen should be one of the most important product surfaces.

Each action should communicate:

- item/provider,
- current size,
- estimated reclaimable size,
- risk,
- recovery cost,
- consequence,
- required privileges,
- required process/environment shutdown.

Example:

```text
Docker Build Cache
14.8 GB reclaimable
Risk: Safe
Recovery: Rebuild
No persistent volumes affected
```

Example:

```text
Playwright Browsers
2.3 GB reclaimable
Risk: Review
Recovery: Download
Browsers must be downloaded again before tests run
```

Example:

```text
Unused Docker Volume
8.1 GB
Risk: Danger
Recovery: State Loss
May contain local database/application data
```

Dangerous actions:

- never preselected,
- visually distinct,
- require explicit user selection,
- require stronger confirmation.

---

# 26. Privilege / UAC Model

PenguinSpace should run normally without Administrator privileges.

Elevation should be requested **only for operations that require it**.

Examples may include:

- specific VHD/VHDX operations,
- protected filesystem actions.

Benefits:

- reduced blast radius,
- better Windows security posture,
- clearer user trust model.

The application must be able to report:

```text
Requires Administrator permission
```

at planning time rather than surprising the user after they press Clean.

---

# 27. Docker Implementation Principles

Docker is a flagship domain.

Do not expose only:

```bash
docker system prune -a --volumes
```

Instead inspect Docker resource classes separately.

Potential information source:

```bash
docker system df
```

Where possible use structured output / stable APIs rather than parsing human-friendly output.

Individual cleanup actions should be mapped to resource semantics.

Examples:

- build cache cleanup,
- dangling image cleanup,
- unused image cleanup,
- stopped container cleanup,
- volume cleanup.

Volume cleanup is always treated with heightened caution.

---

# 28. WSL / VHDX Implementation Principles

WSL/VHDX is one of PenguinSpace's strongest differentiators.

Key product concept:

> Logical cleanup inside Linux does not always equal physical host disk reclamation.

PenguinSpace should track:

- distro,
- filesystem logical usage,
- backing virtual-disk path,
- VHDX physical size,
- estimated compactable space,
- actual physical size after compaction.

The application should explain why a user may have cleaned many GB but not yet recovered Windows disk space.

This is not a hidden advanced detail; it is a first-class product feature.

---

# 29. Wails 3 Considerations

Wails 3 was selected deliberately.

Features relevant to PenguinSpace include the ability to build a desktop utility with capabilities such as:

- system tray behavior,
- background-style operation,
- native menus,
- autostart/startup integration,
- frameless/native-feeling window handling,
- Go backend + web UI frontend architecture.

Because Wails 3 is the selected stack, architecture and dependency versions should be intentionally pinned and kept upgradeable.

---

# 30. Vue Frontend Responsibilities

The Vue frontend should operate on domain models, not hard-coded cleanup commands.

The frontend should understand:

- providers,
- storage categories,
- actions,
- risk,
- recovery cost,
- sizes,
- statuses,
- progress,
- history.

It should **not** need to know:

```text
pnpm store prune
```

or other raw implementation details to render normal product UI.

This separation keeps provider logic in Go and product presentation in Vue.

---

# 31. Suggested Frontend Design System Concepts

Use a small reusable design system inspired by WinUI/Fluent patterns.

Potential components:

- AppShell
- NavigationSidebar
- PageHeader
- MetricCard
- SectionCard
- StorageBar
- StatusBadge
- RiskBadge
- RecoveryCostBadge
- ToolList
- MasterDetailPane
- DataTable
- EmptyState
- ProgressIndicator
- ConfirmationDialog
- DangerConfirmationDialog
- InfoBar / WarningBar
- SearchBox
- SettingsRow
- Toggle
- ContextMenu
- CommandBar

Avoid implementing every surface as a generic rectangular card.

---

# 32. Product Goal Pages

At Product Goal, the primary surfaces are:

1. **Home**
2. **Developer Tools**
3. **Containers & WSL**
4. **Projects**
5. **History**
6. **Settings**
7. **Cleanup Review / Execution flow**
8. **Detail panes** for specific tools/environments/projects

These pages collectively form the intended complete product experience.

---

# 33. Development Milestones Toward Product Goal

These are not “MVP → production” phases.

They are milestones that incrementally implement the already-defined final product.

---

## Milestone 1 — Core Platform

Build the architectural foundation.

Includes:

- Wails 3 application shell
- Vue application
- WinUI-inspired design system
- navigation shell
- domain models
- provider interface
- detection engine
- scanner engine
- cleanup planning engine
- cleanup executor
- verification engine
- storage measurement utilities
- risk model
- recovery-cost model
- history persistence
- privilege/elevation mechanism
- logging foundation
- cancellation/timeout primitives
- error model

Goal:

> The architecture is ready to support all product domains without redesign.

---

## Milestone 2 — Developer Tool Ecosystems

Implement providers for the initial supported tool families.

### Node.js

- npm
- pnpm
- Yarn
- Bun

### Python

- uv

### .NET

- NuGet

### Rust

- Cargo

### Java / JVM

- Gradle
- Maven

### Testing / Browser tooling

- Cypress
- Playwright

For each provider:

- detect,
- inspect,
- estimate,
- plan,
- clean,
- verify,
- surface risk/recovery information.

---

## Milestone 3 — Containers & WSL

Implement:

- Docker detection
- Docker Desktop awareness
- Rancher Desktop awareness
- container/image/build-cache/volume inspection
- Docker cleanup actions
- WSL distro discovery
- WSL storage measurement
- VHDX discovery
- VHDX physical size measurement
- compactability estimation
- VHDX compaction workflow
- UAC/elevation flow
- pre-operation shutdown warnings
- post-operation verification

This is expected to be one of the technically hardest milestones.

---

## Milestone 4 — Project Storage

Implement:

- workspace root configuration
- project discovery
- ecosystem detection
- generated-artifact detection
- recursive size scanning
- project-level reclaim estimation
- project detail view
- cleanup planning
- last-used heuristics where reliable
- exclusions / ignore rules

Target generated folders include at least:

```text
node_modules/
target/
.venv/
.next/
dist/
build/
.gradle/
.turbo/
```

Additional ecosystems may be added through maintainable scanner rules.

---

## Milestone 5 — Complete Desktop Experience

Complete user-facing behavior.

Includes:

- Home metrics
- storage breakdown
- largest consumers
- reclaim opportunities
- virtual disk metrics
- search
- filters
- cleanup review
- progress UX
- cleanup result UX
- history UX
- system tray
- background scanning
- notifications
- startup behavior
- theme behavior
- responsive desktop resizing
- polished empty/error/loading states
- master-detail interactions
- context menus / command bars

---

## Milestone 6 — Production Hardening

Includes:

### Reliability

- crash handling
- error boundaries
- resilient command execution
- timeouts
- cancellation
- filesystem edge cases
- locked files
- permission failures
- missing binaries
- malformed tool output
- partially completed cleanups
- concurrent scan handling
- app restart recovery where needed

### WSL / Docker Safety

- distro-running edge cases
- Docker daemon unavailable
- Rancher/Desktop engine unavailable
- VHDX operation failures
- interrupted compaction
- admin cancellation
- resource refresh after daemon state changes

### Testing

- unit tests
- provider tests
- parser tests
- risk-classification tests
- cleanup-plan tests
- filesystem scanner tests
- integration tests
- Windows-specific tests
- Docker/WSL integration scenarios where feasible

### Distribution

- Windows installer
- release build process
- code signing strategy
- versioning
- auto-update strategy
- release CI/CD
- documentation
- troubleshooting guide
- logs / diagnostic export if useful

After Milestone 6, the **PenguinSpace Product Goal** is considered achieved.

---

# 34. Product Safety Principles

PenguinSpace deals with destructive operations and must be conservative.

## Mandatory principles

1. Scan before clean.
2. Explain before clean.
3. Use tool-native cleanup where possible.
4. Never preselect Danger actions.
5. Never present volume deletion as routine cleanup.
6. Distinguish estimated reclaim from actual reclaimed disk.
7. Verify after cleanup.
8. Request admin privilege only when necessary.
9. Do not silently install third-party cleaners.
10. Do not silently delete unknown filesystem locations.
11. Preserve command logs/diagnostic context for failed actions.
12. If tool semantics differ by version, provider logic must account for it.

---

# 35. Product Tone

The product should be calm and technical.

Avoid scare tactics such as:

- “CRITICAL JUNK DETECTED”
- “PC HEALTH 22%”
- “BOOST NOW”
- “MEGA CLEAN”

Prefer factual language:

```text
14.8 GB reclaimable
```

```text
This action removes build cache.
Projects may rebuild more slowly the next time they run.
```

```text
This volume may contain persistent application data.
```

PenguinSpace is a developer tool, not consumer scareware.

---

# 36. Desired Home Experience

A successful Home screen should make this immediately understandable:

```text
Developer Storage            184.6 GB
Reclaimable                   42.8 GB
Safe to Clean                 31.2 GB
Needs Review                  11.6 GB
```

Then, without excessive scrolling:

- where that storage lives,
- which domain is largest,
- which cleanup opportunities are best,
- whether virtual disks can reclaim additional host space.

The Home page is primarily **observability**, not execution.

---

# 37. Desired Cleanup Result Experience

After a cleanup operation PenguinSpace should communicate actual outcome.

Example:

```text
Cleanup complete

18.7 GB reclaimed

Docker Build Cache       12.4 GB
pnpm Store                3.1 GB
Cypress Versions          2.0 GB
uv Cache                  1.2 GB
```

If VHDX compaction was performed:

```text
Docker logical cleanup
21.4 GB removed

VHDX physical size
Before: 57.8 GB
After:  36.2 GB

Host disk reclaimed
21.6 GB
```

This verifies that the app actually delivered value.

---

# 38. Initial Supported Domains

The currently approved initial Product Goal should cover at least the domains already identified.

## Package / tool cache ecosystem

- uv
- npm
- pnpm
- Yarn
- Bun
- NuGet
- Cargo
- Gradle
- Maven

## Testing / browser tooling

- Cypress
- Playwright

## Container / virtualization environment

- Docker
- Docker Desktop
- Rancher Desktop
- WSL
- VHDX compaction

## Project-local artifacts

At least:

- `node_modules`
- `target`
- `.venv`
- `.next`
- `dist`
- `build`
- `.gradle`
- `.turbo`

This list may grow, but the architecture must remain provider/scanner-driven.

---

# 39. Research-Informed Command Guidance

This section records the current command-level understanding so an implementation agent does not blindly encode the original command list.

| Tool / Domain | Original idea | Preferred product treatment |
|---|---|---|
| uv | `uv cache clean` | Prefer routine `uv cache prune`; keep full clean as aggressive option |
| npm | `npm cache clean --force` | Do not present as ordinary Safe Clean; npm cache is designed to be self-healing |
| pnpm | `pnpm store prune` | Good default tool-native prune behavior |
| Yarn | `yarn cache clean` | Provider must account for Yarn version/behavior |
| Bun | `bun pm cache rm` | Tool-native cleanup provider |
| Cypress | `npx cypress cache clear` | Prefer `cypress cache prune`; full clear as aggressive action |
| Playwright | `npx playwright uninstall --all` | Classify as Review + Download, not generic cache clean |
| NuGet | `dotnet nuget locals all --clear` | Split cache categories where possible; global packages carry restore cost |
| Cargo global cache | install/use `cargo-cache` | Do not auto-install third-party cleaner; account for modern Cargo GC |
| Cargo project | `cargo clean` | Treat as project artifact cleanup / Rebuild cost |
| Gradle | delete entire `~/.gradle/caches` | Not routine Safe Clean; use conservative/tool-aware strategy |
| Maven | delete entire `~/.m2/repository` | Not routine Safe Clean; prefer scoped/tool-aware purge |
| Docker | `docker system prune -a --volumes` | Split resource classes; volumes Danger; never blanket default |
| WSL VHDX | manual diskpart compact flow | First-class guided workflow with measurement, elevation, verification |

---

# 40. External Documentation References

These references informed the current product direction and should be re-verified during implementation because CLI behavior can evolve.

## uv

- https://docs.astral.sh/uv/concepts/cache/

## pnpm

- https://pnpm.io/cli/store

## npm

- https://docs.npmjs.com/cli/v7/commands/npm-cache/

## Cypress

- https://docs.cypress.io/app/references/command-line

## Playwright

- https://playwright.dev/docs/browsers

## NuGet

- https://learn.microsoft.com/en-us/nuget/consume-packages/managing-the-global-packages-and-cache-folders

## Cargo

- https://doc.rust-lang.org/cargo/reference/config.html
- https://doc.rust-lang.org/cargo/commands/cargo-clean.html

## Gradle

- https://docs.gradle.org/current/userguide/directory_layout.html

## Maven

- https://maven.apache.org/plugins/maven-dependency-plugin/examples/purging-local-repository.html

## Docker

- https://docs.docker.com/engine/manage-resources/pruning/
- https://docs.docker.com/reference/cli/docker/system/df/

## WSL / VHDX

- https://learn.microsoft.com/en-us/windows/wsl/disk-space
- https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/compact-vdisk

## Wails 3

- https://v3.wails.io/
- https://v3.wails.io/blog/wails-v3-beta/
- https://v3.wails.io/features/menus/systray/
- https://v3.wails.io/features/windows/frameless/

---

# 41. Coding-Agent Rules

Any coding agent working from this file should follow these rules.

## Do not reinterpret approved product direction

The following are already decided:

- Wails 3
- Go
- Vue
- Windows-first
- horizontal-first
- WinUI 3-inspired
- developer storage manager, not generic PC cleaner
- provider architecture
- risk + recovery-cost model
- Home as metrics/observability
- Developer Tools / Containers & WSL / Projects separated into domains
- Docker/WSL/VHDX are first-class capabilities
- project artifact scanning is part of the Product Goal
- development milestones lead to one complete Product Goal

---

## Do not implement cleanup commands ad hoc

All cleanup behavior must pass through domain/provider logic.

Avoid code patterns where Vue calls arbitrary shell commands directly.

---

## Do not assume command behavior is timeless

Before implementing a provider:

1. verify the current official command/API,
2. verify supported versions,
3. prefer structured output,
4. confirm cleanup semantics,
5. define risk,
6. define recovery cost,
7. define verification strategy.

---

## Do not mix storage concepts

Do not report these as equivalent:

- cache,
- build artifact,
- persistent data,
- virtual disk allocation.

The product value depends on distinguishing them.

---

## Do not sacrifice desktop UX for web conventions

This is a desktop application.

Prefer:

- horizontal space,
- master-detail,
- list/table density,
- native-feeling sidebar,
- compact information hierarchy.

Avoid:

- giant vertically stacked cards,
- mobile-first layout patterns,
- unnecessary route transitions.

---

# 42. Final Product Vision

PenguinSpace should ultimately feel like a **developer storage control center for Windows**.

A developer opens it and immediately sees:

- how much disk space development tooling consumes,
- which domains are responsible,
- what can safely be reclaimed,
- what would require a rebuild/download,
- what could destroy state,
- which old projects are wasting space,
- how much Docker is consuming,
- how much WSL/VHDX is physically consuming,
- how much physical Windows disk space can actually be returned.

The application should remove the need to remember or repeatedly search for commands such as:

```bash
uv cache prune
pnpm store prune
cypress cache prune
dotnet nuget locals ...
cargo clean
docker ...
```

while still respecting the semantics of the underlying tools.

The desired user feeling is:

> “I understand exactly what is consuming my developer storage, what I can remove, what it will cost me afterward, and how much disk space I actually got back.”

That is the core promise of **PenguinSpace**.

---

# 43. Canonical Summary

If a coding agent only retains a short summary, retain this:

> **PenguinSpace is a Windows-first developer storage manager built with Wails 3, Go, and Vue. It uses a horizontal, WinUI 3-inspired desktop UI and separates Developer Tools, Containers & WSL, Projects, History, and Settings into clear domains. It scans before cleaning, classifies every cleanup action by risk and recovery cost, prefers official tool-native cleanup commands, treats Docker volumes as dangerous state, treats Docker/WSL/VHDX physical storage as a flagship domain, scans project-local generated artifacts, verifies actual reclaimed bytes after operations, and is developed through milestones toward one complete Product Goal rather than through a disposable MVP architecture.**
