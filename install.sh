#!/usr/bin/env sh
# install.sh — one-line installer for striatum (GH #160).
#
# Downloads a prebuilt release archive from GitHub Releases (no Go toolchain,
# no clone), verifies it against SHA256SUMS, and installs the three binaries:
# striatum, striatumd, striatum-supervisor-helper.
#
#   curl -fsSL https://raw.githubusercontent.com/halbritt/striatum/main/install.sh | sh
#
# Pass flags after `| sh -s --`, e.g.:
#   curl -fsSL .../install.sh | sh -s -- --prefix /usr/local/bin --version v2.15.0
#
# IMPORTANT: installing a new striatumd does NOT update a daemon that is
# already running — the old image stays in memory (/proc/<pid>/exe shows it
# as deleted). The installer detects this and tells you to restart; pass
# --restart-daemon to let it run `systemctl --user restart striatumd` itself.
#
# For a from-source build instead, clone the repo and run `make install`.

set -eu

REPO="halbritt/striatum"
PREFIX="${HOME}/.local/bin"
VERSION=""              # empty => latest release
RESTART_DAEMON="no"     # yes => systemctl --user restart striatumd after install

usage() {
    cat <<EOF
install.sh — install prebuilt striatum from GitHub Releases.

Usage:
    curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh
    curl -fsSL .../install.sh | sh -s -- [options]

Options:
    --prefix DIR        Install binaries into DIR. Default: ${HOME}/.local/bin.
    --version vX.Y.Z    Install a specific release. Default: latest.
    --restart-daemon    Run 'systemctl --user restart striatumd' after install
                        when a running daemon is detected. Default: print the
                        command and leave the daemon alone.
    -h, --help          Show this help and exit.

Environment:
    STRIATUM_BASE_URL   Override the asset base URL (mirror or local test,
                        e.g. file:///path/to/dist). Requires --version.
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --prefix) PREFIX="${2:?--prefix needs a value}"; shift 2 ;;
        --prefix=*) PREFIX="${1#--prefix=}"; shift ;;
        --version) VERSION="${2:?--version needs a value}"; shift 2 ;;
        --version=*) VERSION="${1#--version=}"; shift ;;
        --restart-daemon) RESTART_DAEMON="yes"; shift ;;
        -h|--help) usage; exit 0 ;;
        *) echo "install.sh: unknown argument: $1" >&2; echo "Run with --help for usage." >&2; exit 2 ;;
    esac
done

# --- platform detection -------------------------------------------------------

os="$(uname -s)"
case "$os" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    *) echo "install.sh: unsupported OS '$os' (striatum ships linux and darwin archives)" >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "install.sh: unsupported architecture '$arch' (striatum ships amd64 and arm64 archives)" >&2; exit 1 ;;
esac

# --- helpers ------------------------------------------------------------------

have() { command -v "$1" >/dev/null 2>&1; }

download() { # url dest
    if have curl; then
        curl -fsSL "$1" -o "$2"
    elif have wget; then
        wget -qO "$2" "$1"
    else
        echo "install.sh: need curl or wget to download" >&2; exit 1
    fi
}

sha256_of() { # file -> stdout hash
    if have sha256sum; then
        sha256sum "$1" | awk '{print $1}'
    elif have shasum; then
        shasum -a 256 "$1" | awk '{print $1}'
    else
        echo ""
    fi
}

# --- resolve version ----------------------------------------------------------

if [ -n "$VERSION" ]; then
    case "$VERSION" in v*) ;; *) VERSION="v${VERSION}" ;; esac
fi

if [ -z "$VERSION" ] && [ -z "${STRIATUM_BASE_URL:-}" ]; then
    # Read the tag from the /releases/latest redirect — no JSON parsing needed.
    if have curl; then
        VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" | sed 's#.*/tag/##')"
    fi
    [ -n "$VERSION" ] || { echo "install.sh: could not resolve the latest version; pass --version vX.Y.Z" >&2; exit 1; }
fi
[ -n "$VERSION" ] || { echo "install.sh: STRIATUM_BASE_URL requires --version vX.Y.Z" >&2; exit 1; }

bare_version="${VERSION#v}"
ASSET_BASE="${STRIATUM_BASE_URL:-https://github.com/${REPO}/releases/download/${VERSION}}"
TARBALL="striatum_${bare_version}_${os}-${arch}.tar.gz"

# --- download + verify --------------------------------------------------------

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "==> Downloading ${TARBALL} (${VERSION})"
download "${ASSET_BASE}/${TARBALL}" "${tmp}/${TARBALL}"
download "${ASSET_BASE}/SHA256SUMS" "${tmp}/SHA256SUMS"

# SHA256SUMS entries carry a ./ prefix (./striatum_X.Y.Z_os-arch.tar.gz).
expected="$(awk -v f="${TARBALL}" '{ name=$2; sub(/^\.\//, "", name); if (name == f) print $1 }' "${tmp}/SHA256SUMS")"
actual="$(sha256_of "${tmp}/${TARBALL}")"
if [ -z "$expected" ]; then
    echo "install.sh: no checksum for ${TARBALL} in SHA256SUMS" >&2; exit 1
elif [ -z "$actual" ]; then
    echo "install.sh: no sha256 tool (sha256sum/shasum) found — cannot verify download" >&2; exit 1
elif [ "$expected" != "$actual" ]; then
    echo "install.sh: checksum mismatch for ${TARBALL}" >&2
    echo "  expected ${expected}" >&2
    echo "  actual   ${actual}" >&2
    exit 1
fi
echo "==> Checksum OK"

# --- extract + install binaries ------------------------------------------------

tar -xzf "${tmp}/${TARBALL}" -C "${tmp}"
extract="${tmp}/striatum_${bare_version}_${os}-${arch}"   # archives wrap in this dir

mkdir -p "${PREFIX}"
for bin in striatum striatumd striatum-supervisor-helper; do
    if [ ! -f "${extract}/bin/${bin}" ]; then
        echo "install.sh: archive is missing bin/${bin}" >&2; exit 1
    fi
    cp "${extract}/bin/${bin}" "${PREFIX}/${bin}"
    chmod 0755 "${PREFIX}/${bin}"
done
echo "==> Installed striatum, striatumd, striatum-supervisor-helper into ${PREFIX}"

case ":${PATH}:" in
    *":${PREFIX}:"*) ;;
    *) echo "NOTE: ${PREFIX} is not on your PATH; add it to your shell profile." ;;
esac

# --- daemon restart nudge (the make-install trap) ------------------------------
# A running striatumd keeps executing the OLD binary image after the file is
# replaced; the upgrade only takes effect after a restart.

daemon_running="no"
if have systemctl && systemctl --user is-active --quiet striatumd 2>/dev/null; then
    daemon_running="yes"
elif have pgrep && pgrep -x striatumd >/dev/null 2>&1; then
    daemon_running="yes"
fi

if [ "$daemon_running" = "yes" ]; then
    if [ "$RESTART_DAEMON" = "yes" ] && have systemctl; then
        echo "==> Restarting the running striatumd (--restart-daemon)"
        systemctl --user restart striatumd
        echo "==> striatumd restarted on the new binary"
    else
        echo ""
        echo "*** A striatumd daemon is RUNNING and still executes the OLD binary. ***"
        echo "*** The upgrade takes effect only after:                             ***"
        echo "***     systemctl --user restart striatumd                           ***"
        echo "*** (Check open sessions first: striatum list sessions)              ***"
    fi
fi

echo "==> Done. $("${PREFIX}/striatum" --version 2>/dev/null || echo "striatum ${VERSION}")"
