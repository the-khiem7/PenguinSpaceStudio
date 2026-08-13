---
baseline_schema: "2.0"
pack: "penguin-space"
document: "useguide"
status: "draft"
updated: "2026-08-13"
code_ref: "31bd100 (M3.1 accepted on Windows via v0.1.2)"
---

# Intended product interaction contract

This is the Product Goal interaction contract. M1 implements its left-navigation shell, fixture lifecycle, and partially runtime-verified Windows UAC probe. Commit `163295b` adds the minimal approved workspace-root contract plus Developer Tools workflows for Cargo, Gradle, Maven, and Playwright hermetic local browsers. The working tree adds discovery-first provider availability: only supported host providers are rendered as full cleanup cards at launch; configuration prerequisites are concise notices, unavailable tools are collapsed diagnostics, and project providers appear only after an approved root has a matching marker. It deliberately does not add a Project storage page, broad scanner, filter, analytics, or Docker cache provider. The prior M2 providers—Bun, npm, conditional pnpm, uv, Yarn Classic, NuGet HTTP cache, and Cypress binary cache—remain available.

The UAC panel's **Test Windows consent** action is a fixed no-op probe, not a cleanup request. Commit `8fc559a` disables every start action immediately while a start is pending or active, so a second click cannot surface the previous stale `already in progress` error. Commit `d0bc468` starts the bounded execution window only after Windows elevation launches. **Test cancellation** starts a delayed no-op; select **Cancel probe** while it is active. **Test timeout** outlives the fixed execution window and ends as `timed-out`. On 2026-08-12, computer-controlled acceptance against the rebuilt executable observed `Succeeded`, `Cancelled`, and `Timed-Out`, each with explicit no-cleanup wording where applicable. These controls still cannot invoke a cleanup provider or shell command.

During acceptance, automation may open the application and operate its normal window, but the operator may need to answer the secure UAC prompt manually. The acceptance runner must wait for the operator and must not cancel merely because the response takes time; that wait is now outside the execution timeout. A host configured to Never notify auto-approves `runas`, so it can verify success/cancellation/timeout but cannot provide refusal evidence. Refusal must be tested on a session configured to show the prompt, without automation changing the security setting.

## Navigation and layout

The primary left-sidebar navigation is Home, Developer Tools, Containers & WSL, Projects, History, and Settings. Wide windows show icon and text; compact windows may use icon-focused navigation. Primary navigation is never a web-style top bar.

Use horizontal desktop space deliberately: metric rows, horizontal storage bars, dense lists/tables, master-detail panes, command bars, and dialogs. Avoid giant vertical card stacks and unnecessary route transitions.

## Provider availability at launch

At launch, PenguinSpace performs bounded availability checks only; it does not measure storage, issue a cleanup plan, or remove data. **Available caches** contains supported host providers that can be inspected. **Configuration required** is reserved for detected providers that need a safe prerequisite, such as pnpm needing an explicit `storeDir`. **Unavailable on this machine** is collapsed and retains an explanatory diagnostic rather than creating large empty cards.

After a workspace root is approved, the application checks project markers before showing project provider cards. A root without `Cargo.toml`, Gradle root configuration, `pom.xml`, or `package.json` does not show the corresponding provider. A project built exclusively inside Docker can therefore have no Cargo card: host-provider discovery does not claim to inspect Cargo or build cache that exists only inside containers.

## M3.1 Docker awareness

At launch and on **Refresh Docker**, PenguinSpace performs a bounded read-only daemon inspection. If the daemon is available, **Containers & WSL** shows separate cards for images, stopped containers, BuildKit cache, custom networks, and volumes. Each card distinguishes item-count availability from daemon-reported size and reclaimable-size availability. These values describe daemon storage, not project ownership or physical Windows disk reclaim.

Volumes are marked **Stateful** because they may contain persistent data. No M3.1 card has a review, clean, prune, or delete control. If the Docker CLI is missing or the daemon cannot be reached, the surface reports that state without showing stale resource claims. Partial category failures appear as warnings and unavailable values. A future cleanup workflow must not reuse this observation surface as authorization.

Windows acceptance on `out/penguinspace-v0.1.2.exe` confirms both states. With Docker 29.5.3 available, the surface displayed exactly five resource categories and Stateful volumes without cleanup controls. After the operator stopped Rancher Desktop, **Refresh Docker** displayed **Daemon unavailable / Docker resources were not inspected** and removed the previous cards. Rancher remains stopped; request that the operator restart it before the next Docker runtime check.

## Scan-to-clean flow

```text
Launch → Scan → Measure → Classify → Dashboard → Review & Clean → Cleanup plan → Confirm → Execute → Verify → Result and history
```

The application must not run cleanup as part of scanning or at launch. Home is observability and routes the user to **Review & Clean**.

M1's only runnable lifecycle is deliberately non-destructive: it scans one in-memory fixture, produces one Safe plan, requires backend confirmation, mutates no filesystem/tool state, verifies exact fixture bytes, and records history. It demonstrates the boundary only; it is not user-facing cleanup support.

