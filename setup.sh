#!/bin/sh
set -e

# --- Constants ---

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="walden"
WALDEN="${INSTALL_DIR}/${BINARY_NAME}"

# --- Colors (degrade gracefully) ---

if [ -t 1 ]; then
  BLUE='\033[0;34m'
  GREEN='\033[0;32m'
  YELLOW='\033[0;33m'
  RED='\033[0;31m'
  BOLD='\033[1m'
  NC='\033[0m'
else
  BLUE='' GREEN='' YELLOW='' RED='' BOLD='' NC=''
fi

# --- UX helpers ---

info()  { printf "${BLUE}[info]${NC}  %s\n" "$*"; }
ok()    { printf "${GREEN}[ok]${NC}    %s\n" "$*"; }
warn()  { printf "${YELLOW}[warn]${NC}  %s\n" "$*"; }
err()   { printf "${RED}[error]${NC} %s\n" "$*" >&2; }

# --- Platform detection ---

detect_platform() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$OS" in
    darwin|linux) ;;
    *) err "Unsupported OS: $OS"; exit 1 ;;
  esac

  raw_arch="$(uname -m)"
  case "$raw_arch" in
    x86_64)        ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) err "Unsupported architecture: $raw_arch"; exit 1 ;;
  esac

  info "Platform: ${OS}/${ARCH}"
}

# --- Version detection ---

detect_version() {
  # Full describe: clean tag on release commits (v0.3.0), honest suffix on
  # dev builds (v0.3.0-9-g337b6b5) so installs from a branch are identifiable.
  VERSION="$(git -C "$SCRIPT_DIR" describe --tags --dirty 2>/dev/null || echo "dev")"
  info "Version: ${VERSION}"
}

# --- Prerequisites ---

check_prerequisites() {
  if ! command -v go >/dev/null 2>&1; then
    err "Go is required but not found. Install Go 1.25.0+ from https://go.dev/dl/"
    exit 1
  fi

  if [ ! -f "$SCRIPT_DIR/go.mod" ]; then
    err "go.mod not found at $SCRIPT_DIR"
    err "Run this script from the walden repository root."
    exit 1
  fi

  ok "Prerequisites satisfied"
}

# --- Build ---

build_binary() {
  info "Building walden..."
  (cd "$SCRIPT_DIR" && GOOS="$OS" GOARCH="$ARCH" go build \
    -ldflags "-X github.com/andrearaponi/walden/internal/app.Version=${VERSION}" \
    -o "${SCRIPT_DIR}/${BINARY_NAME}" ./cmd/walden)
  ok "Binary built"
}

# --- Binary install ---

install_binary() {
  mkdir -p "$INSTALL_DIR"
  cp "${SCRIPT_DIR}/${BINARY_NAME}" "$WALDEN"
  chmod +x "$WALDEN"
  rm -f "${SCRIPT_DIR}/${BINARY_NAME}"

  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) warn "${INSTALL_DIR} is not in your PATH. Add it with:"
       warn "  export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
  esac

  ok "Binary installed to $WALDEN"
}

# --- Binary verify ---

verify_binary() {
  if "$WALDEN" version >/dev/null 2>&1; then
    result="$("$WALDEN" version 2>&1 | head -1)"
    ok "Verified: ${result}"
  else
    warn "Binary installed but 'walden version' did not succeed"
    warn "Check that ${INSTALL_DIR} is in your PATH"
  fi
}

# --- Skill install (delegated to the binary) ---

prompt_skill_install() {
  if ! [ -t 0 ]; then
    info "Non-interactive mode: skipping skill install"
    info "Run '${WALDEN} skill install <agent>' to install the skill"
    return 0
  fi

  printf "\n${BOLD}Install Walden skill for:${NC}\n"
  printf "  1) Claude Code\n"
  printf "  2) Codex\n"
  printf "  3) Copilot\n"
  printf "  4) OpenCode\n"
  printf "  5) All\n"
  printf "  6) Skip\n"
  printf "\n${BOLD}Choice [1-6]:${NC} "

  read -r choice < /dev/tty

  case "$choice" in
    1) "$WALDEN" skill install claude ;;
    2) "$WALDEN" skill install codex ;;
    3) "$WALDEN" skill install copilot ;;
    4) "$WALDEN" skill install opencode ;;
    5) "$WALDEN" skill install --all ;;
    6) info "Skill install skipped" ;;
    *) warn "Invalid choice: ${choice}. Skipping skill install." ;;
  esac
}

verify_skill() {
  "$WALDEN" skill status
}

# --- Uninstall ---

uninstall_skill() {
  if [ -x "$WALDEN" ]; then
    "$WALDEN" skill uninstall --all
  else
    warn "walden binary not found at ${WALDEN}; skill files may remain"
    warn "Reinstall and run '${BINARY_NAME} skill uninstall --all' to remove them"
  fi
}

uninstall_binary() {
  if [ -f "$WALDEN" ]; then
    rm -f "$WALDEN"
    ok "Removed $WALDEN"
  else
    info "Binary not found at $WALDEN (skipping)"
  fi
}

# --- Usage ---

usage() {
  printf "${BOLD}setup.sh${NC} — install or uninstall Walden\n\n"
  printf "Usage:\n"
  printf "  ./setup.sh              Install binary and skill\n"
  printf "  ./setup.sh install      Install binary and skill\n"
  printf "  ./setup.sh uninstall    Remove binary and skill\n"
  printf "  ./setup.sh --help       Show this help\n"
}

# --- Main ---

main() {
  case "${1:-install}" in
    install)
      printf "\n${BOLD}=== Walden Install ===${NC}\n\n"
      check_prerequisites
      detect_platform
      detect_version
      build_binary
      install_binary
      verify_binary
      prompt_skill_install
      verify_skill
      printf "\n${BOLD}=== Done ===${NC}\n"
      ;;
    uninstall)
      printf "\n${BOLD}=== Walden Uninstall ===${NC}\n\n"
      uninstall_skill
      uninstall_binary
      printf "\n${BOLD}=== Done ===${NC}\n"
      ;;
    --help|-h)
      usage
      ;;
    *)
      err "Unknown command: $1"
      usage
      exit 1
      ;;
  esac
}

main "$@"
