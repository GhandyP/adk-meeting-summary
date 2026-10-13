package meeting_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/GhandyP/adk-meeting-summary/internal/meeting"
)

func input() meeting.MeetingInput {
	return meeting.MeetingInput{
		MeetingID: "mtg-2026-10-13",
		Title:     "Sprint review",
		Segments: []meeting.TranscriptSegment{
			{Speaker: "ana", Start: "00:00:05", Text: "We ship the importer on Friday."},
			{Speaker: "Bruno", Start: "00:00:12", Text: "I will write the migration guide."},
			{Speaker: "ana", Text: "The pricing page still has no owner."},
		},
	}
}

func validSummary() meeting.MeetingSummary {
	return meeting.MeetingSummary{
		MeetingID: "mtg-2026-10-13",
		Decisions: []meeting.Decision{
			{Text: "Ship the importer on Friday.", EvidenceIDs: []string{"e1"}},
		},
		Actions: []meeting.Action{
			{Text: "Write the migration guide.", Owner: "Bruno", DueDate: "2026-10-20", EvidenceIDs: []string{"e2"}},
		},
		OpenQuestions: []meeting.OpenQuestion{
			{Text: "Who owns the pricing page?", EvidenceIDs: []string{"e3"}},
		},
		Participants: []meeting.Participant{
			{Name: "ana"}, {Name: "Bruno"},
		},
		Evidence: []meeting.Evidence{
			{ID: "e1", SegmentIndex: 0, Quote: "We ship the importer on Friday."},
			{ID: "e2", SegmentIndex: 1, Quote: "I will write the migration guide."},
			{ID: "e3", SegmentIndex: 2, Quote: "The pricing page still has no owner."},
		},
	}
}

// S2: decisions, actions, questions, participants and evidence are arrays, never null.
func TestSummaryArraysAreNeverNil(t *testing.T) {
	got := meeting.NormalizeSummary(meeting.MeetingSummary{MeetingID: "mtg-2026-10-13"})

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal normalized summary: %v", err)
	}
	for _, key := range []string{"decisions", "actions", "openQuestions", "participants", "evidence"} {
		if !strings.Contains(string(raw), `"`+key+`":[]`) {
			t.Errorf("normalized summary must render %s as an empty array, got %s", key, raw)
		}
	}
}

// S1: the JSON contract keeps the documented field names.
func TestJSONContractFieldNames(t *testing.T) {
	inRaw, err := json.Marshal(input())
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	inKeys := jsonKeys(t, inRaw)
	wantIn := []string{"meetingID", "title", "transcriptSegments"}
	sort.Strings(wantIn)
	if !reflect.DeepEqual(inKeys, wantIn) {
		t.Errorf("MeetingInput keys = %v, want %v", inKeys, wantIn)
	}

	for _, key := range []string{"speaker", "start", "text"} {
		if !strings.Contains(string(inRaw), `"`+key+`"`) {
			t.Errorf("transcript segment contract is missing %q: %s", key, inRaw)
		}
	}

	summaryRaw, err := json.Marshal(meeting.NormalizeSummary(validSummary()))
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	summaryKeys := jsonKeys(t, summaryRaw)
	wantSummary := []string{"meetingID", "decisions", "actions", "openQuestions", "participants", "evidence"}
	sort.Strings(wantSummary)
	if !reflect.DeepEqual(summaryKeys, wantSummary) {
		t.Errorf("MeetingSummary keys = %v, want %v", summaryKeys, wantSummary)
	}
	for _, key := range []string{"text", "evidenceIDs", "owner", "dueDate", "id", "segmentIndex", "quote", "name"} {
		if !strings.Contains(string(summaryRaw), `"`+key+`"`) {
			t.Errorf("summary contract is missing %q: %s", key, summaryRaw)
		}
	}
}

