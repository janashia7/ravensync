#!/bin/sh
# Install script for ravensync — https://github.com/janashia7/ravensync
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/janashia7/ravensync/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/janashia7/ravensync/main/install.sh | sh -s -- --yes
#
# Environment variables:
#   RAVENSYNC_INSTALL_DIR  Override binary installation directory

set -eu

REPO="janashia7/ravensync"
BINARY_NAME="ravensync"
AUTO_YES=0
TMPDIR_PATH=""

# --- Utility functions ---

setup_colors() {
    if [ -t 2 ]; then
        RED='\033[0;31m'
        GREEN='\033[0;32m'
        YELLOW='\033[1;33m'
        BOLD='\033[1m'
        RESET='\033[0m'
    else
        RED=''
        GREEN=''
        YELLOW=''
        BOLD=''
        RESET=''
    fi
}

info() {
    printf "${GREEN}info${RESET}: %s\n" "$*" >&2
}

warn() {
    printf "${YELLOW}warn${RESET}: %s\n" "$*" >&2
}

err() {
    printf "${RED}error${RESET}: %s\n" "$*" >&2
    exit 1
}

confirm() {
    if [ "$AUTO_YES" = "1" ]; then
        return 0
    fi
    if ! [ -e /dev/tty ]; then
        return 1
    fi
    printf "%s [y/N] " "$1" >/dev/tty
    read -r response </dev/tty
    case "$response" in
        [yY]) return 0 ;;
        *) return 1 ;;
    esac
}

need_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        err "need '$1' (command not found)"
    fi
}

check_cmd() {
    command -v "$1" >/dev/null 2>&1
}

in_path() {
    case ":${PATH}:" in
        *":$1:"*) return 0 ;;
        *)        return 1 ;;
    esac
}

# --- Detection functions ---

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --yes | -y)
                AUTO_YES=1
                ;;
            --help | -h)
                cat <<'EOF'
Install ravensync — privacy-first, cross-platform AI memory.

Usage:
  install.sh [OPTIONS]

Options:
  -y, --yes     Skip all confirmation prompts
  -h, --help    Show this help message

Environment variables:
  RAVENSYNC_INSTALL_DIR   Override binary installation directory
EOF
                exit 0
                ;;
            *)
                warn "unknown option: $1"
                ;;
        esac
        shift
    done
}

detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Darwin)  OS="darwin" ;;
        Linux)   OS="linux" ;;
        MINGW* | MSYS* | CYGWIN*) OS="windows" ;;
        *)       err "unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        arm64 | aarch64) ARCH="arm64" ;;
        x86_64 | amd64)  ARCH="amd64" ;;
        *)               err "unsupported architecture: $ARCH" ;;
    esac
}

detect_existing() {
    EXISTING_PATH=""
    if check_cmd "$BINARY_NAME"; then
        EXISTING_PATH="$(command -v "$BINARY_NAME")"
    fi
}

get_latest_version() {
    REDIRECT_URL="$(curl -fsSL -o /dev/null -w '%{redirect_url}' \
        "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"

    if [ -n "$REDIRECT_URL" ]; then
        VERSION="$(printf '%s' "$REDIRECT_URL" | sed 's|.*/||')"
    fi

    if [ -z "${VERSION:-}" ]; then
        VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
            | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')"
    fi

    if [ -z "${VERSION:-}" ]; then
        err "could not determine latest version from GitHub"
    fi

    info "latest version: ${VERSION}"
}

determine_install_dir() {
    if [ -n "${RAVENSYNC_INSTALL_DIR:-}" ]; then
        INSTALL_DIR="$RAVENSYNC_INSTALL_DIR"
        return
    fi
    case "$OS" in
        windows)
            INSTALL_DIR="${LOCALAPPDATA:-$HOME/AppData/Local}/ravensync/bin"
            ;;
        *)
            if [ -w "/usr/local/bin" ]; then
                INSTALL_DIR="/usr/local/bin"
            else
                INSTALL_DIR="${HOME}/.local/bin"
            fi
            ;;
    esac
}

has_prebuilt() {
    case "${OS}_${ARCH}" in
        darwin_arm64 | darwin_amd64 | linux_amd64 | linux_arm64 | windows_amd64 | windows_arm64) return 0 ;;
        *) return 1 ;;
    esac
}

# --- Installation functions ---

