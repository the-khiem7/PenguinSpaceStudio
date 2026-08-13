# Build Artifacts

## Windows executable naming

- Every Windows packaging build must write a versioned executable using the form `out/penguinspace-v<version>.exe` (for example, `out/penguinspace-v0.3.0.exe`).
- The build version is required, explicit, and deterministic. Use a release/semantic version supplied to the build; do not silently substitute a timestamp or reuse an existing versioned filename.
- Never overwrite, delete, rename, or force-replace an existing executable that might be running. Windows locks running `.exe` files; if the destination is locked, report the error and ask the user to close the application before retrying.
- After a successful build, report the exact version and artifact path.
- Preserve prior versioned artifacts unless the user explicitly requests cleanup.
- Do not create or overwrite the unversioned compatibility path `out/penguinspace.exe` automatically. If a compatibility copy is explicitly needed, create it only after the versioned artifact succeeds and only when that destination is not locked.

## Commit composition

When creating a commit for an implementation task, include the related code changes and corresponding docpack/baseline documentation updates in the same commit. Do not split one completed implementation and its docpack update into separate commits.

## Build script changes

When changing build scripts, Docker tasks, or release automation, preserve the versioned-artifact convention above rather than introducing a fixed executable output path.
