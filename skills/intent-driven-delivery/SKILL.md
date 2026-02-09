---
name: intent-driven-delivery
description: >-
  Drive end-to-end software delivery from a user intent. Use when the user
  wants a full workflow for feature, bugfix, or refactor work: create a new
  branch with `featuer-`, `bugfix-`, or `refactor-`; clarify requirements
  through guided questions; write `docs/specs/mrd-*.md`; produce technical
  design in `docs/specs/design-*.md`; implement code; create test-case spec
  `docs/specs/qa-*.md`; implement tests; run/fix tests until passing; enforce
  test coverage at least 80%; generate a change summary for user review; and commit
  only after explicit confirmation.
---

# Intent Driven Delivery

## Overview

Execute a gated, intent-first delivery pipeline from branch creation to commit.
Keep artifacts auditable by writing MRD, design, QA spec, and change summary documents under `docs/specs/`.

## Workflow

1. Classify intent and create branch.
2. Clarify requirements and write MRD.
3. Produce technical design from MRD + repository context.
4. Implement code according to design.
5. Build QA spec from MRD.
6. Implement automated tests from QA spec.
7. Run tests, fix failures, and raise coverage to >=80%.
8. Generate change summary and wait for review confirmation.
9. Generate commit message and commit after explicit approval.

## Step 1: Classify Intent and Create Branch

Infer intent type from user statement:

- New capability or behavior -> `featuer-`
- Defect correction -> `bugfix-`
- Structure/performance/maintainability change without feature scope change -> `refactor-`

Create branch name:

- `<prefix><kebab-intent-slug>`
- Example: `featuer-portfolio-import`

If workspace has uncommitted changes, ask whether to continue on a new branch with current changes or shelve first.

## Step 2: Clarify Requirements and Write MRD

Use a question-driven discovery loop before implementation. Ask in small batches (5-8 concise questions) to avoid overload.

Cover at least:

- Business/user problem and expected value
- User profile and key scenario path
- In-scope and out-of-scope boundaries
- Functional requirements and edge cases
- Acceptance criteria (observable, testable)
- Constraints (platform, data, performance, security)

Write MRD to `docs/specs/mrd-<slug>.md` using `references/mrd-template.md`.

If user declines clarification, proceed with explicit assumptions and record them in the MRD.

## Step 3: Produce Technical Design

Read MRD and relevant code paths before writing design.

Use architecture-focused capabilities when available (for example: DDD/hexagonal, Go architecture skills). If not available, apply standard layered design.

Write design to `docs/specs/design-<slug>.md` using `references/design-template.md`.

Include:

- Architecture decisions and trade-offs
- Data model and interface changes
- API/handler/service/repository impact
- Migration/compatibility considerations
- Risks and mitigations

## Step 4: Implement Code

Implement only what is in MRD scope and design decisions.
Track deviations: if implementation requires design change, update `design-<slug>.md` before or during coding.

## Step 5: Produce QA Spec

Use testing-focused capabilities when available (for example: testing strategy, Go testing skills). If not available, apply risk-based test design.

Write QA spec to `docs/specs/qa-<slug>.md` using `references/qa-template.md`.

Cover:

- Positive/negative scenarios
- Boundary and error handling
- Regression scope
- Automation mapping (unit/integration/e2e)

## Step 6: Implement Tests from QA Spec

Create or update tests that directly map to QA scenarios.
Prefer fast, deterministic tests first (unit/integration) before broader suites.

## Step 7: Run, Fix, and Reach Coverage Gate

Run relevant tests for changed modules, then broader test suites when needed.

If failures occur:

1. Analyze root cause.
2. Fix code or tests.
3. Re-run until all required tests pass.

Measure coverage for impacted modules/project.

If coverage <80%:

1. Add high-value missing tests from QA gaps.
2. Re-run coverage.
3. Repeat until coverage >=80% or tooling cannot measure.

If tooling cannot produce reliable coverage, document limitation and provide best-effort evidence.

## Step 8: Generate Change Summary and Request Review

Write summary to `docs/specs/summary-<slug>.md` using `references/summary-template.md`.

Include:

- What changed
- Why it changed
- Test and coverage evidence
- Known limitations and follow-ups

Stop and ask for explicit review confirmation before commit.

## Step 9: Commit After Confirmation

After user confirmation:

- Generate concise commit message based on change scope.
- Prefer Chinese, action-oriented phrasing.
- Commit only tracked relevant files.

Do not commit if user has not explicitly approved.

## Artifact Rules

- Use a shared slug across files: `mrd-<slug>.md`, `design-<slug>.md`, `qa-<slug>.md`, `summary-<slug>.md`.
- Keep document updates synchronized when scope changes.
- Keep records auditable: decisions, assumptions, and test evidence must be written into docs.

## Resources (optional)

### references/

Load only as needed:

- `references/mrd-template.md`
- `references/design-template.md`
- `references/qa-template.md`
- `references/summary-template.md`
