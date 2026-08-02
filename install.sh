#!/bin/sh

# Install the published toolsniff binary without requiring Go or Homebrew.
#
# Defaults:
#   TOOLSNIFF_VERSION=0.1.0
#
# Test-only override:
#   TEST_BASE_URL=file:///path/to/release-directory \
#   TOOLSNIFF_INSTALL_TESTING=1 ./install.sh
#
# TEST_BASE_URL is intentionally rejected unless TOOLSNIFF_INSTALL_TESTING=1 is
# also set. Normal installs always use the HTTPS GitHub release URL.

set -eu

REPOSITORY_URL='https://github.com/pranvgarg/toolsniff'
VERSION=${TOOLSNIFF_VERSION:-0.1.0}

fail() {
    printf 'install.sh: %s\n' "$1" >&2
    exit 1
}

case "$VERSION" in
    ''|*[!0-9A-Za-z.-]*)
        fail "invalid TOOLSNIFF_VERSION: $VERSION"
        ;;
esac

if [ -z "${HOME:-}" ]; then
    fail 'HOME must be set'
fi

MACHINE=$(uname -m) || fail 'unable to determine the machine architecture'
case "$MACHINE" in
    arm64)
        ARCH=arm64
        ;;
    x86_64)
        ARCH=amd64
        ;;
    *)
        fail "unsupported architecture: $MACHINE (expected arm64 or x86_64)"
        ;;
esac

TAG="v$VERSION"
ARCHIVE_NAME="toolsniff_${VERSION}_darwin_${ARCH}.tar.gz"
CHECKSUMS_NAME='checksums.txt'

if [ -n "${TEST_BASE_URL:-}" ]; then
    [ "${TOOLSNIFF_INSTALL_TESTING:-}" = '1' ] || \
        fail 'TEST_BASE_URL requires TOOLSNIFF_INSTALL_TESTING=1'

    BASE_URL=${TEST_BASE_URL%/}
    case "$BASE_URL" in
        file://*|https://*)
            ;;
        *)
            fail 'TEST_BASE_URL must use file:// or https://'
            ;;
    esac
    TEST_OVERRIDE=1
else
    BASE_URL="$REPOSITORY_URL/releases/download/$TAG"
    TEST_OVERRIDE=0
fi

command -v curl >/dev/null 2>&1 || fail 'curl is required'
command -v tar >/dev/null 2>&1 || fail 'tar is required'

TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/toolsniff-install.XXXXXX") || \
    fail 'unable to create a temporary directory'

cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup 0 1 2 15

download() {
    URL=$1
    DESTINATION=$2

    if [ "$TEST_OVERRIDE" -eq 1 ]; then
        curl --fail --location --silent --show-error \
            "$URL" --output "$DESTINATION"
    else
        curl --fail --location --silent --show-error --retry 3 \
            --retry-delay 1 --proto '=https' --tlsv1.2 \
            "$URL" --output "$DESTINATION"
    fi
}

ARCHIVE_PATH="$TMP_DIR/$ARCHIVE_NAME"
CHECKSUMS_PATH="$TMP_DIR/$CHECKSUMS_NAME"

printf 'Downloading toolsniff %s for %s...\n' "$VERSION" "$MACHINE"
download "$BASE_URL/$ARCHIVE_NAME" "$ARCHIVE_PATH" || \
    fail 'unable to download the release archive'
download "$BASE_URL/$CHECKSUMS_NAME" "$CHECKSUMS_PATH" || \
    fail 'unable to download checksums.txt'

sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{print $1}'
    else
        fail 'sha256sum or shasum is required for checksum verification'
    fi
}

EXPECTED_SHA=$(awk -v file="$ARCHIVE_NAME" \
    '$2 == file { print $1; exit }' "$CHECKSUMS_PATH")
case "$EXPECTED_SHA" in
    ''|*[!0-9A-Fa-f]*)
        fail "no valid SHA-256 checksum found for $ARCHIVE_NAME"
        ;;
esac
[ "${#EXPECTED_SHA}" -eq 64 ] || \
    fail "invalid SHA-256 checksum found for $ARCHIVE_NAME"

EXPECTED_SHA=$(printf '%s' "$EXPECTED_SHA" | tr 'A-F' 'a-f')
ACTUAL_SHA=$(sha256_file "$ARCHIVE_PATH")
if [ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]; then
    fail "SHA-256 checksum mismatch for $ARCHIVE_NAME"
fi
printf 'Checksum verified.\n'

EXTRACT_DIR="$TMP_DIR/extracted"
mkdir -p "$EXTRACT_DIR"
tar -xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR" || \
    fail 'unable to extract the verified release archive'
[ -f "$EXTRACT_DIR/toolsniff" ] || \
    fail 'release archive does not contain toolsniff'

INSTALL_DIR="$HOME/.local/bin"
INSTALL_PATH="$INSTALL_DIR/toolsniff"
STAGED_PATH="$TMP_DIR/toolsniff"
mkdir -p "$INSTALL_DIR" || fail "unable to create $INSTALL_DIR"
cp "$EXTRACT_DIR/toolsniff" "$STAGED_PATH" || \
    fail 'unable to stage toolsniff'
chmod 755 "$STAGED_PATH" || fail 'unable to make toolsniff executable'
mv "$STAGED_PATH" "$INSTALL_PATH" || \
    fail "unable to install toolsniff to $INSTALL_PATH"

printf 'Installed toolsniff %s to %s\n' "$VERSION" "$INSTALL_PATH"
case ":${PATH:-}:" in
    *:"$INSTALL_DIR":*)
        printf 'PATH already includes %s\n' "$INSTALL_DIR"
        ;;
    *)
        printf 'Add toolsniff to PATH for future shells:\n'
        printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
        ;;
esac
