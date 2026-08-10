---
baseline_schema: "2.0"
pack: "penguin-space"
document: "hallucination"
status: "active"
updated: "2026-08-10"
code_ref: "uncommitted"
---

# Decisions, unknowns, and claim boundaries

## Closed decisions

The following are approved and must not be re-litigated during implementation without explicit owner direction:

- Wails 3, Go, Vue, Windows-first delivery.
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

## Unverified external claims

The following proposal guidance is product research, not runtime-confirmed implementation truth. Revalidate it against official documentation and installed versions before encoding it:

- Exact cleanup commands and behaviour for uv, npm, pnpm, Yarn, Bun, Cypress, Playwright, NuGet, Cargo, Gradle, Maven, and Docker.
- Current Wails 3 API, lifecycle, system-tray, window, packaging, and version-pinning behaviour.
- Windows and WSL/VHDX compaction requirements, privileges, supported paths, and recovery behaviour.
- Competitive product capabilities and differentiation statements.

The preserved source contains the complete research command table and external links: [sources/PROPOSAL.md](sources/PROPOSAL.md#39-research-informed-command-guidance) and [sources/PROPOSAL.md](sources/PROPOSAL.md#40-external-documentation-references).

## Implementation decisions still required

| Topic | Why it is open | Decision evidence required |
|---|---|---|
| Supported tool versions | CLI semantics can differ, especially Yarn and cache maintenance | Official docs plus tested versions |
| Provider command/API detail | Product direction intentionally avoids hard-coding stale commands | Structured output/API evidence and safe plan tests |
| Size calculation and rounding | Needed for credible estimates and history | Design decision and test fixtures |
| Persistence format/location | History, settings, diagnostics, and recovery need a durable model | Windows constraints, privacy decision, migration plan |
| Exact elevation bridge | Wails/Windows implementation detail | Tested UAC success/refusal/cancellation flows |
| VHDX compaction strategy | Safety and support vary by disk/environment | Windows documentation and real non-destructive trials |
| Project-root discovery defaults | Broad scans can be expensive or surprising | User-configurable scope and performance tests |
| Confirmation language | Danger needs deliberate informed consent | UX review and destructive-operation testing |

## Evidence rules for future work

- Mark a capability implemented only after source, test, and relevant Windows runtime evidence agree.
- Record actual reclaimed bytes separately from estimates and logical data removed.
- Do not infer Docker, WSL, UAC, installer, auto-update, or background behavior from a successful build.
- Preserve final evidence and material failures; do not turn exploratory attempts into product facts.
