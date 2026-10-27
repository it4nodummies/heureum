# Round 26 — Workflow QA state + real Kanban/Scrum — Design

**Status:** proposed (2026-10-27)
**Driver:** the direction is "zero-config, good defaults, driven by a real dev team's actual
usage" (ADR-0001 frozen-compat; `docs/superpowers/specs/2026-10-01-requisiti-harpa-design.md`).
A read-only tour of the reference team's live Jira (2026-10-27) measured what they actually use.

## Why (evidence from real usage)

The reference dev team runs ~7 software projects, ~2.4k issues, entirely on **stock Jira**
(standard Epic/Story/Task/Bug/Subtask, one bespoke field). The heavy, daily-used surface is:
boards, sprints, backlog, story points, Epic→child hierarchy — **already covered by Heureum
post-R25**. Two things they rely on are *not* yet real in Heureum:

1. **A verification/QA state in the workflow.** 5 of 7 projects run a 4-state flow with a QA
   column (their name: "TO BE TESTED") between *In Progress* and *Done*. Heureum's default
   workflow is only 3 states (TO DO → IN PROGRESS → DONE).
2. **A real behavioral difference between Kanban and Scrum.** The team splits ~4 Scrum
   (backlog + sprints; one board is 49 sprints deep) and ~3 Kanban (no sprints). In Heureum
   `board.Type` is stored (`scrum`/`kanban`, set at creation by `boardTypeFor`, R25) but the UI
   and API behave **identically** for both — the sprint/backlog experience is offered even to a
   Kanban board.

Out of scope (confirmed with the user, consistent with frozen-compat and zero real usage):
service desk / JSM, Components, Advanced Roadmaps / multi-level hierarchy.

## Goals

- A newly created **software** project (scrum or kanban template) gets a default workflow with a
  **QA/verification state**: `TO DO → IN PROGRESS → TO BE TESTED → DONE` (the name "TO BE TESTED"
  matches the reference team's convention). A **business** project keeps the lean 3-state flow. The
  extra state is ordinary workflow data — removable/renamable through the existing per-project
  Workflow editor.
- **Kanban and Scrum boards behave differently:**
  - **Scrum**: Backlog with **sprints** (create/start/complete, drag backlog↔sprint) — current behavior.
  - **Kanban**: **no sprints**, but a **flat backlog** — a single rank-ordered list of the board's
    to-do issues (prioritize by drag-to-reorder; "send to board" transitions an issue onto the
    board). No sprint sections. Creating a sprint against a Kanban board is rejected by the API, not
    just hidden.

## Non-goals

- No workflow *template* system / workflow schemes / per-issue-type workflows (frozen-compat;
  the single software-vs-business split via a flag is enough for zero-config).
- No WIP limits.
- The Kanban backlog is a **prioritization list of to-do issues** (reusing issue rank + a status
  transition to "send to board") — **no new schema** and no hiding of the to-do column from the
  board. A fuller Jira-style "backlog column mapping" (statuses assigned to the backlog, hidden from
  the board) is a later refinement.
- No migration of existing Heureum projects' workflows — R26 changes only the **default created
  at project creation**. Existing projects keep their current workflow.
- No board-type switching after creation.
- No custom status categories — the review state reuses the existing `in-progress` category.

## Design

### Part A — software default workflow gains a "TO BE TESTED" state

- `workflow.Service.CreateDefaultWorkflow` gains a boolean parameter `includeReview`.
  - `true` → statuses TO DO (todo) → IN PROGRESS (in-progress) → TO BE TESTED (in-progress) → DONE
    (done); transitions: *Start Progress* (TODO→INPROG), *Stop Progress* (INPROG→TODO), *Ready to
    Test* (INPROG→TOBETESTED), *Back to In Progress* (TOBETESTED→INPROG), *Done* (TOBETESTED→DONE,
    `setResolution=true`).
  - `false` → unchanged 3-state workflow.
- `project_handler.Create` passes `includeReview = p.Type != project.TypeBusiness`.
- The board created in the same scaffold derives its columns from the workflow statuses (1:1
  fallback when no board columns are persisted), so a new software project's board shows **4
  columns** automatically — no extra board seeding.
- `TO BE TESTED` category is `in-progress` (active work, not done); distinct color.

### Part B — Kanban vs Scrum behavior

- **API**: sprint creation is rejected when the board's `Type != "scrum"`. The guard lives in the
  agile sprint handler (`agile_sprint_handler.Create`), where the board `b` is already resolved —
  return HTTP 400 before `CreateFull`. Kanban boards can never hold sprints.
- **Frontend**:
  - The board detail (`boardsApi.get`) already returns `type`; the board + backlog pages read it.
  - **Scrum board**: unchanged — Backlog link visible, backlog page shows sprints + backlog DnD.
  - **Kanban board**: the backlog page renders a **flat backlog** instead of sprint sections — a
    single rank-ordered list of the board's **to-do-category** issues, drag-to-reorder (existing
    `PUT /issue/rank`), with a **"Send to board"** action that transitions an issue to the first
    in-progress status. No sprint controls anywhere for a kanban board; creating a sprint is blocked
    by the API above.
  - `CreateProjectModal` already picks scrum/kanban/business; `boardTypeFor` already sets the board
    type at creation (R25). No creation-flow change needed.

## Review focus (edge cases the tasks must cover)

- A **business** project still gets exactly the 3-state workflow (no review state).
- A new **scrum/kanban** project's board renders **4 columns** and the *Ready to Test* / *Done*
  transitions work end-to-end (create issue → move across all four columns).
- A **kanban** board's backlog shows the flat to-do list with "Send to board" and **no sprint UI**;
  "Send to board" transitions an issue to the first in-progress status.
- Visiting a **kanban** board's `/backlog` route directly shows **no sprint UI**.
- **Sprint create on a kanban board** is rejected by the API (not merely hidden in the UI).
- **Existing (pre-R26) projects** are unaffected: their stored workflow and board behavior don't change.

## Conformance

The agile/board/sprint endpoints stay Jira-conformant — keep/extend the contract tests in
`internal/contract`. The review state is a normal workflow status (no new DTO fields). No change to
`docs/contracts/gap-report.md` is expected (no new routes); regenerate to confirm no drift.
