# adk-meeting-summary — v1 typed meeting summary

Public Go project: one Google ADK (Go v2) agent turns a supplied **text** transcript into a
validated, evidence-bearing meeting summary. Audio ingestion and task/calendar writes stay out of v1.

Repo: `github.com/GhandyP/adk-meeting-summary` (public, MIT). Work lands on `main`, one work-unit commit per task.

## Specs

S1 — README "Missing elements" → Contract (verbatim):
`MeetingInput{meetingID, transcriptSegments[]}` -> `MeetingSummary{decisions[], actions[], openQuestions[], participants[], evidence[]}`.

S2 — README "Done when" (verbatim): "Decisions, actions, questions, and evidence are returned as validated arrays."

S3 — README "Done when" (verbatim): "Unknown owners/dates are marked unknown and never inferred as facts."

S4 — README "Missing elements" → Fixtures and validation (verbatim): "Cover overlapping speakers, missing timestamps, undecided items, and empty transcripts with golden summaries and owner/date checks."

S5 — README "Done when" (verbatim): "Audio ingestion and task/calendar writes are separate, reviewed, and approval-gated."

S6 — README "Goal" (verbatim): "Summarize a supplied meeting transcript into decisions, actions, and open questions, while making clear that audio transcription is a future boundary."

S7 — README "What exists now" (verbatim): "This is intentionally text-only: it does not accept audio, call recordings, Whisper output, calendar events, or task writes."

S8 — user (verbatim): "crea un repo publico para tener este repo en github con el git y el gh cli".

S9 — user decision (verbatim labels): repo/module "adk-meeting-summary"; push flow "Directo a main"; license "MIT".

S10 — user (verbatim): "vamos a implementar un odd con todos los elementos y estructuras necesarios para ese proyecto".

Non-goals (v1, from S5/S7): no audio/STT adapter, no calendar or task writes, no task-creation tools bound to the agent, no publishing of minutes.

## Tasks

| ID | Task | Specs | Route | Commit |
| --- | --- | --- | --- | --- |
| T1 | Bootstrap: isolated `git init` on `main`, MIT LICENSE, .gitignore, public GitHub repo, push | S8, S9, S10 | inline | pending |
| T2 | Own Go module `github.com/GhandyP/adk-meeting-summary`, ADK Go v2 dep, drop unbuildable `internal/adkhelper` | S8 | inline | pending |
| T3 | Contract types + validation with RED tests (`internal/meeting`) | S1, S2, S3 | inline | pending |
| T4 | Transcript loader + edge-case fixtures (`internal/transcript`, `testdata/`) | S4 | inline | pending |
| T5 | ADK agent with structured output, text-only, no side-effect tools (`internal/summary`) | S1, S2, S6, S7 | inline + 1 explorer | pending |
| T6 | CLI (`main.go`) + README/docs rewrite | S6, S7, S10 | inline | pending |
| T7 | CI workflow (build, vet, fmt, test) | S10 | inline | pending |
| T8 | Independent verification of the whole candidate | S2, S3, S4 | verifier | pending |

## Log

L1 — user (verbatim): "mira quiero hacer este mini proyecto con adk google y montarlo en mi github, dame el plan odd y explora lo que ya esta hecho"

L2 — user (verbatim): "me parece correcto asi que vamos a implementar un odd con todos los elementos y estructuras necesarios para ese proyecto y ademas y primero crea un repo publico para tener este repo en github con el git y el gh cli"

L3 — user decisions (verbatim labels): repo/module `adk-meeting-summary`; `Directo a main`; `MIT`; `Sí, autorizo el cache Go`.

L4 — exploration evidence (2026-10-13): this directory had no local `.git`; `git rev-parse --show-toplevel` resolved to `/home/ghandy` (no commits, no remotes), so no Git operation may run from that boundary. `GhandyP/adk-go-enterprise-cookbook` does not exist on GitHub, so the `internal/adkhelper` import in the prototype is unbuildable and v1 must be self-contained. `go1.26.4`, `GOMODCACHE=/home/ghandy/go/pkg/mod`, `gh` authenticated as `GhandyP` (`repo`, `workflow`). Module proxy confirms `google.golang.org/adk/v2` up to `v2.6.0`.

L5 — decision: v1 target module `google.golang.org/adk/v2` at `v2.6.0`; README's `v2.2.0` mention is updated at T6. Model stays `gemini-flash-latest` via `ADK_MODEL`, per README.
