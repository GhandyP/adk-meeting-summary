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
| T1 | Bootstrap: isolated `git init` on `main`, MIT LICENSE, .gitignore, public GitHub repo, push | S8, S9, S10 | inline | `cd370b5` |
| T2 | Own Go module `github.com/GhandyP/adk-meeting-summary`, ADK Go v2 dep, drop unbuildable `internal/adkhelper` | S8 | inline | `f55da0d` |
| T3 | Contract types + validation with RED tests (`internal/meeting`) | S1, S2, S3 | inline | `87b9aa3` |
| T4 | Transcript loader + edge-case fixtures (`internal/transcript`, `testdata/`) | S4 | inline | `4adf3ff` |
| T5 | ADK agent with structured output, text-only, no side-effect tools (`internal/summary`) | S1, S2, S6, S7 | inline + 1 explorer | `75f1583` |
| T6 | CLI (`main.go`) + README/docs rewrite | S6, S7, S10 | inline | `e7267fc` |
| T7 | CI workflow (build, vet, fmt, test) | S10 | inline | `0e92524` |
| T8 | Independent verification of the whole candidate | S2, S3, S4 | verifier (unavailable) + CI run | candidate `0e92524` |

## Log

L1 — user (verbatim): "mira quiero hacer este mini proyecto con adk google y montarlo en mi github, dame el plan odd y explora lo que ya esta hecho"

L2 — user (verbatim): "me parece correcto asi que vamos a implementar un odd con todos los elementos y estructuras necesarios para ese proyecto y ademas y primero crea un repo publico para tener este repo en github con el git y el gh cli"

L3 — user decisions (verbatim labels): repo/module `adk-meeting-summary`; `Directo a main`; `MIT`; `Sí, autorizo el cache Go`.

L4 — exploration evidence (2026-10-13): this directory had no local `.git`; `git rev-parse --show-toplevel` resolved to `/home/ghandy` (no commits, no remotes), so no Git operation may run from that boundary. `GhandyP/adk-go-enterprise-cookbook` does not exist on GitHub, so the `internal/adkhelper` import in the prototype is unbuildable and v1 must be self-contained. `go1.26.4`, `GOMODCACHE=/home/ghandy/go/pkg/mod`, `gh` authenticated as `GhandyP` (`repo`, `workflow`). Module proxy confirms `google.golang.org/adk/v2` up to `v2.6.0`.

L5 — decision: v1 target module `google.golang.org/adk/v2` at `v2.6.0`; README's `v2.2.0` mention is updated at T6. Model stays `gemini-flash-latest` via `ADK_MODEL`, per README.

L6 — evidence (T1): local repo root is `/home/ghandy/Documents/04-meeting-transcription-summary`, bootstrap commit `cd370b5` on `main`. Public repo created with `gh repo create GhandyP/adk-meeting-summary --public --source=. --remote=origin --push`; `gh repo view` reports `PUBLIC`, `main`; `main` tracks `origin/main`.

L7 — evidence (T2, T3): commits `f55da0d` (ADK v2.6.0, go directive raised to 1.26.6 by the module) and `87b9aa3` (contract + validation). T3 RED was observed as `no non-test Go files in internal/meeting [build failed]`, exit 1; GREEN as `ok github.com/GhandyP/adk-meeting-summary/internal/meeting` with 21 test functions. Both pushed to `origin/main`.

L8 — incident: the first delegation to `gentle-ai-explore` failed with `Session Git identity changed; cannot register a worktree`, because creating this repository mid-session invalidated the session's Git identity (the session started with `/home/ghandy` as its Git root). Fallback: the ADK Go v2 API research ran inline against the module cache, bounded, with no second delegation attempt.

L9 — evidence (ADK Go v2.6.0 API, read from the module cache): `model/gemini/gemini.go:49` `NewModel(ctx, name, *genai.ClientConfig)`; `agent/llmagent/llmagent.go:40` `New(Config)`; `agent/llmagent/llmagent.go:316` `OutputSchema *genai.Schema`; `runner/runner.go:212` `NewInMemory(appName, agent)` with `AutoCreateSession`; `runner/runner.go:538` `Run(ctx, userID, sessionID, *genai.Content, agent.RunConfig, ...) iter.Seq2[*session.Event, error]`; `session/session.go` `Event.Output any` (the parsed structured value) and `Event.IsFinalResponse()`. One session is created per run so one meeting's answer never reaches the next prompt.

L10 — deviation from a wrong test expectation I wrote: the first `validSummary()` fixture gave an action a stated owner but `unknown` due date, which contradicted the same file's expectation that only one action needs confirmation. The test was corrected (fully stated owner and date) before any GREEN was claimed.

