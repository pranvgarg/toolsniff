#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/toolsniff-installer-test.XXXXXX")

cleanup() {
    rm -rf "$TEST_ROOT"
}
trap cleanup 0 1 2 15

create_release() {
    release_dir=$1
    archive_name=$2
    checksum=$3

    mkdir -p "$release_dir"
    printf '#!/bin/sh\nprintf "fixture toolsniff\\n"\n' > \
        "$release_dir/toolsniff"
    chmod 755 "$release_dir/toolsniff"
    tar -czf "$release_dir/$archive_name" -C "$release_dir" toolsniff

    if [ "$checksum" = 'valid' ]; then
        (cd "$release_dir" && shasum -a 256 "$archive_name" > checksums.txt)
    else
        printf '%064d  %s\n' 0 "$archive_name" > "$release_dir/checksums.txt"
    fi
}

run_install() {
    machine=$1
    version=$2
    release_dir=$3
    home_dir=$4
    expected_arch=$5

    fake_bin="$TEST_ROOT/fake-bin-$machine"
    mkdir -p "$fake_bin"
    printf '#!/bin/sh\nprintf "%s\\n"\n' "$machine" > "$fake_bin/uname"
    chmod 755 "$fake_bin/uname"

    archive_name="toolsniff_${version}_darwin_${expected_arch}.tar.gz"
    create_release "$release_dir" "$archive_name" valid
    mkdir -p "$home_dir"

    HOME="$home_dir" PATH="$fake_bin:$PATH" \
        TOOLSNIFF_VERSION="$version" \
        TEST_BASE_URL="file://$release_dir" \
        TOOLSNIFF_INSTALL_TESTING=1 \
        "$ROOT_DIR/install.sh"

    [ -x "$home_dir/.local/bin/toolsniff" ]
    [ "$("$home_dir/.local/bin/toolsniff")" = 'fixture toolsniff' ]
}

run_install arm64 0.1.0 "$TEST_ROOT/release-arm64" \
    "$TEST_ROOT/home-arm64" arm64
run_install x86_64 0.1.0 "$TEST_ROOT/release-x86_64" \
    "$TEST_ROOT/home-x86_64" amd64

BAD_RELEASE="$TEST_ROOT/release-bad"
BAD_HOME="$TEST_ROOT/home-bad"
create_release "$BAD_RELEASE" \
    'toolsniff_0.1.0_darwin_arm64.tar.gz' invalid
mkdir -p "$BAD_HOME"
if HOME="$BAD_HOME" PATH="$TEST_ROOT/fake-bin-arm64:$PATH" \
    TEST_BASE_URL="file://$BAD_RELEASE" \
    TOOLSNIFF_INSTALL_TESTING=1 "$ROOT_DIR/install.sh" >/dev/null 2>&1; then
    printf 'expected checksum verification to fail\n' >&2
    exit 1
fi
[ ! -e "$BAD_HOME/.local/bin/toolsniff" ]

if HOME="$TEST_ROOT/home-guard" PATH="$TEST_ROOT/fake-bin-arm64:$PATH" \
    TEST_BASE_URL="file://$TEST_ROOT/release-arm64" \
    "$ROOT_DIR/install.sh" >/dev/null 2>&1; then
    printf 'expected TEST_BASE_URL guard to fail without test mode\n' >&2
    exit 1
fi

printf 'installer tests passed\n'