func jsonKeys(t *testing.T, raw []byte) []string {
	t.Helper()
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	keys := make([]string, 0, len(decoded))
	for k := range decoded {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// S3: a missing owner or due date is recorded as unknown, never guessed.
func TestNormalizeSummaryMarksUnknownOwnerAndDueDate(t *testing.T) {
	got := meeting.NormalizeSummary(meeting.MeetingSummary{
		MeetingID: "mtg-2026-10-13",
		Actions: []meeting.Action{
			{Text: "Prepare the demo.", Owner: "   ", DueDate: "", EvidenceIDs: []string{"e1"}},
			{Text: "Book the room.", Owner: "Bruno", DueDate: "2026-10-20", EvidenceIDs: []string{"e1"}},
		},
	})

	if got.Actions[0].Owner != meeting.Unknown {
		t.Errorf("blank owner = %q, want %q", got.Actions[0].Owner, meeting.Unknown)
	}
	if got.Actions[0].DueDate != meeting.Unknown {
		t.Errorf("blank due date = %q, want %q", got.Actions[0].DueDate, meeting.Unknown)
	}
	if got.Actions[1].Owner != "Bruno" || got.Actions[1].DueDate != "2026-10-20" {
		t.Errorf("stated owner/date were rewritten: %+v", got.Actions[1])
	}
}

// S3: an action with an unknown owner stays in the summary and is flagged for a human.
func TestUnknownOwnerActionSurvivesAndNeedsConfirmation(t *testing.T) {
	summary := validSummary()
	summary.Actions = append(summary.Actions, meeting.Action{
		Text: "Decide the pricing owner.", Owner: meeting.Unknown, DueDate: meeting.Unknown, EvidenceIDs: []string{"e3"},
	})

	normalized := meeting.NormalizeSummary(summary)
	if err := meeting.ValidateSummary(input(), normalized); err != nil {
		t.Fatalf("summary with an unknown owner must stay valid, got %v", err)
	}
	if len(normalized.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(normalized.Actions))
	}

	needs := meeting.RequiresConfirmation(normalized)
	if len(needs) != 1 {
		t.Fatalf("RequiresConfirmation = %v, want exactly the unknown-owner action", needs)
	}
	if !strings.Contains(needs[0], "Decide the pricing owner.") {
		t.Errorf("confirmation line = %q, want it to name the action", needs[0])
	}

	flagged := meeting.RequiresConfirmation(meeting.NormalizeSummary(validSummary()))
	if len(flagged) != 0 {
		t.Errorf("fully stated action must not require confirmation, got %v", flagged)
	}
}

// S4: empty transcripts and empty meeting ids are rejected with a named error.
func TestValidateInputRejectsEmptyTranscript(t *testing.T) {
	tests := []struct {
		name string
		in   meeting.MeetingInput
		want error
	}{
		{
			name: "no meeting id",
			in:   meeting.MeetingInput{Segments: []meeting.TranscriptSegment{{Speaker: "ana", Text: "hola"}}},
			want: meeting.ErrEmptyMeetingID,
		},
		{
			name: "no segments",
			in:   meeting.MeetingInput{MeetingID: "mtg-1"},
			want: meeting.ErrNoSegments,
		},
		{
			name: "segment without text",
			in:   meeting.MeetingInput{MeetingID: "mtg-1", Segments: []meeting.TranscriptSegment{{Speaker: "ana", Text: "  "}}},
			want: meeting.ErrEmptySegmentText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := meeting.ValidateInput(tt.in)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateInput error = %v, want %v", err, tt.want)
			}
		})
	}

	if err := meeting.ValidateInput(input()); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
}

// S4: missing timestamps and overlapping speakers are legitimate, not errors.
func TestInputAllowsMissingTimestampsAndOverlappingSpeakers(t *testing.T) {
	in := meeting.MeetingInput{
		MeetingID: "mtg-overlap",
		Segments: []meeting.TranscriptSegment{
			{Speaker: "ana", Start: "00:01:00", End: "00:01:30", Text: "First half."},
			{Speaker: "Bruno", Start: "00:01:10", End: "00:01:40", Text: "Speaking over ana."},
			{Speaker: "", Text: "Sentence without a speaker or timestamps."},
		},
	}
	if err := meeting.ValidateInput(in); err != nil {
		t.Fatalf("overlapping or untimed segments must stay valid, got %v", err)
	}

	normalized := meeting.NormalizeInput(in)
	if normalized.Segments[2].Speaker != meeting.Unknown {
		t.Errorf("missing speaker = %q, want %q", normalized.Segments[2].Speaker, meeting.Unknown)
	}
	if normalized.Segments[2].Start != "" || normalized.Segments[2].End != "" {
		t.Errorf("missing timestamps must stay empty, got start=%q end=%q",
			normalized.Segments[2].Start, normalized.Segments[2].End)
	}
	if len(normalized.Segments) != 3 {
		t.Errorf("normalization dropped segments: %d", len(normalized.Segments))
	}
}

