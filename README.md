# 04 — Transcribe and summarize meetings

## Goal

Summarize a supplied meeting transcript into decisions, actions, and open questions, while making clear that audio transcription is a future boundary.

## Run the current prototype

From this case directory:

```bash
export GOOGLE_API_KEY="your-key"
go run .
```

`internal/adkhelper.Run` uses ADK Go v2.2.0, Gemini, and `gemini-flash-latest`; set `ADK_MODEL` to override the model.

Sample prompt:

```text
Summarize these meeting notes, including decisions and owners.
```

## What exists now

- `main.go (exists)` runs one focused Gemini ADK agent over text supplied in the prompt.
- The output is untyped text; speaker, timestamp, decision, owner, and due-date fields are not validated.
- This is intentionally text-only: it does not accept audio, call recordings, Whisper output, calendar events, or task writes.

## Missing elements

- **Contract:** Define `MeetingInput{meetingID, transcriptSegments[]}` -> `MeetingSummary{decisions[], actions[], openQuestions[], participants[], evidence[]}`.
- **ADK workflow:** Add a transcript-normalizer agent, a summary agent, schemas, and a meeting session; a future `create_task` tool must remain disabled during drafting.
- **Domain boundary:** Add a Whisper/STT adapter, speaker/timestamp mapper, and optional calendar/task adapters outside the summarizer.
- **Fixtures and validation:** Cover overlapping speakers, missing timestamps, undecided items, and empty transcripts with golden summaries and owner/date checks.
- **Safety:** Require a human to confirm inferred owners and deadlines before publishing minutes or creating tasks.

## Target structure

```text
04-meeting-transcription-summary/
├── main.go (exists)
├── README.md (exists)
├── workflow.go (next)
├── schema.go (next)
├── adapters/stt.go (next)
├── testdata/transcript.json (next)
└── main_test.go (next)
```

## Implementation order

1. Define timestamped transcript segments and the typed summary contract.
2. Normalize speakers and add summary schemas before connecting audio.
3. Add the STT adapter and a session keyed by meeting ID.
4. Test decisions, unknown owners, overlaps, and missing timestamps.
5. Add approval-gated calendar/task tools for confirmed actions only.

## Done when

- [ ] Decisions, actions, questions, and evidence are returned as validated arrays.
- [ ] Unknown owners/dates are marked unknown and never inferred as facts.
- [ ] Transcript fixtures and tests cover speaker and timestamp edge cases.
- [ ] Audio ingestion and task/calendar writes are separate, reviewed, and approval-gated.

## Next improvement

Make the current transcript path typed and evidence-bearing before adding Whisper or calendar integration.