L11 — evidence and incident (T4): commit `4adf3ff`, pushed; `go test` reported `ok internal/meeting` and `ok internal/transcript` (2.065s) and the golden-summary, overlap, missing-timestamp, undecided and empty-transcript cases passed. Two defects surfaced in the same job and are recorded rather than hidden: (a) the command piped `go test ./internal/...` into `tail`, so the pipeline exit status came from `tail` and the `FAIL` of `internal/summary` (missing `go.sum` entries for ADK's transitive modules at that moment) did not stop the commit; T4's own packages were green, and every later verification command uses `set -o pipefail`. (b) `gofmt -l` flagged `internal/summary/summary.go`; it was reformatted and `gofmt -l .` is now empty.

L12 — constraint: `google.golang.org/adk/v2 v2.6.0` requires Go >= 1.26.6, so the module raises the documented requirement above the README's original wording. The CI workflow installs the toolchain from the `go` directive.

L13 — environment observation: while jobs ran, another session on this machine executed unrelated `go test ./internal/generator ./internal/cli` processes, which competed for CPU and slowed the first build of the ADK dependency tree. Not a project defect.

L14 — incident and correction (dependency resolution): a background `GOFLAGS=-mod=mod go build ./internal/...` (job-6) produced no output for roughly fifteen minutes, which was read as a stall. Diagnosis before acting: disk healthy (113G free, 12% inodes), `proxy.golang.org` and `sum.golang.org` both answering HTTP 200, and `/proc` showed the process sleeping in `futex_do_wait` with 19s of CPU over 8m32. I stopped that job and re-ran the resolution as `go mod tidy` with unbuffered output; it then showed live `go: downloading go.opentelemetry.io/otel/sdk ...` lines and established HTTPS connections, so the earlier job was progressing slowly under CPU contention from the other session, not deadlocked. Cost: one duplicated build cycle; no repository damage, because Go downloads are atomic. Lesson recorded: never pipe a Go build or test into `tail` when the exit status matters, and watch progress through the job log instead of buffered pipes.

L15 — verification evidence (before T5-T7 were committed): `go mod tidy` resolved `go.sum` to 91 lines with direct requirements `google.golang.org/adk/v2 v2.6.0` and `google.golang.org/genai v1.74.0`. Then, with `set -o pipefail`: `gofmt -l .` printed nothing, `go vet ./...` exited 0, `go build ./...` exited 0, `go test ./... -count=1` exited 0, and all four packages reported `ok` (root, `internal/meeting`, `internal/summary`, `internal/transcript`) with 45 test functions in total. `internal/summary` reported 6 PASS and `TestSummarizeLive` SKIP because `GOOGLE_API_KEY` is not set in this environment: the live ADK round trip is therefore NOT verified here and is not claimed.

L16 — evidence: commits `75f1583` (T5), `e7267fc` (T6) and `0e92524` (T7) pushed; `origin/main` = `0e92524`.

L17 — design decision (T5, recorded because it is not provable without a live key): ADK documents that the value parsed against `OutputSchema` is stored on the terminal event, so `Summarize` reads `event.Output` wherever it appears and keeps the last one, and uses the text of the final response only as a fallback. This keeps the summary from disappearing if finality is marked differently than the wrapper comment suggests.

L18 — independent execution evidence (T7): GitHub Actions run `38422222255` on `main` at `0e925247eff0ee85e3d10f37da2e2c02160de603` finished `conclusion=success status=completed`, with every step green: `actions/checkout@v7`, `actions/setup-go@v7`, `gofmt`, `go vet`, `go build`, `go test`, 1m4s total. This is a clean runner that did not share this machine's cache, so it independently confirms the format, vet, build and test claims of the whole tree. The only annotation is GitHub's notice that the `ubuntu-latest` label migrates to Ubuntu 26 after 2026-10-19, which needs no change here.

L19 — unavailable verification (T8): the independent verifier could not run. `gentle-ai-verify` failed with `could not register launched worktree: Session Git identity changed; cannot register a worktree`, the same host-level incident that blocked `gentle-ai-explore` at L8: creating this repository mid-session invalidated the session's registered Git identity, and no native `Agent` role exists in this runtime as a fallback. I did NOT substitute a self-review for an independent verdict: the CI run at L18 is reported as independent execution evidence for the mechanical checks, and no independent reviewer ever read this candidate for correctness. That gap stays open and is reported to the user.

L20 — unavailable tooling: Engram (persistent memory) stopped responding during this session (`mem_save`, `mem_search` and `mem_doctor` all timed out; the socket accepts connections but produces no response). Nothing was retried blindly, so a previous write may or may not have landed, and no memory claim is made in this document or to the user.

L21 — open item (not a defect): `TestSummarizeLive` skips because `GOOGLE_API_KEY` is absent in this environment. The ADK round trip against the real Gemini API — the wiring of `llmagent`, `runner` and `event.Output` — is therefore compiled, statically checked and structurally tested, but never executed end to end here. Running it needs a real key and is the one remaining proof.
