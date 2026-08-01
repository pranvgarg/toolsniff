# Releasing

toolsniff uses semantic Git tags as its release source of truth. The source
tree keeps `internal/version.Version` at `dev`; release builds replace it with
the tag through GoReleaser linker flags.

## Version Rules

Use tags in this form:

```text
vMAJOR.MINOR.PATCH
```

- Patch: backwards-compatible bug fix.
- Minor: backwards-compatible feature.
- Major: breaking behavior, CLI, JSON, or registry changes.
- Prerelease: append a suffix such as `-rc.1` or `-beta.1`.

Examples:

```text
v0.1.1
v0.2.0
v1.0.0
v1.0.0-rc.1
```

Do not edit the `dev` fallback for a release.

An untagged local build should continue to report `dev`. To verify the release
version locally without changing source code, inject it at build time:

```bash
go build \
  -ldflags "-X github.com/pranvgarg/toolsniff/internal/version.Version=0.1.0" \
  -o toolsniff \
  .

./toolsniff --version
# 0.1.0
```

The splash screen receives the same value through `main.go` and displays:

```text
toolsniff  v0.1.0
```

## Release Steps

Run the verification suite on the release commit:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

Create and push an annotated tag from the merged `main` branch:

```bash
git checkout main
git pull --ff-only origin main
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

The release workflow only accepts tags matching the semantic-version format.
It then runs GoReleaser, which builds macOS Intel and Apple Silicon binaries,
injects the version, publishes checksums, and creates the Homebrew formula.

## Initial `0.1.0` Release

Run this from the final, merged commit on `main`:

```bash
git checkout main
git pull --ff-only origin main
git status --short

go test ./...
go test -race ./...
go vet ./...
go build ./...

git tag --list v0.1.0
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin main
git push origin v0.1.0
```

The tag push starts `.github/workflows/release.yml`. GoReleaser takes the tag
value, injects `0.1.0` into the binary, and publishes the release artifacts.

## LLM Release Checklist

An LLM preparing a release should follow this order:

1. Read this document and identify the requested semantic version.
2. Confirm the current branch is `main` and inspect `git status --short`.
3. Confirm the intended release commit is already merged into `main`.
4. Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./...`.
5. Confirm the requested tag does not already exist with `git tag --list vX.Y.Z`.
6. Create the annotated tag with `git tag -a vX.Y.Z -m "Release vX.Y.Z"`.
7. Ask for explicit authorization before any network push if authorization was not already provided.
8. Push the branch and tag with `git push origin main` and `git push origin vX.Y.Z`.
9. Report the tag, workflow URL, and release result.

The LLM must never force-push, overwrite an existing tag, tag a dirty worktree,
or silently choose a version. The version must be explicitly supplied by the
user or derived from an approved release plan.

## Version Display

The application uses one canonical display value:

- `toolsniff --version` prints `1.2.3`.
- The splash screen prints `toolsniff  v1.2.3`.
- Local builds print `dev` and `toolsniff  vdev`.

The normalization prevents a `v` prefix from being duplicated if release
tooling passes `v1.2.3` instead of `1.2.3`.
