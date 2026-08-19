#!/usr/bin/env python3
"""Run the real release snapshot against an attacker-controlled Syft cache/proxy."""

from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from io import BytesIO
import json
import os
from pathlib import Path
import subprocess
import tempfile
from threading import Thread
import zipfile


MODULE = "github.com/openai/openai-go/v3"
MIT_LICENSE = """MIT License

Copyright (c) 2026 Release-boundary regression test

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""


class HostileModuleProxy(BaseHTTPRequestHandler):
    requests = []

    def do_GET(self):
        self.requests.append(self.path)
        module, separator, version = self.path.lstrip("/").partition("/@v/")
        if not separator or not version.endswith(".zip"):
            self.send_error(404)
            return

        archive = BytesIO()
        with zipfile.ZipFile(archive, "w") as output:
            output.writestr(f"{module}@{version[:-4]}/LICENSE", MIT_LICENSE)

        payload = archive.getvalue()
        self.send_response(200)
        self.send_header("Content-Type", "application/zip")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, _format, *_args):
        return


@contextmanager
def hostile_proxy():
    server = ThreadingHTTPServer(("127.0.0.1", 0), HostileModuleProxy)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}"
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def module_version():
    return subprocess.check_output(
        ["go", "list", "-mod=readonly", "-m", "-f", "{{.Version}}", MODULE],
        text=True,
    ).strip()


def write_hostile_home(directory, proxy_url, version):
    cache = directory / "go" / "pkg" / "mod"
    malicious_license = cache / f"{MODULE}@{version}" / "LICENSE"
    malicious_license.parent.mkdir(parents=True)
    malicious_license.write_text(MIT_LICENSE, encoding="utf-8")

    (directory / ".syft.yaml").write_text(
        "check-for-app-update: true\n"
        "enrich:\n"
        "  - all\n"
        "golang:\n"
        f"  local-mod-cache-dir: '{cache}'\n"
        f"  proxy: '{proxy_url}'\n"
        "  search-local-mod-cache-licenses: true\n"
        "  search-local-vendor-licenses: false\n"
        "  search-remote-licenses: true\n",
        encoding="utf-8",
    )


def verify_sboms():
    sboms = sorted(Path("dist").rglob("*.spdx.json"))
    if not sboms:
        raise SystemExit("release snapshot did not produce any SPDX SBOMs")

    for sbom in sboms:
        packages = json.loads(sbom.read_text(encoding="utf-8"))["packages"]
        package = next(
            (entry for entry in packages if entry["name"] == MODULE), None
        )
        if package is None:
            raise SystemExit(f"{sbom}: missing expected module {MODULE}")
        if package.get("licenseConcluded") != "Apache-2.0":
            raise SystemExit(
                f"{sbom}: untrusted cache/proxy changed {MODULE} license to "
                f"{package.get('licenseConcluded')!r}"
            )


def main():
    release_tools = Path(os.environ["RELEASE_TOOLS_DIR"])
    verified_cache = Path(os.environ["GOMODCACHE"])
    version = module_version()
    expected_license = verified_cache / f"{MODULE}@{version}" / "LICENSE"
    if not expected_license.is_file():
        raise SystemExit(f"verified module cache is missing {expected_license}")

    with tempfile.TemporaryDirectory(prefix="syft-release-boundary-") as temporary:
        with hostile_proxy() as proxy_url:
            hostile_home = Path(temporary)
            write_hostile_home(hostile_home, proxy_url, version)

            environment = os.environ.copy()
            environment["HOME"] = str(hostile_home)
            environment["PATH"] = f"{release_tools}{os.pathsep}{environment['PATH']}"
            subprocess.run(
                [
                    str(release_tools / "goreleaser"),
                    "release",
                    "--snapshot",
                    "--clean",
                    "--skip=sign",
                ],
                env=environment,
                check=True,
            )

            if HostileModuleProxy.requests:
                raise SystemExit(
                    f"Syft contacted the hostile module proxy "
                    f"{len(HostileModuleProxy.requests)} times; first request: "
                    f"{HostileModuleProxy.requests[0]}"
                )
            verify_sboms()

    print(f"Verified {MODULE} SBOM licenses without hostile-cache or proxy access.")


if __name__ == "__main__":
    main()
