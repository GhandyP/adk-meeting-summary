# adk-meeting-summary

Turn a supplied **text** meeting transcript into a validated, evidence-bearing summary using one
[Google ADK](https://google.github.io/adk-docs/) (Go, v2) agent.

The summary comes back as validated arrays: decisions, actions, open questions, participants and
evidence. An action whose owner or due date the transcript never stated is marked `unknown` and
reported for human confirmation instead of being guessed. Audio transcription, task creation and
calendar writes are deliberately out of scope.

## Requirements

- Go 1.26.6 or newer
- A Gemini API key in `GOOGLE_API_KEY`, only for an actual model run

## Quick start

```bash
git clone https://github.com/GhandyP/adk-meeting-summary
cd adk-meeting-summary

# No credential needed: validate the transcript and print the payload that would be sent.
go run . -dry-run -in testdata/transcript_overlap.json

# A real run: prints the summary JSON on stdout and the human checks on stderr.
export GOOGLE_API_KEY="your-key"
go run . -in testdata/transcript_ok.json
```

## Usage

```text
adk-meeting-summary [flags]

  -in string     transcript JSON file to read, or "-" for stdin (default "-")
  -out string    file for the summary JSON, or "-" for stdout (default "-")
  -model string  Gemini model name (default: ADK_MODEL, then gemini-flash-latest)
  -dry-run       validate the transcript and print the model payload without calling a model
  -pretty        indent the summary JSON (default true)
```

Exit codes: `0` success or help, `1` transcript/model/validation failure, `2` wrong command line.

## Transcript contract

```json
{
  "meetingID": "mtg-2026-10-13-sprint-review",
  "title": "Sprint review",
  "transcriptSegments": [
    { "speaker": "Ana", "start": "00:00:05", "end": "00:00:09", "text": "We ship the importer on Friday." }
  ]
}
```

`speaker`, `start` and `end` may be absent: a missing speaker is labelled `unknown` and a missing
timestamp stays empty, so the payload never implies a time or a person the transcript did not state.
Overlapping speakers are legitimate and keep their transcript order.

## Summary contract

```json
{
  "meetingID": "mtg-2026-10-13-sprint-review",
  "decisions": [{ "text": "Ship the importer on Friday.", "evidenceIDs": ["e1"] }],
  "actions": [
    { "text": "Write the migration guide.", "owner": "Bruno", "dueDate": "2026-10-20", "evidenceIDs": ["e2"] }
  ],
  "openQuestions": [{ "text": "Who owns the pricing page?", "evidenceIDs": ["e3"] }],
  "participants": [{ "name": "Ana" }, { "name": "Bruno" }],
  "evidence": [{ "id": "e1", "segmentIndex": 0, "quote": "We ship the importer on Friday." }]
}
```

## What is validated

Every model answer is checked against the transcript it claims to describe. A summary is rejected,
never silently repaired, when:

- the meeting id does not match the transcript;
- a decision, action or open question has no text or no evidence id;
- an evidence id is undefined, empty or duplicated;
- an evidence `segmentIndex` falls outside the transcript;
- an evidence quote is empty;
- a participant never appears in the transcript.

On top of the hard rules:

- every slice is rendered as an array, never `null`;
- a blank owner or due date becomes `unknown`;
- quotes that do not appear in the segment they cite are reported as warnings;
- actions whose owner or due date is `unknown` are listed on stderr for human confirmation before
  any minutes are published or any task is created.

## Layout

```text
main.go                      CLI: file or stdin in, summary JSON out
internal/meeting             the contract and its validation rules
internal/transcript          transcript loading, validation and prompt rendering
internal/summary             the ADK agent, its instruction, schema and parser
testdata/                    transcripts and golden summaries for the edge cases
odd/tasks/                   the ODD task document this version was built from
```

## Tests

```bash
go test ./... -count=1
```

Everything except one test runs offline with no credentials: the contract, the validation rules, the
transcript fixtures, the golden summaries, the agent instruction and output schema, the model answer
parser, and the CLI. `TestSummarizeLive` performs the real ADK round trip and is skipped unless
`GOOGLE_API_KEY` is set.

The fixtures cover the cases that break naive summarizers: overlapping speakers, missing timestamps,
a missing speaker, a meeting that decided nothing, an empty transcript, and actions with no owner.

## Boundaries

- Text only. No audio, no speech to text, no diarization.
- No tools are bound to the agent: drafting minutes cannot create tasks, write calendars or publish.
- No task or calendar adapter. Those belong outside the summarizer and stay approval-gated.

## License

MIT. See [LICENSE](LICENSE).
