## Description

<!--- Summarize the change and the motivation. --->

Fixes # (issue)

## Breaking change

<!--- Write "No breaking change." unless existing users must act after they update, or something they rely on stops working. Examples: a setting, an environment variable, a CLI command, or an API field is renamed or removed. Otherwise say what they must do. A maintainer then adds the breaking change label and puts ! in the title, for example feat(api)!: ... The PR checks fail until the PR has both. --->

## Behaviour change

<!--- Changes that existing users notice after they update but need no action for, for example a new sort order, a new status color, or a card that shows other data. Write "No behaviour change." if there is none. --->

## How has this been tested?

<!--- Only manual or live verification that CI cannot do, for example a run against a real service. Name the service and its version. Delete this section if CI covers everything. --->

## Performance

<!--- Say whether the change can make dashbrr slower or heavier, and how you checked.
      Backend: list the requests dashbrr now sends to each service, how often, and how long they take.
      Name new polling, retries, or timeouts, and what happens when a service is slow or down.
      Web app: name new re-renders, polling from the browser, or large lists, and how long they take to render.
      Include before/after numbers when the change can affect performance.
      If no performance risk applies, explain why. Keep this section. --->

## Screenshots (for UI changes)

<!--- Delete this section if your change has no visible UI.
      Screenshots and recordings on GitHub are public. Before you capture, hide or blur
      API keys and tokens, internal hostnames, IP addresses, and Tailscale or NetBird
      device and peer names. --->

## Checklist

- [ ] My PR title follows the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) format (it becomes the squashed commit message)
- [ ] If this changes the database schema, I have added migrations for both SQLite and PostgreSQL

## AI disclosure

<!--- Please do not lie about the AI usage in this PR. --->

Was any AI used in this PR? Roughly how much of the code was AI-written, and what model/tool?
