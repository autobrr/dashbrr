# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues. Use the `gh` CLI for all operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --comments`, filtering comments by `jq` and also fetching labels.
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'` with appropriate `--label` and `--state` filters.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply or remove labels**: `gh issue edit <number> --add-label "..."` or `--remove-label "..."`
- **Close an issue**: `gh issue close <number> --comment "..."`

Infer the repo from `git remote -v`. `gh` does this automatically inside a clone.

## Pull requests as a triage surface

**PRs as a request surface: no.**

Set this to `yes` if the repo treats external PRs as feature requests. `/triage` reads this flag.

When set to `yes`, PRs use the same labels and states as issues:

- **Read a PR**: `gh pr view <number> --comments` and `gh pr diff <number>`.
- **List external PRs for triage**: `gh pr list --state open --json number,title,body,labels,author,authorAssociation,comments`, then keep only `CONTRIBUTOR`, `FIRST_TIME_CONTRIBUTOR`, or `NONE` author associations.
- **Comment, label, or close**: use `gh pr comment`, `gh pr edit --add-label` or `--remove-label`, and `gh pr close`.

GitHub shares one number space across issues and PRs. Resolve a bare `#42` with `gh pr view 42`, then fall back to `gh issue view 42`.

## When a skill says "publish to the issue tracker"

Create a GitHub issue.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`.

## Wayfinding operations

`/wayfinder` uses one issue as a map and child issues as tickets.

- **Map**: an issue labelled `wayfinder:map` that holds Notes, Decisions-so-far, and Fog. Create it with `gh issue create --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue. Where sub-issues are unavailable, add the child to a task list in the map and put `Part of #<map>` at the top of the child body. Apply a `wayfinder:<type>` label using `research`, `prototype`, `grilling`, or `task`. Assign the ticket to the driving developer after it is claimed.
- **Blocking**: use GitHub issue dependencies. Add an edge with `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`. Fetch the blocker database ID with `gh api repos/<owner>/<repo>/issues/<n> --jq .id`. If dependencies are unavailable, put `Blocked by: #<n>, #<n>` at the top of the child body.
- **Frontier query**: list the map's open children, then drop children with an open blocker or assignee. The first remaining child in map order wins.
- **Claim**: run `gh issue edit <n> --add-assignee @me`. This is the session's first write.
- **Resolve**: comment with the answer, close the issue, then append a context pointer and link to the map's Decisions-so-far.
