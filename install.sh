#!/bin/sh
# Install Walden from GitHub release binaries — no Go toolchain required.
#
#   curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
#
# Flags pass through the pipe: `... | sh -s -- --skill claude`
set -e

# --- Constants ---

REPO="andrearaponi/walden"
REPO_URL="https://github.com/${REPO}"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="walden"
WALDEN="${INSTALL_DIR}/${BINARY_NAME}"

# --- Flags ---

VERSION=""
SKILL_TARGET=""
NO_VERIFY=0
UNINSTALL=0

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

# --- Usage ---

usage() {
  printf "${BOLD}install.sh${NC} — install Walden from GitHub releases\n\n"
  printf "Usage:\n"
  printf "  curl -fsSL https://raw.githubusercontent.com/%s/main/install.sh | sh\n" "$REPO"
  printf "  curl -fsSL https://raw.githubusercontent.com/%s/main/install.sh | sh -s -- --skill claude\n\n" "$REPO"
  printf "Flags:\n"
  printf "  --version <tag>   Install a specific release (default: latest)\n"
  printf "  --skill <agent>   Install the AI skill non-interactively: claude|codex|copilot|opencode|all\n"
  printf "  --no-verify       Skip checksum verification (needed for releases <= v0.4.0)\n"
  printf "  --uninstall       Remove the skill (all agents) and the binary\n"
  printf "  --help            Show this help\n"
}

# --- Flag parsing ---

parse_flags() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --version)
        if [ $# -lt 2 ]; then err "--version requires a tag (e.g. --version v0.5.0)"; exit 1; fi
        VERSION="$2"
        shift 2 ;;
      --skill)
        if [ $# -lt 2 ]; then err "--skill requires an agent: claude, codex, copilot, opencode, all"; exit 1; fi
        SKILL_TARGET="$2"
        shift 2 ;;
      --no-verify)
        NO_VERIFY=1
        shift ;;
      --uninstall)
        UNINSTALL=1
        shift ;;
      --help|-h)
        usage
        exit 0 ;;
      *)
        err "Unknown flag: $1"
        usage >&2
        exit 1 ;;
    esac
  done
}

validate_skill_target() {
  case "$SKILL_TARGET" in
    ""|all|claude|codex|copilot|opencode) ;;
    *) err "Unknown --skill target: ${SKILL_TARGET} (supported: claude, codex, copilot, opencode, all)"; exit 1 ;;
  esac
}

# --- Platform detection ---

detect_platform() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$OS" in
    darwin|linux) ;;
    *) err "Unsupported OS: $OS (supported: darwin, linux)"; exit 1 ;;
  esac

  raw_arch="$(uname -m)"
  case "$raw_arch" in
    x86_64)        ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) err "Unsupported architecture: $raw_arch (supported: amd64, arm64)"; exit 1 ;;
  esac

  info "Platform: ${OS}/${ARCH}"
}

# --- Downloader ---

require_downloader() {
  if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
  elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
  else
    err "curl or wget is required but neither was found"
    exit 1
  fi
}

# --- Network seams ---

# fetch <url> <dest> — download url to dest, failing on HTTP errors.
fetch() {
  if [ "$DOWNLOADER" = "curl" ]; then
    curl -fsSL -o "$2" "$1"
  else
    wget -q -O "$2" "$1"
  fi
}

# fetch_final_url <url> — print the URL reached after following redirects.
fetch_final_url() {
  if [ "$DOWNLOADER" = "curl" ]; then
    curl -fsSLI -o /dev/null -w '%{url_effective}' "$1"
  else
    # wget exits non-zero when stopped at a redirect; the pipeline masks it
    # and we only keep the Location header.
    wget --max-redirect=0 -qS -O /dev/null "$1" 2>&1 | sed -n 's/^ *[Ll]ocation: *//p' | tr -d '\r' | head -1
  fi
}

# --- Release resolution ---

resolve_tag() {
  if [ -n "$VERSION" ]; then
    TAG="$VERSION"
    info "Version: ${TAG} (requested)"
    return 0
  fi

  latest_url="$(fetch_final_url "${REPO_URL}/releases/latest" || true)"
  TAG="${latest_url##*/}"
  case "$TAG" in
    v[0-9]*) ;;
    *)
      err "Could not resolve the latest release tag from ${REPO_URL}/releases/latest (got: ${latest_url:-nothing})"
      err "Pin a release explicitly with --version <tag>"
      exit 1 ;;
  esac
  info "Version: ${TAG} (latest)"
}

# --- Download ---

WORKSPACE=""

cleanup() {
  if [ -n "$WORKSPACE" ]; then
    rm -rf "$WORKSPACE"
  fi
}

