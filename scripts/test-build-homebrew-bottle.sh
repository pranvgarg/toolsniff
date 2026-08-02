#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
script="$script_dir/build-homebrew-bottle.sh"
test_root=$(mktemp -d "${TMPDIR:-/tmp}/toolsniff-bottle-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT

bash -n "$script"

tap_dir="$test_root/tap"
mkdir -p "$tap_dir/Formula"
cat > "$tap_dir/Formula/toolsniff.rb" <<'EOF'
class Toolsniff < Formula
  desc "Discover installed developer and AI tools on macOS"
  homepage "https://github.com/pranvgarg/toolsniff"
  url "https://github.com/pranvgarg/toolsniff/releases/download/v1.2.3/toolsniff_1.2.3_Darwin_arm64.tar.gz"
  sha256 "source-sha256"
end
EOF

dry_run_output=$(
    "$script" \
        --tap "$tap_dir" \
        --version v1.2.3 \
        --output-dir "$test_root/output" \
        --dry-run
)

grep -Fq 'validated formula:' <<<"$dry_run_output"
grep -Fq 'bottle mode: bootstrap (missing bottle block allowed; brew bottle JSON expected)' <<<"$dry_run_output"
grep -Fq 'dry-run: brew install --build-bottle' <<<"$dry_run_output"
grep -Fq 'dry-run: brew bottle --json' <<<"$dry_run_output"

if "$script" --tap "$tap_dir" --version 1.2.3 --require-bottle --dry-run >/dev/null 2>&1; then
    printf 'expected --require-bottle to reject a formula without bottle metadata\n' >&2
    exit 1
fi

fake_bin="$test_root/bin"
mkdir -p "$fake_bin"
cat > "$fake_bin/brew" <<'EOF'
#!/usr/bin/env bash

set -euo pipefail

case "${1:-}" in
    install)
        # This fake replaces Homebrew for the test; no real install is run.
        exit 0
        ;;
    bottle)
        touch toolsniff--1.2.3.arm64_sequoia.bottle.tar.gz
        printf '%s\n' '{"formula":"toolsniff","version":"1.2.3","root_url":"https://github.com/pranvgarg/toolsniff/releases/download/v1.2.3","files":{"toolsniff--1.2.3.arm64_sequoia.bottle.tar.gz":{"sha256":{"arm64_sequoia":"bottle-sha256"}}}}' > toolsniff--1.2.3.arm64_sequoia.bottle.json
        ;;
    *)
        printf 'unexpected fake brew command: %s\n' "${1:-}" >&2
        exit 1
        ;;
esac
EOF
chmod +x "$fake_bin/brew"

output_dir="$test_root/output"
PATH="$fake_bin:$PATH" "$script" \
    --tap "$tap_dir" \
    --version 1.2.3 \
    --output-dir "$output_dir" >/dev/null

[[ -f "$output_dir/toolsniff--1.2.3.arm64_sequoia.bottle.tar.gz" ]]
[[ -f "$output_dir/toolsniff--1.2.3.arm64_sequoia.bottle.json" ]]

cat > "$tap_dir/Formula/toolsniff.rb" <<'EOF'
class Toolsniff < Formula
  desc "Discover installed developer and AI tools on macOS"
  homepage "https://github.com/pranvgarg/toolsniff"
  url "https://github.com/pranvgarg/toolsniff/releases/download/v1.2.3/toolsniff_1.2.3_Darwin_arm64.tar.gz"
  sha256 "source-sha256"

  bottle do
    root_url "https://github.com/pranvgarg/toolsniff/releases/download/v1.2.3"
    sha256 cellar: :any_skip_relocation, arm64_sequoia: "bottle-sha256"
  end
end
EOF

existing_bottle_output=$(
    "$script" \
        --tap "$tap_dir" \
        --version 1.2.3 \
        --require-bottle \
        --dry-run
)
grep -Fq 'bottle mode: existing bottle block validated' <<<"$existing_bottle_output"

if "$script" --tap "$tap_dir" --version 1.2.4 --dry-run >/dev/null 2>&1; then
    printf 'expected a release-version mismatch to fail\n' >&2
    exit 1
fi

printf 'homebrew bottle script syntax, bootstrap, and existing-bottle checks passed\n'
