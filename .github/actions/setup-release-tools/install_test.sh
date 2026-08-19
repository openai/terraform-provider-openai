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

echo 'Release tool installation fails closed for tampered and unavailable archives.'
