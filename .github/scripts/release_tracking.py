"""Validate release bookkeeping and finish release-please's label lifecycle."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

REPOSITORY = "openai/terraform-provider-openai"
RELEASE_BRANCH = "release-please--branches--main--components--terraform-provider-openai"
PENDING = "autorelease: pending"
TAGGED = "autorelease: tagged"


def github_api(method, endpoint, payload=None, paginate=False):
    command = ["gh", "api", "--method", method, f"repos/{REPOSITORY}/{endpoint}"]
    if paginate:
        command.extend(["--paginate", "--slurp"])
    if payload is not None:
        command.extend(["--input", "-"])
    result = subprocess.run(
        command,
        input=json.dumps(payload) if payload is not None else None,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode:
        # Do not echo API response bodies or credential-bearing diagnostics.
        raise ValueError(f"GitHub API {method} request failed")
    return json.loads(result.stdout) if result.stdout.strip() else None


def verify_files(root, tag):
    identifier = r"(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)"
    semver = rf"v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-{identifier}(?:\.{identifier})*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
    if not re.fullmatch(semver, tag):
        raise ValueError("release tag must be a v-prefixed semantic version")
    version = tag[1:]
    manifest = json.loads((root / ".release-please-manifest.json").read_text())
    if manifest.get(".") != version:
        raise ValueError("release tag does not match the committed manifest version")
    heading = rf"^## \[{re.escape(version)}\]\(https://github\.com/{REPOSITORY}/compare/[^\n)]+\.\.\.{re.escape(tag)}\) \([0-9]{{4}}-[0-9]{{2}}-[0-9]{{2}}\)$"
    if not re.search(heading, (root / "CHANGELOG.md").read_text(), re.MULTILINE):
        raise ValueError("release tag has no matching committed changelog entry")


def release_pull_request(tag, sha):
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("release commit must be a full commit SHA")
    pages = github_api("GET", f"commits/{sha}/pulls?per_page=100", paginate=True)
    matches = []
    for page in pages:
        for pr in page:
            if (
                pr.get("merged_at")
                and pr.get("merge_commit_sha") == sha
                and pr.get("base", {}).get("ref") == "main"
                and pr.get("base", {}).get("repo", {}).get("full_name") == REPOSITORY
                and pr.get("head", {}).get("ref") == RELEASE_BRANCH
                and (pr.get("head", {}).get("repo") or {}).get("full_name")
                == REPOSITORY
                and pr.get("title") == f"chore(main): release {tag[1:]}"
            ):
                matches.append(pr)
    if len(matches) != 1:
        raise ValueError("tag must point to exactly one matching merged release PR")
    return matches[0]


def complete_release(tag, pr):
    release = github_api("GET", f"releases/tags/{tag}")
    if (
        release.get("tag_name") != tag
        or release.get("draft") is not False
        or not release.get("published_at")
    ):
        raise ValueError("release is not published; retaining the pending label")
    number = pr["number"]
    pages = github_api("GET", f"issues/{number}/labels?per_page=100", paginate=True)
    labels = {label["name"] for page in pages for label in page}
    # Add first so a failed removal is safe to retry. Preserve unrelated labels.
    if TAGGED not in labels:
        github_api("POST", f"issues/{number}/labels", {"labels": [TAGGED]})
    if PENDING in labels:
        github_api("DELETE", f"issues/{number}/labels/autorelease%3A%20pending")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=["verify", "complete"])
    args = parser.parse_args()
    if os.environ.get("GITHUB_REPOSITORY") != REPOSITORY:
        raise ValueError(
            "release tracking is restricted to the public provider repository"
        )
    tag = os.environ["GITHUB_REF_NAME"]
    if os.environ.get("GITHUB_REF_TYPE") != "tag":
        raise ValueError("release tracking requires a tag event")
    verify_files(Path.cwd(), tag)
    pr = release_pull_request(tag, os.environ["GITHUB_SHA"])
    if args.mode == "complete":
        complete_release(tag, pr)
    print(f"Release tracking {args.mode} succeeded for {tag} (PR #{pr['number']})")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError) as error:
        sys.exit(f"Release tracking failed: {error}")
