#!/usr/bin/env bash

set -euo pipefail

usage() {
    cat <<'EOF'
Usage: build-homebrew-bottle.sh --tap PATH --version VERSION [options]

Validate Formula/toolsniff.rb in a Homebrew tap checkout, build a bottle from
it, and write the bottle artifact and brew bottle JSON to an output directory.

Options:
  --tap PATH          Homebrew tap checkout containing Formula/toolsniff.rb
  --version VERSION  Release version, with or without a leading v
  --output-dir PATH  Destination for the bottle and metadata (default: dist/homebrew)
  --root-url URL     Release asset URL prefix (default: toolsniff GitHub release)
  --require-bottle   Require and validate an existing bottle block
  --dry-run          Validate the formula and print brew commands without running them
  -h, --help         Show this help

This command never uses brew bottle --write or --merge and does not modify the
tap checkout. A missing bottle block is allowed for initial bottle generation;
use --require-bottle to validate an already-bottled formula.
EOF
}

die() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

tap_dir=''
version=''
output_dir='dist/homebrew'
root_url=''
require_bottle=false
dry_run=false

while (($# > 0)); do
    case "$1" in
        --tap)
            (($# >= 2)) || die '--tap requires a path'
            tap_dir=$2
            shift 2
            ;;
        --version)
            (($# >= 2)) || die '--version requires a value'
            version=$2
            shift 2
            ;;
        --output-dir)
            (($# >= 2)) || die '--output-dir requires a path'
            output_dir=$2
            shift 2
            ;;
        --root-url)
            (($# >= 2)) || die '--root-url requires a URL'
            root_url=$2
            shift 2
            ;;
        --require-bottle)
            require_bottle=true
            shift
            ;;
        --dry-run)
            dry_run=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            die "unknown argument: $1"
            ;;
    esac
done

[[ -n "$tap_dir" ]] || die '--tap is required'
[[ -n "$version" ]] || die '--version is required'
[[ -d "$tap_dir" ]] || die "tap checkout does not exist: $tap_dir"

version=${version#v}
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
    die "invalid release version: $version"
fi

tap_dir=$(cd "$tap_dir" && pwd -P)
formula="$tap_dir/Formula/toolsniff.rb"
[[ -f "$formula" ]] || die "formula not found: $formula"

if [[ -z "$root_url" ]]; then
    root_url="https://github.com/pranvgarg/toolsniff/releases/download/v$version"
fi
[[ "$root_url" == https://* ]] || die "root URL must use HTTPS: $root_url"

if ! grep -Eq 'class[[:space:]]+Toolsniff[[:space:]]*<[[:space:]]*Formula' "$formula"; then
    die "formula does not define class Toolsniff: $formula"
fi

if ! grep -Fq "/releases/download/v$version/" "$formula"; then
    die "formula has no release asset URL for v$version: $formula"
fi

if ! grep -Eq '^[[:space:]]*url[[:space:]]+"[^"]+\.(tar\.gz|zip)"' "$formula"; then
    die "formula has no archive URL: $formula"
fi

bottle_block_present=false
if grep -Eq '^[[:space:]]*bottle[[:space:]]+do[[:space:]]*$' "$formula"; then
    bottle_block_present=true
fi

if [[ "$bottle_block_present" == true ]]; then
    if ! grep -Fq "root_url \"$root_url\"" "$formula"; then
        die "formula bottle root_url does not match $root_url: $formula"
    fi

    if ! awk '
        /^[[:space:]]*bottle[[:space:]]+do[[:space:]]*$/ {
            in_bottle = 1
            saw_bottle = 1
            saw_sha = 0
            next
        }
        in_bottle && /^[[:space:]]*sha256[[:space:]]/ { saw_sha = 1 }
        in_bottle && /^[[:space:]]*end[[:space:]]*$/ {
            if (saw_sha) {
                valid_bottle = 1
            }
            in_bottle = 0
        }
        END { exit !(saw_bottle && valid_bottle) }
    ' "$formula"; then
        die "formula bottle block has no sha256 entries: $formula"
    fi
elif [[ "$require_bottle" == true ]]; then
    die "formula has no bottle block; omit --require-bottle for initial bottle generation: $formula"
fi

printf 'validated formula: %s\n' "$formula"
printf 'release version: %s\n' "$version"
printf 'bottle root URL: %s\n' "$root_url"
if [[ "$bottle_block_present" == true ]]; then
    printf 'bottle mode: existing bottle block validated\n'
else
    printf 'bottle mode: bootstrap (missing bottle block allowed; brew bottle JSON expected)\n'
fi

if [[ "$dry_run" == true ]]; then
    printf 'dry-run: brew install --build-bottle --formula %q\n' "$formula"
    printf 'dry-run: brew bottle --json --root-url=%q %q\n' "$root_url" "$formula"
    exit 0
fi

command -v brew >/dev/null 2>&1 || die 'brew is required unless --dry-run is used'

mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd -P)
staging_dir=$(mktemp -d "${TMPDIR:-/tmp}/toolsniff-bottle.XXXXXX")
trap 'rm -rf "$staging_dir"' EXIT

(
    cd "$staging_dir"
    HOMEBREW_NO_AUTO_UPDATE=1 brew install --build-bottle --formula "$formula"
    HOMEBREW_NO_AUTO_UPDATE=1 brew bottle --json --root-url="$root_url" "$formula"
)

shopt -s nullglob
artifacts=("$staging_dir"/*.bottle.tar.gz)
metadata=("$staging_dir"/*.bottle.json)
shopt -u nullglob

[[ ${#artifacts[@]} -eq 1 ]] || die "expected one bottle artifact, found ${#artifacts[@]} in $staging_dir"
[[ ${#metadata[@]} -eq 1 ]] || die "expected one bottle JSON file, found ${#metadata[@]} in $staging_dir"

if command -v ruby >/dev/null 2>&1; then
    ruby -rjson -e '
        data = JSON.parse(File.read(ARGV.fetch(0)))
        expected_root = ARGV.fetch(1).sub(%r{/$}, "")
        actual_root = data.fetch("root_url").sub(%r{/$}, "")
        abort "unexpected bottle formula" unless data.fetch("formula") == "toolsniff"
        abort "unexpected bottle root_url" unless actual_root == expected_root
        abort "bottle JSON has no files" unless data.fetch("files").is_a?(Hash) && !data.fetch("files").empty?
    ' "${metadata[0]}" "$root_url" || die "invalid bottle metadata: ${metadata[0]}"
else
    printf 'warning: ruby not found; skipping bottle JSON structure validation\n' >&2
fi

cp "${artifacts[0]}" "$output_dir/"
cp "${metadata[0]}" "$output_dir/"

printf 'BOTTLE_ARTIFACT=%s/%s\n' "$output_dir" "$(basename "${artifacts[0]}")"
printf 'BOTTLE_JSON=%s/%s\n' "$output_dir" "$(basename "${metadata[0]}")"
