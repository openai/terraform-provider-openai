#!/usr/bin/env bash

set -euo pipefail

action_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
test_dir="$(mktemp -d "${RUNNER_TEMP:-/tmp}/release-tool-auth-test.XXXXXXXX")"
trap 'rm -rf "$test_dir"' EXIT

mkdir -p "${test_dir}/bin" "${test_dir}/runner"

cat > "${test_dir}/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$RELEASE_TOOL_TEST_MODE" == download-failure ]]; then
  echo "simulated release archive download failure" >&2
  exit 22
fi

while [[ "$#" -gt 0 ]]; do
  if [[ "$1" == --output ]]; then
    printf 'simulated untrusted executable archive\n' > "$2"
    exit 0
  fi
  shift
done

echo "mock curl did not receive an output path" >&2
exit 1
EOF

chmod +x "${test_dir}/bin/curl"

assert_install_rejected() {
  local mode="$1"
  local expected_error="$2"
  local environment_file="${test_dir}/${mode}.env"
  local output_file="${test_dir}/${mode}.log"

  if PATH="${test_dir}/bin:${PATH}" \
    RELEASE_TOOL_TEST_MODE="$mode" \
    RUNNER_TEMP="${test_dir}/runner" \
    GITHUB_ENV="$environment_file" \
    bash "${action_dir}/install.sh" > "$output_file" 2>&1; then
    echo "release tool installation unexpectedly accepted ${mode}" >&2
    exit 1
  fi

  if ! grep -Fq "$expected_error" "$output_file"; then
    cat "$output_file" >&2
    echo "release tool installation did not report ${mode}" >&2
    exit 1
  fi

  if [[ -s "$environment_file" ]]; then
    echo "release tool installation exposed unverified executables after ${mode}" >&2
    exit 1
  fi
}

assert_install_rejected tampered-archive 'SHA-256 mismatch'
assert_install_rejected download-failure 'simulated release archive download failure'

portable_bin="${test_dir}/portable-bin"
mkdir -p "$portable_bin"
for command_name in awk bash env mktemp; do
  ln -s "$(command -v "$command_name")" "${portable_bin}/${command_name}"
done
ln -s "${test_dir}/bin/curl" "${portable_bin}/curl"
cat > "${portable_bin}/uname" <<'EOF'
#!/usr/bin/env bash
case "$1" in
  -s) printf 'Darwin\n' ;;
  -m) printf 'arm64\n' ;;
  *) exit 1 ;;
esac
EOF
cat > "${portable_bin}/shasum" <<'EOF'
#!/usr/bin/env bash
test "$1" = -a
test "$2" = 256
printf '%064d  %s\n' 0 "$3"
EOF
chmod +x "${portable_bin}/uname" "${portable_bin}/shasum"

portable_log="${test_dir}/portable-checksum.log"
if PATH="$portable_bin" \
  RELEASE_TOOL_TEST_MODE=tampered-archive \
  RUNNER_TEMP="${test_dir}/runner" \
  GITHUB_ENV="${test_dir}/portable-checksum.env" \
  bash "${action_dir}/install.sh" > "$portable_log" 2>&1; then
  echo "release tool installation unexpectedly accepted a tampered Darwin archive" >&2
  exit 1
fi
if ! grep -Fq 'SHA-256 mismatch' "$portable_log"; then
  cat "$portable_log" >&2
  echo "release tool installation did not use the Darwin shasum fallback" >&2
  exit 1
fi

echo 'Release tool installation fails closed and supports the Darwin shasum fallback.'
