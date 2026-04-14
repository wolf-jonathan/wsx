# Releasing wsx

This repo uses Git tags, GitHub Actions, and GoReleaser to publish release
binaries for Windows, Linux, and macOS. End users do not need Go installed when
they install from a GitHub Release asset.

## Versioning

There are two separate versions in this repo:

- CLI release version: the user-facing app version, derived from the Git tag
- Workspace config schema version: the `"version"` field inside `.wsx.json`

### CLI release version

The CLI version is injected at build time by GoReleaser.

- Development builds report `dev`
- Tagged releases report the tag version such as `v0.1.0`

Use Semantic Versioning for release tags:

- `v0.1.0` for an initial release
- `v0.1.1` for bug fixes
- `v0.2.0` for backward-compatible features
- `v1.0.0` when the CLI surface and workspace model are stable

### Workspace config schema version

The `.wsx.json` `"version"` is the workspace config format version. It is not
the CLI release version.

Only change the config schema version when the `.wsx.json` format changes in a
way that requires compatibility handling or migration.

## Release Prerequisites

Before tagging a release:

1. Run the full test suite.
2. Confirm CLI help still matches the implementation.
3. Update `README.md` and other public docs if behavior changed.
4. Commit all intended release changes to `main`.

Recommended local checks:

```powershell
go test ./...
go run . --help
go run . init --help
go run . add --help
go run . list --help
```

## How Releases Work

The release workflow is defined in `.github/workflows/release.yml`.

When you push a tag matching `v*`:

1. GitHub Actions checks out the repo.
2. GoReleaser runs the configured test hook.
3. GoReleaser builds `wsx` for:
   - Windows `amd64`, `arm64`
   - Linux `amd64`, `arm64`
   - macOS `amd64`, `arm64`
4. GoReleaser creates archives and checksums.
5. GitHub creates or updates the release and uploads the assets.

## Release Steps

Example for releasing `v0.1.0`:

```powershell
git checkout main
git pull
go test ./...
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

After the workflow finishes, verify the release page contains the expected
archives and `checksums.txt`.

## Release Assets

GitHub Releases should contain platform-specific archives named like:

- `wsx_v0.1.0_windows_amd64.zip`
- `wsx_v0.1.0_linux_amd64.tar.gz`
- `wsx_v0.1.0_darwin_arm64.tar.gz`

Users can download the archive for their platform, extract it, and place the
binary on their `PATH`.

## Rollback And Fixes

If a tagged release has a problem, do not rewrite the tag. Cut a new patch
release instead, for example `v0.1.1`.

## Notes

- `README.md`, CLI help output, and tests should stay aligned.
- The release version comes from the Git tag, not from a manually edited source
  file.
- The workspace config schema version should remain stable unless the config
  format changes.