download_release() {
  WORKSPACE="$(mktemp -d)"
  trap cleanup EXIT

  ASSET="walden-${TAG}-${OS}-${ARCH}"
  asset_url="${REPO_URL}/releases/download/${TAG}/${ASSET}"

  info "Downloading ${ASSET}..."
  if ! fetch "$asset_url" "${WORKSPACE}/${ASSET}"; then
    err "Download failed: ${asset_url}"
    err "Check that release ${TAG} exists and ships ${OS}/${ARCH} binaries"
    exit 1
  fi

  CHECKSUMS="${WORKSPACE}/checksums.txt"
  if ! fetch "${REPO_URL}/releases/download/${TAG}/checksums.txt" "$CHECKSUMS" 2>/dev/null; then
    CHECKSUMS=""
  fi
}

# --- Checksum verification ---

compute_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    err "sha256sum or shasum is required for checksum verification (or pass --no-verify)"
    exit 1
  fi
}

verify_checksum() {
  if [ "$NO_VERIFY" = "1" ]; then
    warn "Checksum verification skipped (--no-verify)"
    return 0
  fi

  if [ -z "$CHECKSUMS" ]; then
    err "Release ${TAG} does not provide checksums.txt (releases up to v0.4.0 predate checksums)"
    err "Rerun with --no-verify to install anyway"
    exit 1
  fi

  expected="$(awk -v asset="$ASSET" '$2 == asset {print $1}' "$CHECKSUMS")"
  if [ -z "$expected" ]; then
    err "No checksums.txt entry for ${ASSET} in release ${TAG}"
    exit 1
  fi

  actual="$(compute_sha256 "${WORKSPACE}/${ASSET}")"
  if [ "$actual" != "$expected" ]; then
    err "Checksum mismatch for ${ASSET}"
    err "  expected: ${expected}"
    err "  actual:   ${actual}"
    exit 1
  fi

  ok "Checksum verified"
}

# --- Binary install ---

install_binary() {
  mkdir -p "$INSTALL_DIR"

  # Stage inside the target directory so the final rename is atomic: no
  # failure path can leave a partial ~/.local/bin/walden behind.
  tmp_target="${INSTALL_DIR}/.${BINARY_NAME}.tmp.$$"
  cp "${WORKSPACE}/${ASSET}" "$tmp_target"
  chmod +x "$tmp_target"
  mv "$tmp_target" "$WALDEN"

  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) warn "${INSTALL_DIR} is not in your PATH. Add it with:"
       warn "  export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
  esac

  ok "Binary installed to ${WALDEN}"
}

verify_binary() {
  if "$WALDEN" version >/dev/null 2>&1; then
    result="$("$WALDEN" version 2>&1 | head -1)"
    ok "Verified: ${result}"
  else
    err "Installed binary failed to run '${BINARY_NAME} version'"
    exit 1
  fi
}

# --- Skill handoff (delegated to the binary) ---

prompt_skill_install() {
  # Piped installs (curl | sh) have a pipe on stdin; the terminal, when
  # there is one, is still reachable through /dev/tty.
  if ! ( : < /dev/tty ) 2>/dev/null; then
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

install_skill() {
  if [ -n "$SKILL_TARGET" ]; then
    if [ "$SKILL_TARGET" = "all" ]; then
      "$WALDEN" skill install --all
    else
      "$WALDEN" skill install "$SKILL_TARGET"
    fi
    return 0
  fi
  prompt_skill_install
}

# --- Uninstall ---

uninstall() {
  if [ -x "$WALDEN" ]; then
    "$WALDEN" skill uninstall --all
  else
    warn "walden binary not found at ${WALDEN}; skill files may remain"
    warn "Reinstall and run '${BINARY_NAME} skill uninstall --all' to remove them"
  fi

  if [ -f "$WALDEN" ]; then
    rm -f "$WALDEN"
    ok "Removed ${WALDEN}"
  else
    info "Binary not found at ${WALDEN} (skipping)"
  fi
}

# --- Main ---

main() {
  parse_flags "$@"
  validate_skill_target

  if [ "$UNINSTALL" = "1" ]; then
    printf "\n${BOLD}=== Walden Uninstall ===${NC}\n\n"
    uninstall
    printf "\n${BOLD}=== Done ===${NC}\n"
    return 0
  fi

  printf "\n${BOLD}=== Walden Install ===${NC}\n\n"
  detect_platform
  require_downloader
  resolve_tag
  download_release
  verify_checksum
  install_binary
  verify_binary
  install_skill
  printf "\n${BOLD}=== Done ===${NC}\n"
}

main "$@"
