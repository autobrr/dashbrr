# Domain docs

Engineering skills use this repo's domain documentation when exploring the codebase.

## Read before exploring

- Read `CONTEXT.md` at the repo root.
- Read ADRs under `docs/adr/` that affect the area being changed.

If these files do not exist, proceed without mentioning their absence. The `/domain-modeling` skill creates them when the project resolves domain terms or architectural decisions.

## Layout

This repo uses a single-context layout:

```text
/
├── CONTEXT.md
└── docs/adr/
```

## Use glossary terms

Use terms as defined in `CONTEXT.md` when naming domain concepts in issues, proposals, hypotheses, and tests. Do not replace defined terms with synonyms.

If a required concept is missing, reconsider whether the project uses that concept. If it does, record the gap for `/domain-modeling`.

## Flag ADR conflicts

State when proposed work contradicts an existing ADR. Name the ADR and explain why the decision may need reconsideration.
