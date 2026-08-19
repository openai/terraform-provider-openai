#!/usr/bin/env bash

set -euo pipefail

runner_temp="${RUNNER_TEMP:?RUNNER_TEMP must be set}"
github_env="${GITHUB_ENV:?GITHUB_ENV must be set}"
tools_dir="$(mktemp -d "${runner_temp}/verified-release-tools.XXXXXXXX")"

case "$(uname -s)/$(uname -m)" in
  Linux/x86_64)
    goreleaser_archive="goreleaser_Linux_x86_64.tar.gz"
    goreleaser_sha256="eaae05b5eba07533bd0f06846b68c808399504784df00c62eb219541fc04e5e2"
    syft_archive="syft_1.51.0_linux_amd64.tar.gz"
    syft_sha256="2a2e837a2c8d59ec9af5472ee22d3b04ee463c4e44476ecf993fd1e5ab6ebc7f"
    ;;
  Darwin/arm64)
    goreleaser_archive="goreleaser_Darwin_arm64.tar.gz"
    goreleaser_sha256="8f6898256f35531165d90f2db581c5ee0d32bda83ebc25ac231ff5bdb9d2071a"
    syft_archive="syft_1.51.0_darwin_arm64.tar.gz"
    syft_sha256="4f37f4c7fefce0a68e4cf71ba3f5f9829a99e65d89b29f7ee41b8c2c10ea8c59"
    ;;
  *)
    echo "Unsupported release-tool platform: $(uname -s)/$(uname -m)" >&2
    exit 1
    ;;
esac

install_verified_archive() {
  local repository="$1"
  local version="$2"
  local archive="$3"
  local expected_sha256="$4"
  local executable="$5"
  local archive_path="${tools_dir}/${archive}"
  local actual_sha256

  curl --proto '=https' --proto-redir '=https' --fail --silent --show-error --location \
    --output "$archive_path" \
    "https://github.com/${repository}/releases/download/${version}/${archive}"

  actual_sha256="$(sha256sum "$archive_path")"
  actual_sha256="${actual_sha256%% *}"
  if [[ "$actual_sha256" != "$expected_sha256" ]]; then
    echo "SHA-256 mismatch for ${archive}: expected ${expected_sha256}, got ${actual_sha256}" >&2
    exit 1
  fi

  tar -xzf "$archive_path" -C "$tools_dir" "$executable"
  test -x "${tools_dir}/${executable}"
}

install_verified_archive goreleaser/goreleaser v2.16.0 "$goreleaser_archive" "$goreleaser_sha256" goreleaser
install_verified_archive anchore/syft v1.51.0 "$syft_archive" "$syft_sha256" syft

"${tools_dir}/goreleaser" --version
"${tools_dir}/syft" version
printf 'RELEASE_TOOLS_DIR=%s\n' "$tools_dir" >> "$github_env"