install_binary() {
    TMPDIR_PATH="$(mktemp -d)"

    _ext=""
    if [ "$OS" = "windows" ]; then
        _ext=".exe"
    fi

    _archive="${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
    if [ "$OS" = "windows" ]; then
        _archive="${BINARY_NAME}_${OS}_${ARCH}.zip"
    fi

    _url="https://github.com/${REPO}/releases/download/${VERSION}/${_archive}"

    info "downloading ${_archive}..."
    if ! curl -fL# "$_url" -o "${TMPDIR_PATH}/${_archive}"; then
        err "download failed: ${_url}"
    fi

    info "extracting..."
    if [ "$OS" = "windows" ]; then
        unzip -q "${TMPDIR_PATH}/${_archive}" -d "${TMPDIR_PATH}"
    else
        tar -xzf "${TMPDIR_PATH}/${_archive}" -C "${TMPDIR_PATH}"
    fi

    if [ ! -f "${TMPDIR_PATH}/${BINARY_NAME}${_ext}" ]; then
        err "expected binary '${BINARY_NAME}${_ext}' not found in archive"
    fi

    mkdir -p "$INSTALL_DIR"

    if [ -w "$INSTALL_DIR" ]; then
        cp "${TMPDIR_PATH}/${BINARY_NAME}${_ext}" "${INSTALL_DIR}/${BINARY_NAME}${_ext}"
    else
        info "elevated permissions required for ${INSTALL_DIR}"
        sudo cp "${TMPDIR_PATH}/${BINARY_NAME}${_ext}" "${INSTALL_DIR}/${BINARY_NAME}${_ext}"
    fi
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}${_ext}" 2>/dev/null || true

    info "installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}${_ext}"
    rm -rf "$TMPDIR_PATH"
    TMPDIR_PATH=""
}

install_from_source() {
    if ! check_cmd go; then
        err "go is not installed and no prebuilt binary available. Install Go: https://go.dev/dl"
    fi
    info "building from source..."
    go install "github.com/${REPO}/cmd/ravensync@latest"
    info "installed via 'go install'"
}

setup_shell_profile() {
    if in_path "$INSTALL_DIR"; then
        return
    fi

    _source_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
    _profile=""

    case "${SHELL:-}" in
        */zsh)
            _profile="$HOME/.zshrc"
            ;;
        */bash)
            if [ -f "$HOME/.bashrc" ]; then
                _profile="$HOME/.bashrc"
            elif [ -f "$HOME/.bash_profile" ]; then
                _profile="$HOME/.bash_profile"
            fi
            ;;
        */fish)
            _profile="$HOME/.config/fish/config.fish"
            _source_line="set -gx PATH ${INSTALL_DIR} \$PATH"
            ;;
    esac

    if [ -z "$_profile" ]; then
        warn "could not detect shell profile"
        info "add this to your shell profile manually:"
        printf "  %s\n" "$_source_line" >&2
        return
    fi

    if [ -f "$_profile" ] && grep -qF "$INSTALL_DIR" "$_profile" 2>/dev/null; then
        return
    fi

    echo ""
    if confirm "add ${BINARY_NAME} to PATH in ${_profile}?"; then
        echo "$_source_line" >> "$_profile"
        info "added to ${_profile} — restart your shell or run:"
        printf "  %s\n" "$_source_line" >&2
    else
        info "add it manually:"
        printf "  %s\n" "$_source_line" >&2
    fi
}

post_install() {
    setup_shell_profile

    echo ""
    printf "${BOLD}${GREEN}ravensync${RESET} installed successfully!\n\n"
    printf "  Get started:\n"
    printf "    ${BOLD}ravensync init${RESET}     Set up config & encryption\n"
    printf "    ${BOLD}ravensync serve${RESET}    Start the agent\n"
    printf "    ${BOLD}ravensync doctor${RESET}   Check your setup\n"
    echo ""
}

cleanup() {
    if [ -n "${TMPDIR_PATH:-}" ] && [ -d "${TMPDIR_PATH}" ]; then
        rm -rf "$TMPDIR_PATH"
    fi
}

# --- Main ---

main() {
    parse_args "$@"
    setup_colors
    need_cmd curl
    need_cmd uname
    detect_platform
    detect_existing
    determine_install_dir
    trap cleanup EXIT

    echo ""
    printf "${BOLD}Ravensync Installer${RESET}\n"
    echo ""

    if [ -n "$EXISTING_PATH" ]; then
        warn "${BINARY_NAME} is already installed at ${EXISTING_PATH}"
        if ! confirm "do you want to override it?"; then
            info "installation cancelled."
            exit 0
        fi
    fi

    info "platform: ${OS}/${ARCH}"
    info "install dir: ${INSTALL_DIR}"

    if has_prebuilt; then
        get_latest_version
        install_binary
    else
        warn "no prebuilt binary for ${OS}/${ARCH}, falling back to source build"
        install_from_source
    fi

    post_install
}

main "$@"
