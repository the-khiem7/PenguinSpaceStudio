# Docker build environment and Rancher / WSL disk runbook

## Purpose and current status

PenguinSpace is a Windows-first desktop application using Wails 3, Go, Vue, Vite, and Bun. This runbook makes its build environment a Docker-only boundary: the Windows host is an editor and container orchestrator, not a place to install project toolchains or SDKs.

M1 bootstrap is committed on `main` as `5ba3fe8` and contains `Dockerfile`, `docker-compose.yml`, Compose delegates, and a Wails/Go/Vue shell. The elevation probe is implemented across `6ee788d`, `8fc559a`, and the post-consent execution timing fix `d0bc468`. With Rancher Desktop running, the latest Docker verification passed Bun/Vue/Wails binding generation, Go formatting/vet/tests, and a Windows cross-build. Docker build produced `out/penguinspace.exe` (13,556,736 bytes; SHA-256 `f606c7a33c6ee13856db965cea2193c2e43197018fee30b6bbf078921125c4f1`). The rebuilt executable passed visible Windows auto-consent, cancellation, timeout, and 3.00 MiB fixture-lifecycle smoke tests. The host's UAC level is Never notify, so `runas` auto-approved and refusal could not be exercised; that runtime case requires a session that presents a prompt. This host also has unrelated Go and Bun executables installed, so it cannot supply clean-host evidence even though every PenguinSpace build command used Docker. The Dockerfile sets an explicit non-login toolchain `PATH`; use `sh -c` rather than `sh -lc` in diagnostics because a login shell resets `PATH`. Installer/package creation remains separate work.

On 2026-08-11, an explicitly approved host-side `docker builder prune -af` reclaimed 6.151 GB of BuildKit cache. It did not remove images, containers, networks, or volumes. This was an operator maintenance action, not a PenguinSpace product capability; subsequent image builds will repopulate BuildKit cache.

## Mandatory build-environment policy

**All PenguinSpace dependency installation, code generation, verification, tests, builds, and packaging must execute inside Docker.** Do not install or invoke project build toolchains or SDKs directly on the Windows host.

The host may contain and use the editor, Git, Rancher Desktop / Docker, and `docker compose` as the orchestrator. Starting a container from PowerShell is compliant only when the compiler and SDK run inside that container.

Do not install or invoke these tools for PenguinSpace directly on Windows:

- Go: `go`, Go SDK updates, module cache, build cache, or code generators.
- Wails: the Wails 3 CLI, its generated bindings, or desktop build tooling.
- Vue frontend build tooling: Bun installs, Vite, Vue type checking, bundlers, and their package/build caches.
- Windows build SDKs, compilers, linkers, or packaging dependencies added solely to build this project.

Do not work around a missing compiler by installing it on the host. Add or repair the project Docker image instead.

## Required Docker contract

When the application shell is created, commit a Dockerfile and Compose manifest that provide these container services (the exact service names may differ, but their responsibilities may not):

| Service | Required responsibility |
| --- | --- |
| `verify` | Install locked frontend dependencies; format/lint/type-check Vue; run Go formatting, static analysis, and tests. |
| `build` | Produce the Windows application/package using the container-owned Go, Wails, frontend, SDK, and linker environment. |
| `shell` | Provide an interactive, disposable diagnostic shell using the same pinned toolchain image. |

The image must pin compatible Go, Wails 3, Bun, and Windows cross-build dependency versions. Keep all dependency caches in Docker named volumes or container-managed paths, including at minimum:

- Go module and build caches;
- Bun download cache and frontend `node_modules` when practical;
- Wails-generated/build artifacts and any Windows cross-build SDK or linker cache.

The source checkout may be bind-mounted read/write only where build generation requires it. Do not bind-mount the host's Go, Node, Wails, SDK, or cache directories into the containers. A deliberately exported installer may be written to a repository `out/` directory; it is a release artifact, not a toolchain cache.

## Intended local workflow

After the Compose manifest exists, expose a small set of project scripts that delegate to Docker, for example:

```powershell
# These delegates only orchestrate Docker; no project SDK runs on the host.
.\scripts\verify.ps1
.\scripts\build-windows.ps1
.\scripts\shell.ps1
```

Each script must resolve to `docker compose ... run` and must not fall back to host `go`, `wails`, `npm`, or `vite`. CI can use an equivalent container image, but its success does not replace a documented local Docker path.

Before considering the bootstrap complete, verify that a clean Windows host without Go, Wails, Bun build tooling, or Windows build SDKs can run the Docker verification command. M1 recorded container verification plus visible Windows success, cancellation, timeout, and fixture-lifecycle acceptance. Interactive UAC refusal remains required on a host/session that displays the prompt.

## Shared Rancher / WSL storage policy

IRYS and PenguinSpace share the same Rancher Desktop WSL distribution and its Docker storage. The current machine-wide WSL configuration is:

```ini
[wsl2]
defaultVhdSize=20GB
```

This cap applies to new WSL distributions. It is a maximum, not 20 GB reserved on the host, but it is shared by all Docker projects. Keep Kubernetes disabled in Rancher Desktop unless this project requires it.

Check capacity before large dependency or package builds:

```powershell
Get-PSDrive C
docker system df
docker builder du
```

To remove disposable Docker data, while Rancher Desktop is healthy:

```powershell
docker system prune -af
```

This removes unused images, stopped containers, networks, and build cache. It does **not** remove named volumes, but it may require image downloads and rebuilds later. Never use a blanket volume prune as routine cleanup: volumes may hold persistent build or application state.

## Exception process

There is no standing exception for a host toolchain. A temporary host install requires explicit owner approval before installation, a reason Docker cannot perform the work, a cleanup plan, and confirmation after the toolchain/SDK and all of its caches are removed. Record the exception and resolution in this runbook.

## Bootstrap acceptance checklist

- [x] Dockerfile and Compose manifest are committed in `5ba3fe8`.
- [x] Go, Wails 3, Vue/Bun, and Windows build dependencies execute only in containers.
- [x] Go, package-manager, and build caches stay out of the host profile and source checkout.
- [x] `verify`, `build-windows`, and `shell` delegate only to Docker.
- [ ] A clean Windows host completes the Docker verification path. (Container verification passed; clean-host audit remains.)
- [x] Docker space usage and a non-destructive cleanup path are documented; the approved BuildKit-only prune evidence is recorded above.

## References

- [Wails 3](https://v3.wails.io/)
- [Microsoft WSL configuration](https://learn.microsoft.com/windows/wsl/wsl-config)
- [Docker resource pruning](https://docs.docker.com/engine/manage-resources/pruning/)