The Bun card is the first real cleanup workflow. **Inspect Bun cache** detects only supported Bun 1.x installations, measures logical cache bytes, and displays the cache location plus Safe/Download consequence text. **Review Bun cleanup** reveals an explicit second-stage choice. **Cancel** must close review without executing or invalidating the displayed measurement. **Confirm and clear** may invoke the backend-owned plan; the backend rejects a missing plan or a cache path that changed after review, then remeasures logical bytes and records the outcome. The UI must never describe the logical result as physical reclaim because Bun may hardlink cache entries on Windows.

The npm card uses the same review contract for npm 10/11 but is Review/Download rather than Safe. It measures only `_cacache`, not `_logs`, npx cache, or arbitrary cache-root files. Its consequence must say that `--force` is required and future installs may download packages again. Windows acceptance must preserve the measurement when review is cancelled; real user cache deletion still requires action-time confirmation.

The pnpm card supports pnpm 11/12 and must distinguish detection from actionable support. Without an explicit absolute `storeDir`, it explains that pnpm's default stores are per disk and that project-root drive context is deferred to M4; it shows no measured size, no estimate, and no review button. With an explicit `storeDir`, it shows the versioned store's observed logical size, labels the reclaim estimate unavailable, and offers a Safe/Download review for `pnpm store prune`. Confirmation is still mandatory. After execution, the card reports the before/after difference as verified logical bytes; it must never relabel the entire pre-prune store as reclaimable.

The uv card supports uv 0.12.x. **Inspect uv cache** resolves the actual cache location and shows its total observed size, but labels the pre-prune reclaim estimate unavailable because `uv cache prune` decides what is unused. The Safe/Download consequence must disclose that prune also removes all cached centralized project environments, which will be recreated, and that future work may download packages or rebuild source distributions. Review and explicit confirmation remain mandatory. Windows acceptance detected uv 0.12.1, displayed 3.45 GiB, opened review, and cancelled with the measurement intact; the real user cache was not pruned. The common command runner suppresses child console windows; the 2026-08-13 Windows inspection observed no Windows Terminal or console flash.

The Yarn Classic card supports only version 1.x. **Inspect Yarn cache** resolves the global cache with `yarn cache dir`; **Confirm and clear** invokes only `yarn cache clean`, after revalidating that same global location. It is Review/Download because packages may need downloading again. Yarn modern cache modes are explicitly not inspected or cleaned because they can be project-local or shared and need M4 workspace scope.

The NuGet card supports .NET SDK 6 or later. **Inspect NuGet HTTP cache** lists and measures only the `http-cache` root; **Confirm and clear** invokes only `dotnet nuget locals http-cache --clear`. It is Safe/Download. Global packages, temporary resources, and plugins cache are excluded and must never be presented as part of this action.

The Cypress card supports versions 13 through 15. **Inspect Cypress cache** resolves the binary cache with `cypress cache path`, shows its total observed size, and labels the pre-prune reclaim estimate unavailable. **Confirm and prune** invokes only `cypress cache prune`, which retains the version in use while removing other cached binaries. It is Safe/Download; Cypress App Data and project dependencies remain outside this action.

## Home contract

Home answers total developer storage, estimated reclaimable bytes, Safe-to-clean amount, needs-review amount, domain breakdown, largest consumers, top reclaim opportunities, last scan metadata, and—when present—virtual-disk metrics.

Largest consumer and largest reclaim opportunity are distinct. Estimated VHDX compactability is displayed separately from ordinary reclaimable cache totals until compaction finishes.

## Review and confirmation

Each cleanup row must show item/provider, current and estimated reclaimable size, risk, recovery cost, consequence, privilege requirement, and required environment/process shutdown.

- Safe: disposable data, but state the recovery cost.
- Review: source/state is retained but a rebuild or download is expected.
- Danger: never preselected; visually distinct; selected only deliberately and confirmed in a separate step that cannot be disabled by ordinary confirmation preferences.

VHDX workflows must clearly explain the distinction between cleaning data inside the environment and shrinking physical Windows host allocation. UAC requirements must be announced at planning time.

## Results, history, and tone

Results report actual outcome: before/after measurements, actual reclaimed bytes, individual actions, failures, and a refreshed state. For VHDX, report logical deletion and physical before/after separately.

Maintain a calm technical tone. Use factual statements such as “14.8 GB reclaimable” and consequences such as “Projects may rebuild more slowly next time.” Do not use scare tactics, health scores, boost claims, or aggressive-cleaner language.

## Settings contract

Settings covers general behavior, workspace roots, scan exclusions/ignore rules, cleanup safety defaults, and advanced diagnostics/behaviour. Broad project scanning must remain intentional and configurable.

For the full page-level direction, examples, layouts, and surface list, see the preserved proposal sections 10–20 and 24–37 in [sources/PROPOSAL.md](sources/PROPOSAL.md).
