#!/bin/sh
set -e

# --- Constants ---

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="walden"
SKILL_SOURCE="$SCRIPT_DIR/skill/walden/SKILL.md"
CLAUDE_TARGET="$HOME/.claude/commands/walden.md"
CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"
CODEX_TARGET="$CODEX_HOME/AGENTS.md"
CODEX_BEGIN="# --- BEGIN WALDEN SKILL ---"
CODEX_END="# --- END WALDEN SKILL ---"

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
  VERSION="$(git -C "$SCRIPT_DIR" describe --tags --abbrev=0 2>/dev/null || echo "dev")"
  info "Version: ${VERSION}"
}

# --- Prerequisites ---

check_prerequisites() {
  if ! command -v go >/dev/null 2>&1; then
    err "Go is required but not found. Install Go 1.25.0+ from https://go.dev/dl/"
    exit 1
  fi

  if [ ! -f "$SKILL_SOURCE" ]; then
    err "SKILL.md not found at $SKILL_SOURCE"
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
  cp "${SCRIPT_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
  rm -f "${SCRIPT_DIR}/${BINARY_NAME}"

  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) warn "${INSTALL_DIR} is not in your PATH. Add it with:"
       warn "  export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
  esac

  ok "Binary installed to ${INSTALL_DIR}/${BINARY_NAME}"
}

# --- Binary verify ---

verify_binary() {
  if "${INSTALL_DIR}/${BINARY_NAME}" version >/dev/null 2>&1; then
    result="$("${INSTALL_DIR}/${BINARY_NAME}" version 2>&1 | head -1)"
    ok "Verified: ${result}"
  else
    warn "Binary installed but 'walden version' did not succeed"
    warn "Check that ${INSTALL_DIR} is in your PATH"
  fi
}

# --- Skill install: Claude ---

install_skill_claude() {
  mkdir -p "$(dirname "$CLAUDE_TARGET")"
  cp "$SKILL_SOURCE" "$CLAUDE_TARGET"
  ok "Skill installed for Claude Code at ${CLAUDE_TARGET}"
}

# --- Skill install: Codex ---

install_skill_codex() {
  if grep -q "$CODEX_BEGIN" "$CODEX_TARGET" 2>/dev/null; then
    ok "Skill already installed for Codex (skipping)"
    return 0
  fi

  mkdir -p "$(dirname "$CODEX_TARGET")"
  {
    printf '\n%s\n' "$CODEX_BEGIN"
    cat "$SKILL_SOURCE"
    printf '\n%s\n' "$CODEX_END"
  } >> "$CODEX_TARGET"
  ok "Skill installed for Codex at ${CODEX_TARGET}"
}

# --- Skill verify ---

verify_skill() {
  verified=0
  if [ -f "$CLAUDE_TARGET" ]; then
    ok "Claude skill present at ${CLAUDE_TARGET}"
    verified=1
  fi
  if grep -q "$CODEX_BEGIN" "$CODEX_TARGET" 2>/dev/null; then
    ok "Codex skill present at ${CODEX_TARGET}"
    verified=1
  fi
  if [ "$verified" -eq 0 ]; then
    info "No skill files installed (skipped)"
  fi
}

# --- Skill prompt ---

prompt_skill_install() {
  if ! [ -t 0 ]; then
    info "Non-interactive mode: skipping skill install"
    info "Run './setup.sh install' interactively to install the skill"
    return 0
  fi

  printf "\n${BOLD}Install Walden skill for:${NC}\n"
  printf "  1) Claude Code\n"
  printf "  2) Codex\n"
  printf "  3) Both\n"
  printf "  4) Skip\n"
  printf "\n${BOLD}Choice [1-4]:${NC} "

  read -r choice < /dev/tty

  case "$choice" in
    1) install_skill_claude ;;
    2) install_skill_codex ;;
    3) install_skill_claude; install_skill_codex ;;
    4) info "Skill install skipped" ;;
    *) warn "Invalid choice: ${choice}. Skipping skill install." ;;
  esac
}

# --- Uninstall ---

uninstall_binary() {
  if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    ok "Removed ${INSTALL_DIR}/${BINARY_NAME}"
  else
    info "Binary not found at ${INSTALL_DIR}/${BINARY_NAME} (skipping)"
  fi
}

uninstall_skill_claude() {
  if [ -f "$CLAUDE_TARGET" ]; then
    rm -f "$CLAUDE_TARGET"
    ok "Removed ${CLAUDE_TARGET}"
  else
    info "Claude skill not found (skipping)"
  fi
}

uninstall_skill_codex() {
  if [ ! -f "$CODEX_TARGET" ]; then
    info "Codex AGENTS.md not found (skipping)"
    return 0
  fi

  if ! grep -q "$CODEX_BEGIN" "$CODEX_TARGET" 2>/dev/null; then
    info "No Walden block in Codex AGENTS.md (skipping)"
    return 0
  fi

  tmp="$(mktemp)"
  awk -v begin="$CODEX_BEGIN" -v end="$CODEX_END" '
    $0 == begin { skip=1; next }
    $0 == end   { skip=0; next }
    !skip       { print }
  ' "$CODEX_TARGET" > "$tmp"

  if [ -s "$tmp" ]; then
    mv "$tmp" "$CODEX_TARGET"
  else
    rm -f "$tmp" "$CODEX_TARGET"
  fi

  ok "Removed Walden block from ${CODEX_TARGET}"
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
      uninstall_binary
      uninstall_skill_claude
      uninstall_skill_codex
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
