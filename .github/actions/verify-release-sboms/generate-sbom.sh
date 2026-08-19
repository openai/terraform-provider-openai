#!/usr/bin/env bash

set -euo pipefail

archive="${1:?provider archive path is required}"
document="${2:?SPDX document path is required}"
release_tools="${RELEASE_TOOLS_DIR:?authenticated release tools are required}"
action_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"${release_tools}/syft" \
  --config "${action_dir}/syft.yaml" \
  "file:${archive}" \
  --output "spdx-json=${document}"

binary_name="$(unzip -Z -1 "$archive" | awk '
  /^terraform-provider-openai_v/ && index($0, "/") == 0 {
    if (++count == 1) print
  }
  END {
    if (count != 1) exit 1
  }
')"

temporary_dir="$(mktemp -d "${RUNNER_TEMP:-/tmp}/verified-release-sbom.XXXXXXXX")"
trap 'rm -rf "$temporary_dir"' EXIT
binary="${temporary_dir}/provider"
unzip -p "$archive" "$binary_name" > "$binary"

modules="$(
  go version -m "$binary" |
    awk -F '\t' '$2 == "dep" { print $3 "\t" $4 }' |
    jq -Rn '[inputs | split("\t") | {name: .[0], version: .[1]}]'
)"
archive_sha256="$(sha256sum "$archive")"
archive_sha256="${archive_sha256%% *}"

jq -e \
  --arg archive "$(basename "$archive")" \
  --arg archive_sha256 "$archive_sha256" \
  --argjson modules "$modules" \
  '
    . as $document |
    [
      .relationships[]? |
      select(
        .spdxElementId == "SPDXRef-DOCUMENT" and
        .relationshipType == "DESCRIBES"
      ) |
      .relatedSpdxElement
    ] as $roots |
    .spdxVersion and
    .name == $archive and
    ($roots | length) == 1 and
    any(
      .packages[]?;
      .SPDXID == $roots[0] and
      .name == $archive and
      .versionInfo == ("sha256:" + $archive_sha256) and
      (.supplier // "NOASSERTION") == "NOASSERTION"
    ) and
    ($modules | length) > 0 and
    all(
      $modules[];
      . as $module |
      any(
        $document.packages[]?;
        .name == $module.name and .versionInfo == $module.version
      )
    ) and
    any(
      .packages[]?;
      .name == "github.com/openai/openai-go/v3" and
      .licenseConcluded == "Apache-2.0"
    )
  ' \
  "$document" > /dev/null || {
  echo "SBOM failed archive identity, digest, or Go dependency verification: ${document}" >&2
  exit 1
}
