---
baseline_schema: "2.0"
pack: "penguin-space"
document: "hallucination"
status: "active"
updated: "2026-08-10"
code_ref: "worktree-m1-elevation-after-e3f7522"
---

# Decisions, unknowns, and claim boundaries

## Closed decisions

The following are approved and must not be re-litigated during implementation without explicit owner direction:

- Wails 3, Go, Vue 3 + TypeScript + Vite, Windows-first delivery.
- Horizontal-first, WinUI 3-inspired Windows utility rather than a mobile-stretched SaaS dashboard or consumer scareware.
- Complete Product Goal architecture delivered through milestones rather than a disposable MVP.
- Provider/scanner-driven model; backend owns commands and tool semantics.
- Risk plus recovery-cost communication for every cleanup action.
- Home is observability; clean actions begin in a review workflow.
- Docker, WSL, and VHDX are first-class; Docker resource types are distinct and volumes are danger state.
- Project-local artifact discovery is in scope.
- Scan and clean are separate, and cleanup requires review, confirmation, execution, and verification.
- Use tool-native cleanup when possible; do not silently install third-party cleaners or delete unknown filesystem paths.
- Administrator elevation is per-operation, never the default launch mode.
- PenguinSpace build, dependency installation, code generation, verification, tests, and packaging run inside Docker; the Windows host is only an editor, Git client, and Docker/Rancher orchestrator.
- Bun `1.3.14` is the frontend package manager and script runtime inside the Docker toolchain. Node and Corepack are not part of the PenguinSpace M1 build environment; the repository will use a single `bun.lock` lockfile and no competing JavaScript package-manager lockfile.
- The frontend pins Vue `3.5.41`, Vite `8.2.1`, TypeScript `6.0.3`, `@vitejs/plugin-vue` `6.0.8`, `vue-tsc` `3.3.9`, and `@wailsio/runtime` `3.0.0-beta.5`. The runtime registry has no `3.0.0-beta.6` package, so it is intentionally recorded separately from Wails CLI `v3.0.0-beta.6`.
- Store every measurement as exact integer bytes; distinguish measured, estimated, logical-deletion, and physical-reclamation values; display IEC units with at most three significant digits.
- Persist settings and cleanup/history records in a schema-versioned SQLite database under the Windows per-user Local AppData known folder; keep diagnostics as rotated text files beside it and never ingest an untrusted database file.
- Discover project artifacts only below explicitly configured roots. Do not follow symbolic links/reparse points by default; record inaccessible paths rather than treating them as deletable.
- Keep the main application non-elevated. Privileged operations will run in an isolated helper launched with the Windows `runas` verb, using a narrowly scoped plan/result contract.
- Danger actions are never preselected and require an explicit consequence summary plus a deliberate confirmation step; ordinary Review confirmations are preference-controlled but cannot weaken Danger confirmation.

## Unverified external claims

The following proposal guidance is product research, not runtime-confirmed implementation truth. Revalidate it against official documentation and installed versions before encoding it:

- Exact cleanup commands and behaviour for uv, npm, pnpm, Yarn, Bun, Cypress, Playwright, NuGet, Cargo, Gradle, Maven, and Docker.
- Current Wails 3 API, lifecycle, system-tray, window, and packaging behaviour beyond the Phase 0 CLI/install and Windows cross-build research.
- Windows and WSL/VHDX compaction requirements, privileges, supported paths, and recovery behaviour.
- Wails installer/package creation and interactive Windows UI behavior. A Docker-built Windows executable has passed only a hidden process-start smoke test.
- Competitive product capabilities and differentiation statements.

