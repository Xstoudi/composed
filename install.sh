#!/usr/bin/env sh

set -e

REPO="Xstoudi/composed"
BINARY="composed"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── helpers ────────────────────────────────────────────────────────────────────

say()  { printf '%s\n' "$*"; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not found in PATH"
}

# ── detect OS / arch ───────────────────────────────────────────────────────────

detect_os() {
  case "$(uname -s)" in
    Linux)  echo "linux"  ;;
    Darwin) echo "darwin" ;;
    *)      die "Unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)   echo "amd64" ;;
    aarch64|arm64)  echo "arm64" ;;
    *)              die "Unsupported architecture: $(uname -m)" ;;
  esac
}

# ── resolve latest version ─────────────────────────────────────────────────────

latest_version() {
  need curl
  curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\(.*\)".*/\1/'
}

# ── main ───────────────────────────────────────────────────────────────────────

main() {
  need curl
  need tar

  OS="$(detect_os)"
  ARCH="$(detect_arch)"

  VERSION="${VERSION:-$(latest_version)}"
  [ -n "$VERSION" ] || die "Could not determine latest version"

  ARCHIVE="${BINARY}_${OS}_${ARCH}.tar.gz"
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

  say "Installing ${BINARY} ${VERSION} (${OS}/${ARCH}) → ${INSTALL_DIR}/${BINARY}"

  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT

  curl -sSfL "$URL" -o "${TMP}/${ARCHIVE}"
  tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

  # install — use sudo only when the target directory is not writable
  if [ -w "$INSTALL_DIR" ]; then
    install -m 0755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  else
    sudo install -m 0755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  fi

  say "Done! Run '${BINARY} --help' to get started."
}

main "$@"