// S2: summary items must carry text and at least one existing evidence id.
func TestValidateSummaryRejectsUnbackedItems(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*meeting.MeetingSummary)
		want   error
	}{
		{
			name:   "decision without text",
			mutate: func(s *meeting.MeetingSummary) { s.Decisions[0].Text = " " },
			want:   meeting.ErrItemWithoutText,
		},
		{
			name:   "action without evidence",
			mutate: func(s *meeting.MeetingSummary) { s.Actions[0].EvidenceIDs = nil },
			want:   meeting.ErrItemWithoutEvidence,
		},
		{
			name:   "question with dangling evidence id",
			mutate: func(s *meeting.MeetingSummary) { s.OpenQuestions[0].EvidenceIDs = []string{"e9"} },
			want:   meeting.ErrDanglingEvidenceID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := validSummary()
			tt.mutate(&summary)
			err := meeting.ValidateSummary(input(), summary)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSummary error = %v, want %v", err, tt.want)
			}
		})
	}

	if err := meeting.ValidateSummary(input(), validSummary()); err != nil {
		t.Errorf("valid summary rejected: %v", err)
	}
}

// S2: evidence must point at a real transcript segment and quote it.
func TestValidateSummaryChecksEvidenceAgainstTranscript(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*meeting.MeetingSummary)
		want   error
	}{
		{
			name:   "index out of range",
			mutate: func(s *meeting.MeetingSummary) { s.Evidence[0].SegmentIndex = 7 },
			want:   meeting.ErrEvidenceIndexOutOfRange,
		},
		{
			name:   "empty quote",
			mutate: func(s *meeting.MeetingSummary) { s.Evidence[0].Quote = "" },
			want:   meeting.ErrEmptyEvidenceQuote,
		},
		{
			name:   "evidence without id",
			mutate: func(s *meeting.MeetingSummary) { s.Evidence[0].ID = " " },
			want:   meeting.ErrEvidenceIDRequired,
		},
		{
			name:   "duplicated evidence id",
			mutate: func(s *meeting.MeetingSummary) { s.Evidence[1].ID = s.Evidence[0].ID },
			want:   meeting.ErrDuplicateEvidenceID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := validSummary()
			tt.mutate(&summary)
			err := meeting.ValidateSummary(input(), summary)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSummary error = %v, want %v", err, tt.want)
			}
		})
	}
}

// S2: a summary of another meeting, and invented participants, are rejected.
func TestValidateSummaryRejectsSummaryOfAnotherMeeting(t *testing.T) {
	summary := validSummary()
	summary.MeetingID = "mtg-other"
	err := meeting.ValidateSummary(input(), summary)
	if !errors.Is(err, meeting.ErrSummaryMeetingIDMismatch) {
		t.Fatalf("error = %v, want %v", err, meeting.ErrSummaryMeetingIDMismatch)
	}
}

func TestValidateSummaryRejectsUnknownParticipants(t *testing.T) {
	summary := validSummary()
	summary.Participants = append(summary.Participants, meeting.Participant{Name: "Carla"})
	err := meeting.ValidateSummary(input(), summary)
	if !errors.Is(err, meeting.ErrUnknownParticipant) {
		t.Fatalf("error = %v, want %v", err, meeting.ErrUnknownParticipant)
	}

	unnamed := validSummary()
	unnamed.Participants = []meeting.Participant{{Name: " ", Unknown: true}}
	if err := meeting.ValidateSummary(input(), unnamed); !errors.Is(err, meeting.ErrEmptyParticipantName) {
		t.Fatalf("error = %v, want %v", err, meeting.ErrEmptyParticipantName)
	}

	unknownFloor := validSummary()
	unknownFloor.Participants = []meeting.Participant{{Name: meeting.Unknown, Unknown: true}}
	if err := meeting.ValidateSummary(input(), unknownFloor); err != nil {
		t.Errorf("an explicit unknown participant must stay valid, got %v", err)
	}
}

// S2/S3 taken together: quoted evidence that does not come from the transcript is reported.
func TestAuditEvidenceReportsQuotesThatAreNotInTheTranscript(t *testing.T) {
	summary := validSummary()
	summary.Evidence[1].Quote = "I will write the migration guide and the release notes."

	mismatches := meeting.AuditEvidence(input(), summary)
	if len(mismatches) != 1 {
		t.Fatalf("AuditEvidence = %v, want one mismatch", mismatches)
	}
	if !strings.Contains(mismatches[0], "e2") {
		t.Errorf("mismatch = %q, want it to name the evidence id", mismatches[0])
	}

	clean := meeting.AuditEvidence(input(), validSummary())
	if len(clean) != 0 {
		t.Errorf("faithful quotes reported as mismatches: %v", clean)
	}
}
