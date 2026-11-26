import json
import os
import re
import sys
from pathlib import Path

from markdown_it import MarkdownIt

TEMPLATE_PATH = Path(__file__).resolve().parents[1] / "pull_request_template.md"
REQUIRED = {
    "Description",
    "Breaking change",
    "Behaviour change",
    "How has this been tested?",
    "Performance",
    "Checklist",
    "AI disclosure",
}
OPTIONAL_FOR_MEMBERS = {"AI disclosure"}
MEMBERS = {"OWNER", "MEMBER", "COLLABORATOR"}
MAY_STAY_UNCHANGED = {"Checklist"}
BREAKING_LABEL = "breaking change"
NO_BREAKING_CHANGE = re.compile(r"no breaking changes?\.?", re.I)
NO_ANSWER = re.compile(r"(none|n/?a|-)\.?", re.I)
COMMENT = re.compile(r"<!--.*?-->", re.S)
MARKDOWN = MarkdownIt("commonmark")


def sections(markdown):
    lines = markdown.splitlines()
    tokens = MARKDOWN.parse(markdown)
    result = {}
    parts = None
    for index, token in enumerate(tokens):
        if token.level != 0 or token.map is None:
            continue
        if token.type == "heading_open" and token.tag == "h2":
            parts = result.setdefault(tokens[index + 1].content, [])
            continue
        if parts is None:
            continue
        text = "\n".join(lines[token.map[0]:token.map[1]])
        if token.type == "html_block":
            text = COMMENT.sub("", text)
        parts.append(text.strip())
    return {heading: "\n".join(part for part in parts if part) for heading, parts in result.items()}


def check_body(body, template, labels=(), required=REQUIRED):
    actual = {heading.casefold(): text for heading, text in sections(body).items()}
    errors = []
    for title, placeholder in sections(template).items():
        text = actual.get(title.casefold())
        if text is None:
            if title in required:
                errors.append(f"Missing section: {title}")
        elif not text or (title not in MAY_STAY_UNCHANGED and text == placeholder):
            errors.append(f"Empty or unchanged template section: {title}")
        elif title == "Performance" and NO_ANSWER.fullmatch(text):
            errors.append("Performance: explain why no performance risk applies")
        elif title == "How has this been tested?" and NO_ANSWER.fullmatch(text):
            errors.append("How has this been tested?: say how you checked the change")
        elif (
            title == "Breaking change"
            and not NO_BREAKING_CHANGE.fullmatch(text)
            and BREAKING_LABEL not in labels
        ):
            errors.append(
                'Breaking change: the section describes a breaking change, so a maintainer must add '
                f'the "{BREAKING_LABEL}" label. If nothing breaks, write only "No breaking change."'
            )
    return errors


def check_pull_request(pr, template):
    if pr.get("user", {}).get("type") == "Bot":
        return None
    labels = {label["name"] for label in pr.get("labels", [])}
    required = REQUIRED - OPTIONAL_FOR_MEMBERS if pr.get("author_association") in MEMBERS else REQUIRED
    return check_body(pr.get("body") or "", template, labels, required)


def main(argv):
    event_path = argv[1] if len(argv) > 1 else os.environ["GITHUB_EVENT_PATH"]
    pr = json.loads(Path(event_path).read_text())["pull_request"]
    errors = check_pull_request(pr, TEMPLATE_PATH.read_text())
    if errors is None:
        print("Skipped: a bot opened this PR.")
        return 0
    for error in errors:
        print(f"::error::{error}")
    if not errors:
        print("PR template sections are complete.")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
