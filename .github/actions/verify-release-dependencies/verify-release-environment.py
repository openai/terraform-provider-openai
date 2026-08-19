#!/usr/bin/env python3
"""Reject inherited Go and Git overrides through real authenticated builds."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


ROOT = Path.cwd()
POLICY = (
    ROOT / ".github/actions/verify-release-dependencies/trusted-release-environment.sh"
)
EXPECTED_RELEASE_ENVIRONMENT = {
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
    "GIT_CONFIG_GLOBAL": "/dev/null",
    "GIT_CONFIG_SYSTEM": "/dev/null",
    "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_CONFIG_PARAMETERS": "",
    "GIT_CONFIG_COUNT": "5",
    "GIT_CONFIG_KEY_0": "core.fsmonitor",
    "GIT_CONFIG_VALUE_0": "false",
    "GIT_CONFIG_KEY_1": "core.hooksPath",
    "GIT_CONFIG_VALUE_1": "/dev/null",
    "GIT_CONFIG_KEY_2": "diff.external",
    "GIT_CONFIG_VALUE_2": "",
    "GIT_CONFIG_KEY_3": "credential.helper",
    "GIT_CONFIG_VALUE_3": "",
    "GIT_CONFIG_KEY_4": "core.askPass",
    "GIT_CONFIG_VALUE_4": "",
    "GIT_EXTERNAL_DIFF": "",
    "GIT_ASKPASS": "",
    "GIT_SSH": "",
    "GIT_SSH_COMMAND": "",
    "GIT_PAGER": "cat",
    "GIT_EDITOR": "false",
    "GIT_SEQUENCE_EDITOR": "false",
    "GIT_TERMINAL_PROMPT": "0",
}


def trusted_environment(hostile, directory, case):
    persisted = directory / f"{case}.env"
    environment = hostile | {
        "GITHUB_ENV": str(persisted),
        "TRUSTED_RELEASE_POLICY": str(POLICY),
    }
    subprocess.run(
        ["bash", "-c", 'source "$TRUSTED_RELEASE_POLICY"'],
        env=environment,
        check=True,
    )
    trusted = environment | dict(
        line.split("=", 1)
        for line in persisted.read_text(encoding="utf-8").splitlines()
    )

    for name, expected in EXPECTED_RELEASE_ENVIRONMENT.items():
        actual = trusted.get(name)
        if actual != expected:
            raise SystemExit(
                f"trusted release environment did not persist {name}={expected!r}: "
                f"got {actual!r}"
            )

    trusted_root = subprocess.check_output(
        ["go", "env", "GOROOT"], env=trusted, text=True
    ).strip()
    if trusted.get("GOROOT") != trusted_root:
        raise SystemExit("trusted release environment did not pin the installed GOROOT")

    trusted_git_exec_path = subprocess.check_output(
        ["git", "--exec-path"], env=trusted, text=True
    ).strip()
    if trusted.get("GIT_EXEC_PATH") != trusted_git_exec_path:
        raise SystemExit(
            "trusted release environment did not pin Git's executable root"
        )

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


def hostile_git_configurations(directory, hook):
    included_configuration = directory / "hostile-included-gitconfig"
    included_configuration.write_text(
        f"[core]\n\tfsmonitor = {hook}\n"
        f"\thooksPath = {directory}\n"
        f"\taskPass = {hook}\n"
        f"\tpager = {hook}\n"
        f"\teditor = {hook}\n"
        f"\tsshCommand = {hook}\n"
        f"[diff]\n\texternal = {hook}\n"
        f"[credential]\n\thelper = !{hook}\n",
        encoding="utf-8",
    )
    hostile_configuration = directory / "hostile-gitconfig"
    hostile_configuration.write_text(
        f"[include]\n\tpath = {included_configuration}\n", encoding="utf-8"
    )
    hostile_home = directory / "hostile-home"
    hostile_home.mkdir()
    shutil.copy2(hostile_configuration, hostile_home / ".gitconfig")
    hostile_xdg = directory / "hostile-xdg" / "git"
    hostile_xdg.mkdir(parents=True)
    shutil.copy2(hostile_configuration, hostile_xdg / "config")
    verified_module_cache = subprocess.check_output(
        ["go", "env", "GOMODCACHE"], text=True
    ).strip()
    return {
        "global-fsmonitor": {"GIT_CONFIG_GLOBAL": str(hostile_configuration)},
        "system-fsmonitor": {"GIT_CONFIG_SYSTEM": str(hostile_configuration)},
        "home-fsmonitor": {
            "HOME": str(hostile_home),
            "GOMODCACHE": verified_module_cache,
        },
        "xdg-fsmonitor": {"XDG_CONFIG_HOME": str(hostile_xdg.parent)},
        "inherited-config-pairs": {
            "GIT_CONFIG_COUNT": "2",
            "GIT_CONFIG_KEY_0": "core.fsmonitor",
            "GIT_CONFIG_VALUE_0": str(hook),
            "GIT_CONFIG_KEY_1": "core.hooksPath",
            "GIT_CONFIG_VALUE_1": str(directory),
        },
        "inherited-config-parameters": {
            "GIT_CONFIG_PARAMETERS": (
                f"'core.fsmonitor={hook}' 'core.hooksPath={directory}'"
            )
        },
    }


def verify_hostile_git_configuration(release_tools, directory, hostile):
    hook_marker = directory / "git-hook-executed"
    credential_marker = directory / "git-hook-credential-access"
    hook = directory / "hostile-fsmonitor"
    replacement = directory / "replacement-goreleaser"

    replacement.write_text(
        "#!/usr/bin/env bash\n"
        "set -euo pipefail\n"
        f"printf '%s|%s\\n' \"$GITHUB_TOKEN\" "
        f"\"$GPG_SIGNING_SENTINEL\" > '{credential_marker}'\n"
        'exec "$RELEASE_TOOLS_DIR/goreleaser.authenticated" "$@"\n',
        encoding="utf-8",
    )
    replacement.chmod(0o755)

    hook.write_text(
        "#!/usr/bin/env bash\n"
        "set -euo pipefail\n"
        f"if [[ ! -e '{hook_marker}' ]]; then\n"
        f"  printf 'unreviewed Git executable\\n' > '{hook_marker}'\n"
        '  mv "$RELEASE_TOOLS_DIR/goreleaser" '
        '"$RELEASE_TOOLS_DIR/goreleaser.authenticated"\n'
        f"  cp '{replacement}' \"$RELEASE_TOOLS_DIR/goreleaser\"\n"
        '  chmod +x "$RELEASE_TOOLS_DIR/goreleaser"\n'
        "fi\n"
        "printf 'attacker-token\\0'\n",
        encoding="utf-8",
    )
    hook.chmod(0o755)

    isolated_tools = directory / "isolated-release-tools"
    isolated_tools.mkdir()
    shutil.copy2(release_tools / "goreleaser", isolated_tools / "goreleaser")
    hostile_git_environment = {
        name: value
        for name, value in hostile.items()
        if not name.startswith("GIT_CONFIG_")
    }

    for case, configuration in hostile_git_configurations(directory, hook).items():
        candidate = (
            hostile_git_environment
            | configuration
            | {"RELEASE_TOOLS_DIR": str(isolated_tools)}
        )
        trusted = trusted_environment(candidate, directory, case)

        subprocess.run(["go", "mod", "verify"], env=trusted, check=True)
        subprocess.run(
            ["git", "diff", "--exit-code", "--", "go.mod", "go.sum"],
            env=trusted,
            check=True,
        )

        if hook_marker.exists():
            build_reviewed_provider(isolated_tools, trusted, directory, case)
            raise SystemExit(
                f"{case}: final Git lockfile verification executed an inherited "
                "fsmonitor hook, replaced authenticated GoReleaser, and exposed "
                "signing credentials: "
                f"{credential_marker.read_text(encoding='utf-8').strip()}"
            )

        if case == "global-fsmonitor":
            build_reviewed_provider(isolated_tools, trusted, directory, case)


def main():
    release_tools = Path(os.environ["RELEASE_TOOLS_DIR"])

    with tempfile.TemporaryDirectory(
        prefix="trusted-release-environment-"
    ) as temporary:
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
            "GIT_EXEC_PATH": str(directory / "attacker-git-tools"),
            "GIT_EXTERNAL_DIFF": str(wrapper),
            "GIT_ASKPASS": str(wrapper),
            "GIT_SSH": str(wrapper),
            "GIT_SSH_COMMAND": str(wrapper),
            "GIT_PAGER": str(wrapper),
            "GIT_EDITOR": str(wrapper),
            "GIT_SEQUENCE_EDITOR": str(wrapper),
            "GITHUB_TOKEN": "synthetic-publish-token",
            "GPG_SIGNING_SENTINEL": "synthetic-imported-gpg-key",
        }

        verify_hostile_git_configuration(release_tools, directory, hostile)

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
        "Verified real GoReleaser builds reject ambient Git hooks/configuration, "
        "GOENV, -toolexec, -overlay, external cache programs, workspaces, "
        "and toolchain overrides."
    )


if __name__ == "__main__":
    main()
