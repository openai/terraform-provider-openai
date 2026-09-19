"""Offline regressions for tag validation and post-publication label updates."""

import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import call, patch

import release_tracking as tracking

SHA = "a" * 40
TAG = "v1.2.3"


def release_pr():
    return {
        "number": 56,
        "merged_at": "2026-09-15T00:00:00Z",
        "merge_commit_sha": SHA,
        "base": {"ref": "main", "repo": {"full_name": tracking.REPOSITORY}},
        "head": {
            "ref": tracking.RELEASE_BRANCH,
            "repo": {"full_name": tracking.REPOSITORY},
        },
        "title": "chore(main): release 1.2.3",
    }


class ReleaseTrackingTest(unittest.TestCase):
    def test_tag_must_match_both_committed_files(self):
        cases = [
            (TAG, "1.2.3", TAG, None),
            ("v1.2.3-rc.1", "1.2.3-rc.1", "v1.2.3-rc.1", None),
            ("v01.2.3", "01.2.3", "v01.2.3", "semantic version"),
            ("v1.2.3-01", "1.2.3-01", "v1.2.3-01", "semantic version"),
            ("main", "1.2.3", TAG, "semantic version"),
            (TAG, "1.2.2", TAG, "manifest version"),
            (TAG, "1.2.3", "v1.2.2", "changelog entry"),
        ]
        for tag, version, heading_tag, error in cases:
            with self.subTest(tag=tag, version=version, heading=heading_tag):
                with tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    (root / ".release-please-manifest.json").write_text(
                        json.dumps({".": version})
                    )
                    (root / "CHANGELOG.md").write_text(
                        f"## [{heading_tag[1:]}](https://github.com/"
                        f"{tracking.REPOSITORY}/compare/v1.2.2..."
                        f"{heading_tag}) (2026-09-15)\n"
                    )
                    if error:
                        with self.assertRaisesRegex(ValueError, error):
                            tracking.verify_files(root, tag)
                    else:
                        tracking.verify_files(root, tag)

    def test_only_the_matching_merged_release_pr_is_selected(self):
        mutations = [
            {"merged_at": None},
            {"merge_commit_sha": "b" * 40},
            {"title": "chore(main): release 1.2.2"},
            {"base": {"ref": "other", "repo": {"full_name": tracking.REPOSITORY}}},
            {"head": {"ref": "unrelated", "repo": {"full_name": tracking.REPOSITORY}}},
            {
                "head": {
                    "ref": tracking.RELEASE_BRANCH,
                    "repo": {"full_name": "other/fork"},
                }
            },
            {"head": {"ref": tracking.RELEASE_BRANCH, "repo": None}},
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                invalid = copy.deepcopy(release_pr())
                invalid.update(mutation)
                with patch.object(tracking, "github_api", return_value=[[invalid]]):
                    with self.assertRaisesRegex(
                        ValueError, "matching merged release PR"
                    ):
                        tracking.release_pull_request(TAG, SHA)
        with patch.object(tracking, "github_api", return_value=[[], [release_pr()]]):
            self.assertEqual(tracking.release_pull_request(TAG, SHA)["number"], 56)
        for pages in [[], [[release_pr(), release_pr()]]]:
            with patch.object(tracking, "github_api", return_value=pages):
                with self.assertRaisesRegex(ValueError, "exactly one"):
                    tracking.release_pull_request(TAG, SHA)

    def test_publication_requires_a_draft_at_the_release_commit(self):
        draft = {"tag_name": TAG, "draft": True, "target_commitish": SHA}
        with patch.object(tracking, "github_api", return_value=[[], [draft]]) as api:
            tracking.verify_draft(TAG, SHA)
            api.assert_called_once_with("GET", "releases?per_page=100", paginate=True)
        for pages in [[], [[draft, draft]], [[draft | {"tag_name": "v9.9.9"}]]]:
            with self.subTest(pages=pages):
                with patch.object(tracking, "github_api", return_value=pages):
                    with self.assertRaisesRegex(ValueError, "exactly one draft"):
                        tracking.verify_draft(TAG, SHA)
        for mutation in [
            {"draft": False},
            {"target_commitish": "main"},
            {"target_commitish": "b" * 40},
        ]:
            with self.subTest(mutation=mutation):
                with patch.object(
                    tracking, "github_api", return_value=[[draft | mutation]]
                ):
                    with self.assertRaisesRegex(ValueError, "draft at the verified"):
                        tracking.verify_draft(TAG, SHA)
        with patch.object(tracking, "github_api", side_effect=ValueError("API failed")):
            with self.assertRaisesRegex(ValueError, "API failed"):
                tracking.verify_draft(TAG, SHA)

    def test_unpublished_release_never_changes_labels(self):
        for release in [
            {"tag_name": TAG, "draft": True, "published_at": None},
            {"tag_name": TAG, "draft": False, "published_at": None},
            {"tag_name": "v1.2.2", "draft": False, "published_at": "date"},
        ]:
            with self.subTest(release=release):
                with patch.object(tracking, "github_api", return_value=release) as api:
                    with self.assertRaisesRegex(ValueError, "not published"):
                        tracking.complete_release(TAG, release_pr())
                    api.assert_called_once_with("GET", f"releases/tags/{TAG}")

    def test_publication_preserves_release_please_labels_and_retries(self):
        published = {"tag_name": TAG, "draft": False, "published_at": "date"}
        endpoint = "issues/56/labels"
        for existing in [
            ["autorelease: tagged", "documentation"],
            ["autorelease: tagged", tracking.PUBLISHED, "documentation"],
            ["autorelease: pending", "documentation"],
        ]:
            with self.subTest(existing=existing):
                with patch.object(tracking, "github_api") as api:
                    api.side_effect = [
                        published,
                        [[{"name": x} for x in existing]],
                        None,
                        None,
                    ]
                    tracking.complete_release(TAG, release_pr())
                    expected = [
                        call("GET", f"releases/tags/{TAG}"),
                        call("GET", endpoint + "?per_page=100", paginate=True),
                    ]
                    if tracking.PUBLISHED not in existing:
                        expected.append(
                            call("POST", endpoint, {"labels": [tracking.PUBLISHED]})
                        )
                    self.assertEqual(api.call_args_list, expected)
        with patch.object(tracking, "github_api") as api:
            api.side_effect = [
                published,
                [[{"name": "autorelease: tagged"}]],
                ValueError("API failed"),
            ]
            with self.assertRaisesRegex(ValueError, "API failed"):
                tracking.complete_release(TAG, release_pr())
            self.assertNotIn("DELETE", [args.args[0] for args in api.call_args_list])

    def test_api_failure_does_not_expose_response(self):
        with patch.object(tracking.subprocess, "run") as run:
            run.return_value.returncode = 1
            run.return_value.stderr = "secret-bearing response"
            with self.assertRaisesRegex(
                ValueError, "GitHub API GET request failed"
            ) as error:
                tracking.github_api("GET", "releases/tags/v1.2.3")
            self.assertNotIn("secret", str(error.exception))

    def test_workflow_gates(self):
        root = Path(__file__).resolve().parents[2]
        workflow = (root / ".github/workflows/release.yml").read_text()
        gate = workflow.index("release_tracking.py verify")
        self.assertLess(
            gate, workflow.index("uses: ./.github/actions/verify-release-sboms")
        )
        self.assertIn("needs: release-sbom", workflow)
        snapshot_job, publisher = workflow.split("  goreleaser:\n", 1)
        self.assertIn("contents: read", snapshot_job)
        self.assertNotIn("contents: write", snapshot_job.split("  release-sbom:", 1)[1])
        self.assertIn("contents: write", publisher)
        self.assertNotIn("release_tracking.py verify-draft", snapshot_job)
        self.assertLess(
            publisher.index("release_tracking.py verify-draft"),
            publisher.index("name: Check publish environment secrets"),
        )
        self.assertNotIn("secrets.", snapshot_job)
        self.assertIn("needs: release-sbom", publisher)
        completion = workflow.split("  complete-release:\n", 1)[1]
        self.assertIn("needs: goreleaser", completion)
        self.assertNotIn("always()", completion)
        self.assertNotIn("secrets.", completion)
        self.assertIn("pull-requests: write", completion)
        please = (root / ".github/workflows/release-please.yml").read_text()
        self.assertNotIn("skip-labeling: true", please)
        self.assertNotIn("skip-github-release: true", please)
        # A draft alone does not create a tag and would strand publication.
        config = json.loads((root / "release-please-config.json").read_text())
        package = config["packages"]["."]
        self.assertIs(package["draft"], True)
        self.assertIs(package["force-tag-creation"], True)
        self.assertIn("token: ${{ steps.app-token.outputs.token }}", please)
        goreleaser = (root / ".goreleaser.yml").read_text()
        self.assertIn("use_existing_draft: true", goreleaser)
        self.assertIn('goreleaser" release --clean --draft', workflow)
        self.assertLess(
            workflow.index("--source-digest"),
            workflow.index('release-verifier --publish "$GITHUB_REF_NAME"'),
        )

    def test_completion_is_restricted_to_public_tag_events(self):
        with patch.dict(os.environ, {"GITHUB_REPOSITORY": "other/repo"}, clear=True):
            with patch("sys.argv", ["release_tracking.py", "complete"]):
                with self.assertRaisesRegex(ValueError, "public provider repository"):
                    tracking.main()


if __name__ == "__main__":
    unittest.main()
