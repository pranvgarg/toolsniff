# Homebrew Compatibility and Installation

## Current Release

toolsniff `v0.1.0` is published through:

- Main repository: `https://github.com/pranvgarg/toolsniff`
- Release: `https://github.com/pranvgarg/toolsniff/releases/tag/v0.1.0`
- Custom tap: `https://github.com/pranvgarg/homebrew-toolsniff`

The release contains prebuilt macOS binaries for:

- Apple Silicon (`arm64`)
- Intel (`amd64`)

## Correct Homebrew Installation

The formula currently lives in a custom tap, not Homebrew Core. The formula is
the preferred Homebrew path and uses the release's prebuilt macOS binary. A
fresh Mac must reference the tap before installing it:

```bash
brew tap pranvgarg/toolsniff
brew trust pranvgarg/toolsniff
brew install pranvgarg/toolsniff/toolsniff
```

The `brew trust` command is required by newer Homebrew versions for custom
taps. Older Homebrew versions may not have this command; in that case, skip it
and continue with the install.

After installation:

```bash
toolsniff --version
```

Expected version:

```text
0.1.0
```

Verify the installation directly:

```bash
toolsniff --version
```

The tap also contains a cask fallback. Use it when the formula bottle is not
usable on a particular Homebrew/macOS combination:

```bash
brew install --cask pranvgarg/toolsniff/toolsniff
```

Install either the formula or the cask, not both. toolsniff detects which
Homebrew source owns the installation and keeps formula and cask observations
separate.

Do not use a bare command on a fresh laptop:

```bash
brew install toolsniff
```

That command searches Homebrew Core and already-installed taps. It will not
find toolsniff until the formula is submitted to Homebrew Core.

## Direct Installer

The repository also contains `install.sh` for machines where Homebrew is not
available or cannot satisfy its local prerequisites. It requires neither Go
nor Homebrew. The script detects `arm64` or `x86_64`, downloads the matching
release archive, verifies `checksums.txt`, and installs to
`~/.local/bin/toolsniff`:

```bash
curl -fsSL https://raw.githubusercontent.com/pranvgarg/toolsniff/main/install.sh | sh
```

From a checkout, the equivalent command is:

```bash
./install.sh
```

Add `~/.local/bin` to `PATH` when the installer tells you to. This is a direct
binary installation, not a Homebrew installation; rerun `install.sh` for a
newer release instead of using the Homebrew self-update command.

## Homebrew Self-Update

The self-update command applies only to a toolsniff installation managed by
Homebrew:

```bash
toolsniff --update
```

toolsniff first verifies that `brew` is available, detects whether the binary
was installed as a formula or cask, and checks that same source for an
outdated release. It prompts only when an update is available. Formula and
cask upgrades use their source-specific Homebrew commands:

```text
formula: brew upgrade toolsniff
cask:    brew upgrade --cask toolsniff
```

For automation, skip the confirmation prompt:

```bash
toolsniff --update --yes
```

If both the formula and cask are installed, the installation is ambiguous and
the update stops rather than choosing one arbitrarily. If neither is installed,
or if the binary came from `install.sh` or a source build, `--update` does not
apply.

## What the Installation Error Means

There are two different errors users may see.

### Formula Not Found

```text
Error: No formulae or casks found for toolsniff.
```

This means the custom tap was not referenced. Use:

```bash
brew tap pranvgarg/toolsniff
brew install pranvgarg/toolsniff/toolsniff
```

### Command Line Tools Outdated

```text
Error: Your Command Line Tools are too outdated.
```

This is a local Mac/Homebrew prerequisite issue, not a toolsniff release or
formula issue. The formula already downloaded and verified the correct
prebuilt binary before Homebrew stopped.

Check the current state:

```bash
xcode-select -p
brew config
```

Update the Apple Command Line Tools through Software Update:

```bash
softwareupdate --list
```

Install the available Command Line Tools update using the exact label shown by
that command. For example:

```bash
sudo softwareupdate --install "Command Line Tools for Xcode 26.6-26.6"
```

If System Settings says the tools are already installed but Homebrew still
reports an outdated version, update Command Line Tools through:

```text
System Settings > General > Software Update
```

Only if the standalone installation is stuck should it be removed and
reinstalled:

```bash
sudo rm -rf /Library/Developer/CommandLineTools
sudo xcode-select --install
```

Retry after updating:

```bash
brew update
brew install pranvgarg/toolsniff/toolsniff
```

## Self-Discovery After Installation

After Homebrew installs toolsniff, toolsniff may report itself through two
truthful observations:

```text
BREW-FORMULA
  toolsniff       installed

PATH
  toolsniff       available at /opt/homebrew/bin/toolsniff
```

These are not duplicate installation claims:

- `brew-formula` means Homebrew installed the tool.
- `path` means the command is available to the shell.

toolsniff does not execute itself recursively. It only reads Homebrew's
inventory and enumerates executable files from PATH.

## Compatibility Behavior

### Older Homebrew Versions

The install instructions should support both Homebrew trust models:

```bash
brew tap pranvgarg/toolsniff

if brew help trust >/dev/null 2>&1; then
  brew trust pranvgarg/toolsniff
fi

brew install pranvgarg/toolsniff/toolsniff
```

This avoids requiring `brew trust` on older Homebrew versions while remaining
safe on newer versions.

The same compatibility rule applies to both the formula and cask. Homebrew
version compatibility does not change the toolsniff binary's source-build
requirement: the published formula, cask, and direct installer all use a
prebuilt release binary.

### Older macOS and Command Line Tools

Homebrew cannot reliably bypass an outdated or broken Apple developer-tools
installation. The project should define and document a supported macOS
version matrix rather than pretending every old Mac is supported.

For machines that cannot update Homebrew prerequisites, use the direct binary
installer described above as the alternative distribution channel.

### Older toolsniff Configurations

Configuration compatibility should follow these rules:

- New TOML fields must be optional.
- Missing theme settings use the default theme.
- Missing source roles use compatibility defaults where possible.
- Existing registry files must remain readable.
- Registry format changes require an explicit migration.

Add a future registry schema version before changing persisted data:

```json
{
  "schema_version": 1,
  "tools": []
}
```

## Additional Distribution Options

### Homebrew Core

Submit the formula to `Homebrew/homebrew-core`. If accepted, users can use:

```bash
brew install toolsniff
```

This removes the custom-tap step but does not guarantee installation on a Mac
with outdated Command Line Tools.

### Direct Binary Installer (Current)

`install.sh` currently:

1. Detects Apple Silicon or Intel.
2. Downloads the matching GitHub release archive.
3. Verifies the checksum.
4. Installs to `~/.local/bin/toolsniff`.
5. Explains how to add that directory to PATH.

This path does not require Go or Homebrew. It is independent of Homebrew and
is not updated by `toolsniff --update`.

### Go Installation

Developer fallback:

```bash
go install github.com/pranvgarg/toolsniff@v0.1.0
```

This requires Go and is not the recommended end-user path.

## Recommended Next Steps

1. Keep the custom tap installation as the supported `v0.1.0` path.
2. Publish each formula bottle manually by building the bottle, uploading the
   archive to the GitHub release, and merging the generated checksum metadata
   into the tap formula; see `docs/releasing.md`.
3. Submit the formula to Homebrew Core when the project meets its criteria.
4. Add `toolsniff --doctor` to diagnose Homebrew, PATH, config, registry, and
   Command Line Tools issues.
5. Add registry schema versioning before changing persisted data.
6. Define a supported macOS and Homebrew compatibility matrix.
