import unittest

from check_pr_body import TEMPLATE_PATH, check_body, check_pull_request

TEMPLATE = TEMPLATE_PATH.read_text()
BODY = """## Description
Add a PR body check.
## Breaking change
No breaking change.
## Behaviour change
No behaviour change.
## How has this been tested?
CI covers this: test_check_pr_body.py.
## Performance
CI only, nothing runs in dashbrr.
## Checklist
- [ ] My PR title follows the Conventional Commits format
## AI disclosure
Claude wrote all of it.
"""


class TestCheckPRBody(unittest.TestCase):
    def test_sections(self):
        cases = [
            ("complete", BODY, []),
            ("crlf", BODY.replace("\n", "\r\n"), []),
            ("lowercase heading", BODY.replace("## Performance", "## performance"), []),
            ("three-space heading", BODY.replace("## Performance", "   ## Performance"), []),
            (
                "four-space heading",
                BODY.replace("## Performance", "    ## Performance"),
                ["Missing section: Performance"],
            ),
            (
                "missing required",
                BODY.replace("## Behaviour change\nNo behaviour change.\n", ""),
                ["Missing section: Behaviour change"],
            ),
            ("optional deleted", BODY, []),
            (
                "testing missing",
                BODY.replace("## How has this been tested?\nCI covers this: test_check_pr_body.py.\n", ""),
                ["Missing section: How has this been tested?"],
            ),
            (
                "testing with only the template comment",
                BODY.replace(
                    "CI covers this: test_check_pr_body.py.",
                    TEMPLATE.split("## How has this been tested?")[1].split("## Performance")[0].strip(),
                ),
                ["Empty or unchanged template section: How has this been tested?"],
            ),
            (
                "testing filled",
                BODY.replace("CI covers this: test_check_pr_body.py.", "Ran it against Sonarr 4.0."),
                [],
            ),
            (
                "optional empty",
                BODY + "## Screenshots (for UI changes)\n",
                ["Empty or unchanged template section: Screenshots (for UI changes)"],
            ),
            (
                "comments only",
                BODY.replace("CI only, nothing runs in dashbrr.", "<!-- Explain performance -->"),
                ["Empty or unchanged template section: Performance"],
            ),
            (
                "only the issue placeholder",
                BODY.replace("Add a PR body check.", "Fixes # (issue)"),
                ["Empty or unchanged template section: Description"],
            ),
            (
                "AI question unanswered",
                BODY.replace(
                    "Claude wrote all of it.",
                    "Was any AI used in this PR? Roughly how much of the code was AI-written, and what model/tool?",
                ),
                ["Empty or unchanged template section: AI disclosure"],
            ),
            (
                "AI answer on the question line",
                BODY.replace(
                    "Claude wrote all of it.",
                    "Was any AI used in this PR? Roughly how much of the code was AI-written, "
                    "and what model/tool? No AI.",
                ),
                [],
            ),
            (
                "performance without a reason",
                BODY.replace("CI only, nothing runs in dashbrr.", "N/A"),
                ["Performance: explain why no performance risk applies"],
            ),
            (
                "unticked checklist",
                BODY.replace(
                    "- [ ] My PR title follows the Conventional Commits format",
                    TEMPLATE.split("## Checklist")[1].split("## AI disclosure")[0].strip(),
                ),
                [],
            ),
            ("no breaking change plural", BODY.replace("No breaking change.", "no breaking changes"), []),
            (
                "fenced heading",
                BODY.replace(
                    "## Performance\nCI only, nothing runs in dashbrr.",
                    "```markdown\n## Performance\nCI only.\n```",
                ),
                ["Missing section: Performance"],
            ),
            (
                "invalid closing fence",
                BODY.replace(
                    "## Performance\nCI only, nothing runs in dashbrr.",
                    "```markdown\n```example\n## Performance\nCI only.\n```",
                ),
                ["Missing section: Performance"],
            ),
            (
                "tilde fence inside a backtick fence",
                BODY.replace(
                    "## Performance\nCI only, nothing runs in dashbrr.",
                    "```markdown\n~~~\n## Performance\nCI only.\n```",
                ),
                ["Missing section: Performance"],
            ),
            (
                "heading in a quote",
                BODY.replace("## Performance", "> ## Performance"),
                ["Missing section: Performance"],
            ),
            (
                "third-level heading",
                BODY.replace("## Performance", "### Performance"),
                ["Missing section: Performance"],
            ),
            ("setext heading", BODY.replace("## Performance\n", "\nPerformance\n---\n"), []),
            ("heading in the description", BODY.replace("Add a PR body check.", "### Why\nCI."), []),
            (
                "code block as content",
                BODY.replace("CI only, nothing runs in dashbrr.", "```text\n3 requests, 40 ms\n```"),
                [],
            ),
            (
                "untouched template",
                TEMPLATE,
                [
                    "Empty or unchanged template section: Description",
                    "Empty or unchanged template section: Breaking change",
                    "Empty or unchanged template section: Behaviour change",
                    "Empty or unchanged template section: How has this been tested?",
                    "Empty or unchanged template section: Performance",
                    "Empty or unchanged template section: Screenshots (for UI changes)",
                    "Empty or unchanged template section: AI disclosure",
                ],
            ),
            (
                "blank body",
                "",
                [
                    "Missing section: Description",
                    "Missing section: Breaking change",
                    "Missing section: Behaviour change",
                    "Missing section: How has this been tested?",
                    "Missing section: Performance",
                    "Missing section: Checklist",
                    "Missing section: AI disclosure",
                ],
            ),
        ]
        for name, body, expected in cases:
            with self.subTest(name=name):
                self.assertEqual(check_body(body, TEMPLATE), expected)

    def test_breaking_change_label(self):
        breaking = BODY.replace("No breaking change.", "The NetBird token moves to a new setting.")
        cases = [
            ("described without label", breaking, [], 1),
            ("described with label", breaking, ["breaking change"], 0),
            ("no breaking change without label", BODY, [], 0),
        ]
        for name, body, labels, errors in cases:
            with self.subTest(name=name):
                self.assertEqual(len(check_body(body, TEMPLATE, labels)), errors)

    def test_pull_request(self):
        cases = [
            ("bot skips the template", {"body": "Bumps a dependency.", "user": {"type": "Bot"}}, None),
            ("user needs the template", {"body": None, "user": {"type": "User"}, "labels": []}, 7),
            (
                "member may delete the AI disclosure",
                {"body": BODY.split("## AI disclosure")[0], "user": {"type": "User"}, "author_association": "MEMBER"},
                0,
            ),
            (
                "contributor must keep the AI disclosure",
                {"body": BODY.split("## AI disclosure")[0], "user": {"type": "User"}, "author_association": "CONTRIBUTOR"},
                1,
            ),
            (
                "label from the event",
                {
                    "body": BODY.replace("No breaking change.", "Removes a setting."),
                    "user": {"type": "User"},
                    "labels": [{"name": "breaking change"}],
                },
                0,
            ),
        ]
        for name, pr, errors in cases:
            with self.subTest(name=name):
                result = check_pull_request(pr, TEMPLATE)
                self.assertEqual(result if result is None else len(result), errors)


if __name__ == "__main__":
    unittest.main()
