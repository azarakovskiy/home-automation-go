# Agent Instructions For `specs/reminders`

## Purpose

This directory defines the reminders feature before and during implementation.

Files:

- `SPECIFICATION.md`: final north-star behavior and scope definition for humans
- `IMPLEMENTATION_PLAN.md`: execution tracker and versioned rollout plan

## Working Rules

- Read `SPECIFICATION.md` and `IMPLEMENTATION_PLAN.md` before making reminders-related changes.
- Treat `SPECIFICATION.md` as read-only unless the user explicitly asks to change the specification.
- Suggestions are welcome, but record them in `IMPLEMENTATION_PLAN.md` under `Suggestions And Open Questions` unless the user asks to update the spec itself.
- Follow the implementation plan in order unless the user explicitly reprioritizes work.
- Keep changes minimal, atomic, and reviewable.
- If you discover a mismatch between code and spec, do not silently rewrite the spec. Either:
  - fix the code to match the spec, or
  - record the issue in the plan and ask for a decision if the mismatch is material

## Status Maintenance

- There must be exactly one `IN PROGRESS` implementation step at a time in `IMPLEMENTATION_PLAN.md`.
- When you finish a step, mark it `DONE`.
- Immediately promote the next logical step to `IN PROGRESS`, unless work is blocked.
- If blocked, mark the blocked step `BLOCKED` and add a short reason in the plan.
- Do not leave the plan without a clear current step.

## Scope Control

- Do not widen V1 scope just because the schema can support future features.
- Keep future-proofing at the model seam level, not as unused runtime abstractions.
- Do not add Home Assistant YAML or helper-state persistence as part of this feature unless explicitly requested.
- All Home Assistant-facing reminder entities must remain runtime MQTT entities only.

## When Updating The Spec

Only update `SPECIFICATION.md` when one of the following is true:

- the user explicitly asks to change the feature behavior
- the current specification is internally inconsistent
- an approved decision changes version scope or acceptance criteria

When changing the spec:

- keep it as the final feature description, not the rollout plan
- keep observable behavior explicit
- update acceptance criteria and `Done When` conditions

## Preferred Workflow

1. Read the current `IN PROGRESS` step in `IMPLEMENTATION_PLAN.md`.
2. Implement only that step and any tiny prerequisite needed to complete it cleanly.
3. Run focused verification for the step.
4. Update the plan status.
5. Record any non-blocking ideas in `Suggestions And Open Questions`.

## Escalation Guidance

- If a requested implementation contradicts the specification, call it out clearly.
- If a better idea appears during implementation, propose it, but do not silently swap it in if it changes behavior or scope.
