package transcript_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GhandyP/adk-meeting-summary/internal/meeting"
	"github.com/GhandyP/adk-meeting-summary/internal/transcript"
)

const fixtures = "../../testdata"

func mustLoad(t *testing.T, name string) meeting.MeetingInput {
	t.Helper()
	in, err := transcript.Load(filepath.Join(fixtures, name), nil)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return in
}

func mustLoadSummary(t *testing.T, name string) meeting.MeetingSummary {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtures, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var summary meeting.MeetingSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return summary
}

// S4: the loader validates and normalizes a real transcript, keeping stated
// timestamps and recording the missing speaker as unknown.
func TestLoadValidatesAndNormalizes(t *testing.T) {
	got := mustLoad(t, "transcript_ok.json")

	if got.MeetingID != "mtg-2026-10-13-sprint-review" {
		t.Errorf("meetingID = %q", got.MeetingID)
	}
	if len(got.Segments) != 4 {
		t.Fatalf("segments = %d, want 4", len(got.Segments))
	}
	if last := got.Segments[3]; last.Speaker != meeting.Unknown {
		t.Errorf("missing speaker = %q, want %q", last.Speaker, meeting.Unknown)
	}
	if got.Segments[0].Start != "00:00:05" {
		t.Errorf("stated timestamp = %q, want it preserved", got.Segments[0].Start)
	}
}

func TestLoadReadsStandardInputWhenPathIsDash(t *testing.T) {
	got, err := transcript.Load("-", strings.NewReader(
		`{"meetingID":"mtg-stdin","transcriptSegments":[{"speaker":"Ana","text":"Single line."}]}`))
	if err != nil {
		t.Fatalf("load from stdin: %v", err)
	}
	if got.MeetingID != "mtg-stdin" || len(got.Segments) != 1 {
		t.Fatalf("unexpected input: %+v", got)
	}
}

// S4: an empty transcript is rejected with the named contract error.
func TestLoadRejectsEmptyTranscript(t *testing.T) {
	_, err := transcript.Load(filepath.Join(fixtures, "transcript_empty.json"), nil)
	if !errors.Is(err, meeting.ErrNoSegments) {
		t.Fatalf("error = %v, want %v", err, meeting.ErrNoSegments)
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	_, err := transcript.Decode(strings.NewReader("{ not json"))
	if err == nil {
		t.Fatal("malformed JSON must fail")
	}
	if errors.Is(err, meeting.ErrNoSegments) {
		t.Errorf("malformed JSON must not be reported as a contract error: %v", err)
	}
}

// S4: missing timestamps are labelled instead of invented, so the prompt payload
// never implies a time the transcript did not state.
func TestRenderPlainTextKeepsTimestampsAndMarksUnknownOnes(t *testing.T) {
	in := mustLoad(t, "transcript_overlap.json")

	want := strings.Join([]string{
		"[00:01:00] Ana: The deploy window moves to Thursday.",
		"[00:01:10] Bruno: I can prepare the rollback plan.",
		"[unknown] Ana: Nobody volunteered for the customer note.",
		"[unknown] unknown: Second recorder picked up an unidentified voice.",
	}, "\n") + "\n"

	if got := transcript.RenderPlainText(in); got != want {
		t.Errorf("RenderPlainText =\n%q\nwant\n%q", got, want)
	}
}

// S4: the golden summaries stay valid against their transcript, with owner and
// due-date checks for every action the transcript left open.
func TestGoldenSummariesValidateAgainstTheirTranscripts(t *testing.T) {
	tests := []struct {
		name                 string
		transcript           string
		summary              string
		wantConfirmations    int
		confirmationContains string
	}{
		{
			name:                 "complete meeting",
			transcript:           "transcript_ok.json",
			summary:              "summary_golden_ok.json",
			wantConfirmations:    2,
			confirmationContains: "Find an owner for the pricing page.",
		},
		{
			name:                 "overlapping speakers",
			transcript:           "transcript_overlap.json",
			summary:              "summary_golden_overlap.json",
			wantConfirmations:    1,
			confirmationContains: "Volunteer for the customer note.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := mustLoad(t, tt.transcript)
			summary := meeting.NormalizeSummary(mustLoadSummary(t, tt.summary))

			if err := meeting.ValidateSummary(in, summary); err != nil {
				t.Fatalf("golden summary rejected: %v", err)
			}
			if mismatches := meeting.AuditEvidence(in, summary); len(mismatches) != 0 {
				t.Errorf("golden quotes do not match the transcript: %v", mismatches)
			}

			needs := meeting.RequiresConfirmation(summary)
			if len(needs) != tt.wantConfirmations {
				t.Fatalf("RequiresConfirmation = %v, want %d entries", needs, tt.wantConfirmations)
			}
			found := false
			for _, need := range needs {
				if strings.Contains(need, tt.confirmationContains) {
					found = true
				}
			}
			if !found {
				t.Errorf("confirmation list %v does not name %q", needs, tt.confirmationContains)
			}
		})
	}
}

// S4: a meeting that decided nothing still produces arrays, not null, and asks
// for no confirmation.
func TestUndecidedMeetingKeepsEmptyArrays(t *testing.T) {
	in := mustLoad(t, "transcript_undecided.json")

	summary := meeting.NormalizeSummary(meeting.MeetingSummary{
		MeetingID: in.MeetingID,
		OpenQuestions: []meeting.OpenQuestion{
			{Text: "Which tooling budget do we approve?", EvidenceIDs: []string{"e1"}},
		},
		Participants: []meeting.Participant{{Name: "Ana"}, {Name: "Bruno"}},
		Evidence: []meeting.Evidence{
			{ID: "e1", SegmentIndex: 0, Quote: "We still have not decided on the tooling budget."},
		},
	})

	if err := meeting.ValidateSummary(in, summary); err != nil {
		t.Fatalf("undecided summary rejected: %v", err)
	}
	if needs := meeting.RequiresConfirmation(summary); len(needs) != 0 {
		t.Errorf("undecided summary must need no confirmation, got %v", needs)
	}

	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, empty := range []string{`"decisions":[]`, `"actions":[]`} {
		if !strings.Contains(string(raw), empty) {
			t.Errorf("summary must render %s, got %s", empty, raw)
		}
	}
}