The preserved source contains the complete research command table and external links: [sources/PROPOSAL.md](sources/PROPOSAL.md#39-research-informed-command-guidance) and [sources/PROPOSAL.md](sources/PROPOSAL.md#40-external-documentation-references).

## Phase 0 decision register

The owner of every implementation task below is **Engineering**. `Closed` means the architecture or policy is settled; it is not runtime proof. `Deferred` means the scope, owner, risk, and target milestone are explicit.

| Topic | Outcome and status | Evidence and chosen approach | Follow-on task |
|---|---|---|---|
| Docker build-image contract | **Implemented for M1 bootstrap.** Dockerfile pins `golang:1.25-bookworm@sha256:908f8ff2ec296df2f349563072c7925775cd28b50361a52ed834a8a37399b9bf`, `oven/bun:1.3.14@sha256:e10577f0db68676a7024391c6e5cb4b879ebd17188ab750cf10024a6d700e5c4`, and Wails CLI `v3.0.0-beta.6`; it never uses `@latest`. Vue 3 + TypeScript + Vite uses Bun and one `bun.lock`; Node/Corepack and their lockfiles are excluded. | `docker compose run --rm verify` passed Bun 1.3.14, Go 1.25.12, Wails beta.6, generated TypeScript bindings, Vue type-check/build, Go internal vet/tests, and a Windows cross-compile. `docker compose run --rm build` produced `out/penguinspace.exe`; hidden host process start passed. `@wailsio/runtime` is pinned to registry version `3.0.0-beta.5`, the newest matching beta available during M1. The checked-in Vite/Vue declaration shim remains required. [Wails cross-build](https://v3.wails.io/guides/build/cross-platform/), [Bun Docker image](https://github.com/oven-sh/bun), [Docker volumes](https://docs.docker.com/engine/storage/volumes/) | Preserve `verify`, `build`, and `shell`; test real Windows elevation and package creation before release. |
| Supported tool versions | **Deferred — M2, non-blocking for M1.** Support is version-gated per provider, not inferred from command names. A provider supports only profiled versions listed in code and tests; unknown major versions may be detected but must not yield a cleanup plan. | CLI behavior is time-sensitive. This avoids a false global version promise before representative installations exist. | M2 adds a version matrix and fixture evidence with each provider implementation. |
| Provider command/API detail | **Deferred — M2, non-blocking for M1.** The command catalogue below is an approved research baseline, not an executable batch script. | `npm cache verify` is inspection/repair; `pnpm store prune` removes only unreferenced entries; Cargo supports project-scoped `clean --dry-run`; Gradle provides its own cache lifecycle; NuGet, Cypress, and Playwright expose scoped cache operations. [npm](https://docs.npmjs.com/cli/cache/), [pnpm](https://pnpm.io/cli/store), [uv](https://docs.astral.sh/uv/concepts/cache/), [NuGet](https://learn.microsoft.com/en-us/dotnet/core/tools/dotnet-nuget-locals), [Cargo](https://doc.rust-lang.org/cargo/commands/cargo-clean.html), [Gradle](https://docs.gradle.org/current/userguide/directory_layout.html), [Cypress](https://docs.cypress.io/app/references/command-line), [Playwright](https://playwright.dev/docs/browsers), [Maven](https://maven.apache.org/components/plugins-archives/maven-dependency-plugin-3.8.1/purge-local-repository-mojo.html), [Yarn](https://yarnpkg.com/cli/cache/clean), [Bun](https://bun.sh/docs/pm/cli/pm) | M2 encodes one provider at a time: inspect first, then a reviewable plan and fixture tests. No bulk global cleanup is authorized. |
| Size calculation and rounding | **Closed.** Backend values are `uint64` bytes and never rounded before persistence. UI uses IEC units (`KiB`, `MiB`, `GiB`, `TiB`) with at most three significant digits; byte values remain available in detail/export. | Exact bytes prevent sum/display drift; separate result fields preserve the proposal's logical-versus-physical distinction. | M1 defines models and format/rounding fixture tests. |
| Persistence format/location | **Closed.** Use a schema-versioned SQLite database for settings, history, plans, outcomes, and recovery records below the Windows per-user Local AppData known folder; diagnostics are separate rotated text files. | SQLite provides a stable, single-file, transactional application format; Windows recommends known-folder APIs over deprecated CSIDL paths. [SQLite](https://www.sqlite.org/appfileformat.html), [Windows known folders](https://learn.microsoft.com/en-us/windows/win32/shell/known-folders) | M1 chooses the pure-Go SQLite driver, defines migrations, redaction, retention, and recovery tests. |
| Exact elevation bridge | **Prototype built and container-verified; interactive runtime verification remains open.** Worktree code after `e3f7522` uses a separate helper invoked through Windows `runas`. It accepts only an opaque request ID, resolves a versioned local request, and validates the one fixed `m1.elevation.probe` action; neither the UI nor the request carries a shell command or cleanup path. Cancellation and expiry create terminal statuses without starting a cleanup command. Docker verification passed bindings, frontend checks, formatting, vet, internal tests, and a Windows cross-build; the generated executable passed a hidden process-start smoke test. | Windows documents `runas` as the UAC-elevated launch verb. This preserves normal non-elevated operation and narrows privilege scope. [ShellExecute](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shellexecutea) | Run Windows UAC acceptance, refusal, cancellation, timeout, and malformed-request checks. A real privileged provider remains M2+ work. |
| VHDX compaction strategy | **Deferred — M3, blocking only for VHDX execution.** Until a real Docker/WSL fixture exists, support discovery and an explanatory plan only; no compaction command is exposed. | `compact vdisk` applies only to dynamically expanding VHDs that are detached or read-only; `Optimize-VHD` may succeed without reducing file size. The local `.wslconfig` sets new WSL2 VHDs to 20GB, and Rancher Desktop currently has a running engine plus a stopped data distro. [compact vdisk](https://learn.microsoft.com/en-us/Windows-server/administration/windows-commands/compact-vdisk), [Optimize-VHD](https://learn.microsoft.com/en-us/powershell/module/hyper-v/optimize-vhd), [WSL config](https://learn.microsoft.com/windows/wsl/wsl-config) | M3 tests discovery, shutdown warning, UAC refusal, interruption, and measured physical size before/after against a disposable WSL/VHDX fixture. |
| Project-root discovery defaults | **Closed.** Start with no implicit user-profile scan: users explicitly configure roots; the app may suggest detected Git/workspace roots only for approval. Reparse points are listed but not traversed. | This prevents broad, surprising, or cyclic scans while preserving a discoverable workflow. | M1 implements root configuration and traversal/error fixtures; M4 adds workspace discovery UX. |
| Confirmation language | **Closed.** Every action states item, bytes, risk, recovery cost, consequence, and whether privilege/shutdown is required. Review uses a normal confirmation; Danger uses a separate deliberate confirmation that cannot be disabled. | Preserved proposal requires explicit consequences, no preselection, and deliberate confirmation for Danger. | M1 defines confirmation models and copy fixtures; M5 validates the visual/accessibility experience. |

### Provider research boundary

The following classifications guide later per-provider plans; they are not support claims until the M2 version matrix and fixtures exist.

| Ecosystem | Research conclusion | Initial risk boundary |
|---|---|---|
| npm | Inspect with `npm cache verify`; clearing requires `npm cache clean --force`. | `verify`: Safe; clearing: Review. |
| pnpm | `pnpm store prune` removes unreferenced packages and reports removed size. | Safe; recovery cost Download. |
| Yarn / Bun | Yarn provides local/global cache scopes; Bun exposes global cache path/removal. | Review; never erase a project-local cache without its project context. |
| uv / NuGet | Both offer official cache-specific clear commands. | Review; global packages/caches may cause later download or rebuild. |
| Cargo | `cargo clean --dry-run` scopes inspection to a project target directory. Cargo-home layout is explicitly unstable. | Review for project artifacts; do not delete Cargo home by path. |
| Gradle / Maven | Prefer Gradle's retention lifecycle; Maven purge is project/dependency scoped and may re-resolve unless configured. | Review; no blanket `.gradle` or `.m2` deletion. |
| Cypress / Playwright | Cypress lists sizes and can prune old binaries; Playwright distinguishes this installation from `--all`. | Cypress prune: Safe; browser removal: Review; Playwright `--all`: Danger. |

## Evidence rules for future work

- Mark a capability implemented only after source, test, and relevant Windows runtime evidence agree.
- Record actual reclaimed bytes separately from estimates and logical data removed.
- Do not infer Docker, WSL, UAC, installer, auto-update, or background behavior from a successful build.
- Preserve final evidence and material failures; do not turn exploratory attempts into product facts.
