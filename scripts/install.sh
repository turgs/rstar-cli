#!/usr/bin/env bash
# install.sh — install the rstar CLI from GitHub Releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/turgs/rstar-cli/main/scripts/install.sh | bash
#
# Environment:
#   RSTAR_BIN_DIR   Install directory (default: ~/bin if on PATH, else ~/.local/bin)
#   RSTAR_VERSION   Version without leading v (default: latest stable release)
#   RSTAR_REPO      GitHub repo (default: turgs/rstar-cli)
#
# Verifies SHA-256 of the downloaded archive against checksums.txt on the same release.
# Does not run setup or touch existing auth config.

set -euo pipefail

REPO="${RSTAR_REPO:-turgs/rstar-cli}"
BIN_DIR="${RSTAR_BIN_DIR:-}"
VERSION="${RSTAR_VERSION:-}"
BIN_NAME="rstar"

info() { printf '  ✓ %s\n' "$1"; }
step() { printf '  → %s\n' "$1"; }
err()  { printf '  ✗ ERROR: %s\n' "$1" >&2; exit 1; }

path_contains_dir() {
  local dir="$1"
  [[ ":${PATH}:" == *":${dir}:"* ]]
}

default_bin_dir() {
  if path_contains_dir "${HOME}/bin"; then
    echo "${HOME}/bin"
    return 0
  fi
  if path_contains_dir "${HOME}/.local/bin"; then
    echo "${HOME}/.local/bin"
    return 0
  fi
  echo "${HOME}/.local/bin"
}

detect_platform() {
  local os arch
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    darwin) os="darwin" ;;
    linux) os="linux" ;;
    mingw*|msys*|cygwin*) os="windows" ;;
    *) err "Unsupported OS: $os" ;;
  esac

  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) err "Unsupported architecture: $arch" ;;
  esac

  echo "${os}_${arch}"
}

find_sha256_cmd() {
  if command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum"
  elif command -v shasum >/dev/null 2>&1; then
    echo "shasum -a 256"
  else
    err "No SHA-256 tool found (need sha256sum or shasum)"
  fi
}

get_latest_version() {
  local url version
  # Follow releases/latest redirect — avoids GitHub API rate limits.
  url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest") || \
    err "Could not resolve latest release for ${REPO}"
  version="${url##*/}"
  version="${version#v}"
  if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
    err "Could not parse version from ${url}"
  fi
  echo "$version"
}

verify_checksum() {
  local file="$1" expected="$2"
  local cmd actual
  cmd=$(find_sha256_cmd)
  actual=$($cmd "$file" | awk '{print $1}')
  # bash 3.2 (macOS) has no ${var,,}; compare via tr
  actual=$(printf '%s' "$actual" | tr '[:upper:]' '[:lower:]')
  expected=$(printf '%s' "$expected" | tr '[:upper:]' '[:lower:]')
  if [[ "$actual" != "$expected" ]]; then
    err "Checksum mismatch for $(basename "$file"): expected ${expected}, got ${actual}"
  fi
}

main() {
  local platform os arch ext archive_name tag tmp dir expected checksums_url archive_url
  local line hash name bin_src dest

  command -v curl >/dev/null 2>&1 || err "curl is required"
  command -v tar >/dev/null 2>&1 || err "tar is required"

  platform=$(detect_platform)
  os="${platform%_*}"
  arch="${platform#*_}"

  if [[ -z "$VERSION" ]]; then
    step "Resolving latest release…"
    VERSION=$(get_latest_version)
  fi
  VERSION="${VERSION#v}"
  tag="v${VERSION}"
  info "Version ${VERSION} (${platform})"

  if [[ -z "$BIN_DIR" ]]; then
    BIN_DIR=$(default_bin_dir)
  fi
  mkdir -p "$BIN_DIR"

  if [[ "$os" == "windows" ]]; then
    ext="zip"
    archive_name="${BIN_NAME}_${VERSION}_${os}_${arch}.zip"
  else
    ext="tar.gz"
    archive_name="${BIN_NAME}_${VERSION}_${os}_${arch}.tar.gz"
  fi

  archive_url="https://github.com/${REPO}/releases/download/${tag}/${archive_name}"
  checksums_url="https://github.com/${REPO}/releases/download/${tag}/checksums.txt"

  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT

  step "Downloading ${archive_name}…"
  curl -fsSL -o "${tmp}/${archive_name}" "$archive_url" || err "Download failed: ${archive_url}"
  curl -fsSL -o "${tmp}/checksums.txt" "$checksums_url" || err "Download failed: ${checksums_url}"

  expected=""
  while IFS= read -r line || [[ -n "$line" ]]; do
    # goreleaser: "<hash>  <filename>" (two spaces)
    hash="${line%% *}"
    name="${line##* }"
    name="${name##*/}"
    if [[ "$name" == "$archive_name" ]]; then
      expected="$hash"
      break
    fi
  done < "${tmp}/checksums.txt"
  [[ -n "$expected" ]] || err "checksums.txt has no entry for ${archive_name}"

  step "Verifying SHA-256…"
  verify_checksum "${tmp}/${archive_name}" "$expected"
  info "Checksum ok"

  dir="${tmp}/extract"
  mkdir -p "$dir"
  if [[ "$ext" == "zip" ]]; then
    command -v unzip >/dev/null 2>&1 || err "unzip is required for Windows archives"
    # -j junk paths: only basenames, avoids zip-slip into parent dirs
    unzip -j -q "${tmp}/${archive_name}" -d "$dir"
  else
    tar -xzf "${tmp}/${archive_name}" -C "$dir"
  fi

  bin_src=$(find "$dir" -type f \( -name "$BIN_NAME" -o -name "${BIN_NAME}.exe" \) | head -n 1)
  [[ -n "$bin_src" ]] || err "Archive did not contain ${BIN_NAME}"
  # Refuse path escape if an archive member resolved outside the extract dir
  case "$bin_src" in
    "$dir"/*) ;;
    *) err "Refusing archive member outside extract dir: $bin_src" ;;
  esac

  dest="${BIN_DIR}/${BIN_NAME}"
  if [[ "$os" == "windows" ]]; then
    dest="${BIN_DIR}/${BIN_NAME}.exe"
  fi

  step "Installing to ${dest}…"
  install -m 0755 "$bin_src" "$dest"
  info "Installed rstar ${VERSION}"

  if ! path_contains_dir "$BIN_DIR"; then
    printf '\n  Add %s to your PATH, for example:\n' "$BIN_DIR"
    printf '    export PATH="%s:$PATH"\n\n' "$BIN_DIR"
  fi

  printf '  Next:\n'
  printf '    rstar version\n'
  printf '    rstar auth login --base-url https://YOUR.receptionstar.com TOKEN\n'
  printf '    rstar doctor\n'
  printf '  Later upgrades: rstar upgrade\n'
}

main "$@"
