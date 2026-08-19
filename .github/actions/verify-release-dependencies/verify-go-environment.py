#!/usr/bin/env python3
"""Reject inherited Go execution overrides through real authenticated builds."""

import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path.cwd()
POLICY = ROOT / ".github/actions/verify-release-dependencies/trusted-go-environment.sh"
EXPECTED_GO_ENVIRONMENT = {
    "GOENV": "off",
    "GOFLAGS": "",
    "GOCACHEPROG": "",
    "GOWORK": "off",
    "GOTOOLCHAIN": "local",
    "GO111MODULE": "on",
    "CGO_ENABLED": "0",
    "GOEXPERIMENT": "",
    "GOFIPS140": "off",
    "GODEBUG": "",
    "GOTMPDIR": "",
    "GOAUTH": "netrc",
    "GCCGO": "",
    "GCCGOTOOLDIR": "",
    "CC": "",
    "CXX": "",
    "FC": "",
    "AR": "",
    "PKG_CONFIG": "",
}


def trusted_environment(hostile, directory, case):
    persisted = directory / f"{case}.env"
    environment = hostile | {
        "GITHUB_ENV": str(persisted),
        "TRUSTED_GO_POLICY": str(POLICY),
    }
    subprocess.run(
        ["bash", "-c", 'source "$TRUSTED_GO_POLICY"'], env=environment, check=True
    )
    trusted = environment | dict(
        line.split("=", 1)
        for line in persisted.read_text(encoding="utf-8").splitlines()
    )

    for name, expected in EXPECTED_GO_ENVIRONMENT.items():
        actual = trusted.get(name)
        if actual != expected:
            raise SystemExit(
                f"trusted Go environment did not persist {name}={expected!r}: "
                f"got {actual!r}"
            )

    trusted_root = subprocess.check_output(
        ["go", "env", "GOROOT"], env=trusted, text=True
    ).strip()
    if trusted.get("GOROOT") != trusted_root:
        raise SystemExit("trusted Go environment did not pin the installed GOROOT")

    return trusted


def build_reviewed_provider(release_tools, environment, directory, case):
    provider = directory / f"{case}-provider"
    subprocess.run(
        [
            str(release_tools / "goreleaser"),
            "build",
            "--snapshot",
            "--single-target",
            "--clean",
            "--output",
            str(provider),
        ],
        env=environment,
        check=True,
    )
    contents = provider.read_bytes()
    if b"registry.terraform.io/openai/openai" not in contents:
        raise SystemExit(f"{case}: real GoReleaser did not build reviewed provider")
    if b"registry.terraform.io/openai/attacker-controlled" in contents:
        raise SystemExit(f"{case}: unreviewed source overlay changed provider binary")


def main():
    release_tools = Path(os.environ["RELEASE_TOOLS_DIR"])

    with tempfile.TemporaryDirectory(prefix="trusted-release-go-") as temporary:
        directory = Path(temporary)
        marker = directory / "credential-access"
        wrapper = directory / "untrusted-executable"
        wrapper.write_text(
            "#!/usr/bin/env bash\n"
            "set -euo pipefail\n"
            f"printf '%s|%s\\n' \"$GITHUB_TOKEN\" "
            f"\"$GPG_SIGNING_SENTINEL\" > '{marker}'\n"
            "exit 97\n",
            encoding="utf-8",
        )
        wrapper.chmod(0o755)

        ambient_goenv = directory / "ambient-goenv"
        ambient_goenv.write_text(f"GOFLAGS=-toolexec={wrapper}\n", encoding="utf-8")

        original_source = ROOT / "main.go"
        overlay_source = directory / "overlay-main.go"
        overlay_source.write_text(
            original_source.read_text(encoding="utf-8").replace(
                "registry.terraform.io/openai/openai",
                "registry.terraform.io/openai/attacker-controlled",
            ),
            encoding="utf-8",
        )
        overlay = directory / "overlay.json"
        overlay.write_text(
            json.dumps({"Replace": {str(original_source): str(overlay_source)}}),
            encoding="utf-8",
        )

        hostile = os.environ | {
            "GOENV": str(ambient_goenv),
            "GOCACHEPROG": str(wrapper),
            "GOWORK": str(directory / "attacker.go.work"),
            "GOTOOLCHAIN": "go999.999.999+auto",
            "GOROOT": str(directory / "attacker-goroot"),
            "GOEXPERIMENT": "attacker-controlled",
            "GOFIPS140": "attacker-controlled",
            "GOAUTH": f"command {wrapper}",
            "GOTMPDIR": str(directory / "attacker-temp"),
            "GCCGO": str(wrapper),
            "GCCGOTOOLDIR": str(directory / "attacker-tools"),
            "CC": str(wrapper),
            "CXX": str(wrapper),
            "FC": str(wrapper),
            "AR": str(wrapper),
            "PKG_CONFIG": str(wrapper),
            "GITHUB_TOKEN": "synthetic-publish-token",
            "GPG_SIGNING_SENTINEL": "synthetic-imported-gpg-key",
        }

        cases = {
            "ambient-goenv-toolexec": None,
            "inherited-toolexec": f"-toolexec={wrapper}",
            "inherited-overlay": f"-overlay={overlay}",
        }
        for case, flags in cases.items():
            candidate = hostile.copy()
            if flags is None:
                candidate.pop("GOFLAGS", None)
            else:
                candidate["GOFLAGS"] = flags

            trusted = trusted_environment(candidate, directory, case)
            subprocess.run(["go", "mod", "verify"], env=trusted, check=True)
            build_reviewed_provider(release_tools, trusted, directory, case)
            if marker.exists():
                raise SystemExit(
                    f"{case}: inherited Go executable accessed signing credentials: "
                    f"{marker.read_text(encoding='utf-8').strip()}"
                )

    print(
        "Verified real GoReleaser builds reject ambient GOENV, -toolexec, "
        "-overlay, external cache programs, workspaces, and toolchain overrides."
    )


if __name__ == "__main__":
    main()
