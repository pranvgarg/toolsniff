# Releasing Homebrew Bottles

`scripts/build-homebrew-bottle.sh` packages a formula from a local Homebrew tap
checkout without editing that checkout. It validates the formula's release
asset URL, builds the bottle with Homebrew, runs `brew bottle --json`, and
copies the resulting `.bottle.tar.gz` and `.bottle.json` files to the requested
output directory. Initial bottle generation does not require a bottle block;
the generated JSON is the metadata to merge into the tap afterward.

## Prerequisites

The tap checkout must contain `Formula/toolsniff.rb` with:

- A release asset URL containing the requested tag, such as
  `.../releases/download/v1.2.3/...tar.gz`.

By default, a missing bottle block is treated as bootstrap mode. If a bottle
block is present, its `root_url` must match the release asset location and it
must contain at least one `sha256` entry. Use `--require-bottle` to reject a
formula without an existing bottle block and validate an already-bottled tap.
The script does not use `brew bottle --write` or `brew bottle --merge`.

## Build Locally

Run this from the toolsniff checkout after the GitHub release assets for the
version exist:

```bash
scripts/build-homebrew-bottle.sh \
  --tap ../homebrew-toolsniff \
  --version 1.2.3 \
  --output-dir dist/homebrew
```

The default bottle root URL is:

```text
https://github.com/pranvgarg/toolsniff/releases/download/v1.2.3
```

Use `--root-url` when bottles are published to a different HTTPS release asset
location. The command prints `BOTTLE_ARTIFACT` and `BOTTLE_JSON` paths on
success.

To validate a formula and inspect the exact Homebrew commands without running
Homebrew, including whether bootstrap metadata is expected:

```bash
scripts/build-homebrew-bottle.sh \
  --tap ../homebrew-toolsniff \
  --version 1.2.3 \
  --dry-run
```

For an already-bottled formula, add `--require-bottle`:

```bash
scripts/build-homebrew-bottle.sh \
  --tap ../homebrew-toolsniff \
  --version 1.2.3 \
  --require-bottle \
  --dry-run
```

## Publish Sequence

The script produces packaging outputs; it does not publish them or update the
external tap. A release owner must complete both publishing operations:

1. Upload the `.bottle.tar.gz` file to the matching GitHub release.
2. Merge the generated `.bottle.json` checksums into `Formula/toolsniff.rb` in
   the tap, then push that formula change to the tap repository.

For example, a release owner with the appropriate GitHub permissions can upload
the artifact with `gh release upload`. The tap update must be performed in the
tap checkout using the tap's normal review process. The formula's bottle block
must reference the same release asset root URL and the checksums from the JSON;
building an artifact alone is not a complete Homebrew bottle release.

No new GitHub Actions job is included here: the existing release workflow has
`GITHUB_TOKEN` and `HOMEBREW_TAP_TOKEN` patterns for source releases and formula
updates, but it does not safely complete both bottle publication and formula
metadata merging. Keeping those steps explicit avoids presenting a build-only
workflow as a finished bottle publisher.

## Verification

The shell test performs syntax checks, bootstrap validation, existing-bottle
validation, and fake-Homebrew artifact emission. It never runs real
`brew install`:

```bash
scripts/test-build-homebrew-bottle.sh
```
