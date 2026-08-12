---
baseline_schema: "2.0"
pack: "penguin-space"
document: "useguide"
status: "draft"
updated: "2026-08-12"
code_ref: "d0bc468"
---

# Intended product interaction contract

This is the Product Goal interaction contract. M1 now implements its left-navigation shell, a visible fixture lifecycle panel, and a partially runtime-verified Windows UAC probe panel; all other pages and real cleanup workflows remain planned.

The UAC panel's **Test Windows consent** action is a fixed no-op probe, not a cleanup request. Commit `8fc559a` disables every start action immediately while a start is pending or active, so a second click cannot surface the previous stale `already in progress` error. Commit `d0bc468` starts the bounded execution window only after Windows elevation launches. **Test cancellation** starts a delayed no-op; select **Cancel probe** while it is active. **Test timeout** outlives the fixed execution window and ends as `timed-out`. On 2026-08-12, computer-controlled acceptance against the rebuilt executable observed `Succeeded`, `Cancelled`, and `Timed-Out`, each with explicit no-cleanup wording where applicable. These controls still cannot invoke a cleanup provider or shell command.

During acceptance, automation may open the application and operate its normal window, but the operator may need to answer the secure UAC prompt manually. The acceptance runner must wait for the operator and must not cancel merely because the response takes time; that wait is now outside the execution timeout. A host configured to Never notify auto-approves `runas`, so it can verify success/cancellation/timeout but cannot provide refusal evidence. Refusal must be tested on a session configured to show the prompt, without automation changing the security setting.

## Navigation and layout

The primary left-sidebar navigation is Home, Developer Tools, Containers & WSL, Projects, History, and Settings. Wide windows show icon and text; compact windows may use icon-focused navigation. Primary navigation is never a web-style top bar.

Use horizontal desktop space deliberately: metric rows, horizontal storage bars, dense lists/tables, master-detail panes, command bars, and dialogs. Avoid giant vertical card stacks and unnecessary route transitions.

## Scan-to-clean flow

```text
Launch → Scan → Measure → Classify → Dashboard → Review & Clean → Cleanup plan → Confirm → Execute → Verify → Result and history
```

The application must not run cleanup as part of scanning or at launch. Home is observability and routes the user to **Review & Clean**.

M1's only runnable lifecycle is deliberately non-destructive: it scans one in-memory fixture, produces one Safe plan, requires backend confirmation, mutates no filesystem/tool state, verifies exact fixture bytes, and records history. It demonstrates the boundary only; it is not user-facing cleanup support.

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
