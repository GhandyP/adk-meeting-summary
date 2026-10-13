package summary_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/genai"

	"github.com/GhandyP/adk-meeting-summary/internal/meeting"
	"github.com/GhandyP/adk-meeting-summary/internal/summary"
	"github.com/GhandyP/adk-meeting-summary/internal/transcript"
)

const fixtures = "../../testdata"

// modelAnswer is what the agent is expected to return for transcript_ok.json:
// complete items that cite their segment, and one action whose owner and due
// date the transcript never stated.
const modelAnswer = `{
  "decisions": [
    { "text": "Ship the importer on Friday.", "evidenceIDs": ["e1"] }
  ],
  "actions": [
    { "text": "Write the migration guide.", "owner": "Bruno", "dueDate": "2026-10-20", "evidenceIDs": ["e2"] },
    { "text": "Find an owner for the pricing page.", "evidenceIDs": ["e3"] }
  ],
  "openQuestions": [
    { "text": "Who owns the pricing page?", "evidenceIDs": ["e3"] }
  ],
  "participants": [
    { "name": "Ana" },
    { "name": "Bruno" }
  ],
  "evidence": [
    { "id": "e1", "segmentIndex": 0, "quote": "We ship the importer on Friday." },
    { "id": "e2", "segmentIndex": 1, "quote": "I will write the migration guide." },
    { "id": "e3", "segmentIndex": 2, "quote": "The pricing page still has no owner." }
  ]
}`

func transcriptOf(t *testing.T, name string) meeting.MeetingInput {
	t.Helper()
	in, err := transcript.Load(filepath.Join(fixtures, name), nil)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return in
}

// S3 and S6: the instruction carries the honesty rules and the text-only boundary.
func TestInstructionStatesTheHonestyRules(t *testing.T) {
	instruction := summary.Instruction()

	for _, want := range []string{
		"unknown",
		"Never guess or infer",
		"0-based",
		"verbatim",
		"Never claim to have transcribed audio",
	} {
		if !strings.Contains(instruction, want) {
			t.Errorf("instruction is missing %q", want)
		}
	}
}

// S1 and S2: the model is asked for the documented arrays and for nothing else.
func TestOutputSchemaDescribesTheSummaryContract(t *testing.T) {
	schema := summary.OutputSchema()

	if schema.Type != genai.TypeObject {
		t.Fatalf("schema type = %v, want object", schema.Type)
	}

	want := []string{"decisions", "actions", "openQuestions", "participants", "evidence"}
	for _, key := range want {
		property, ok := schema.Properties[key]
		if !ok {
			t.Fatalf("schema is missing %q", key)
		}
		if property.Type != genai.TypeArray {
			t.Errorf("%s type = %v, want array", key, property.Type)
		}
		if property.Items == nil {
			t.Errorf("%s has no item schema", key)
		}
	}
	if !reflect.DeepEqual(schema.Required, want) {
		t.Errorf("required = %v, want %v", schema.Required, want)
	}
	if _, requested := schema.Properties["meetingID"]; requested {
		t.Error("meetingID must not be requested from the model: the caller stamps it")
	}

	action := schema.Properties["actions"].Items
	for _, key := range []string{"text", "owner", "dueDate", "evidenceIDs"} {
		if _, ok := action.Properties[key]; !ok {
			t.Errorf("action schema is missing %q", key)
		}
	}
	if got := schema.Properties["evidence"].Items.Properties["segmentIndex"].Type; got != genai.TypeInteger {
		t.Errorf("segmentIndex type = %v, want integer", got)
	}
}

// S2 and S3: a model answer is parsed, the meeting id is stamped by the caller,
// and an omitted owner or due date becomes unknown.
func TestParseSummaryStampsTheMeetingIDAndMarksUnknownOwners(t *testing.T) {
	in := transcriptOf(t, "transcript_ok.json")

	got, err := summary.ParseSummary(in, []byte(modelAnswer))
	if err != nil {
		t.Fatalf("parse model answer: %v", err)
	}
	if got.MeetingID != in.MeetingID {
		t.Errorf("meetingID = %q, want %q", got.MeetingID, in.MeetingID)
	}
	if len(got.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(got.Actions))
	}
	if got.Actions[1].Owner != meeting.Unknown || got.Actions[1].DueDate != meeting.Unknown {
		t.Errorf("omitted owner/due date = %q/%q, want %q",
			got.Actions[1].Owner, got.Actions[1].DueDate, meeting.Unknown)
	}
	if needs := meeting.RequiresConfirmation(got); len(needs) != 1 {
		t.Errorf("RequiresConfirmation = %v, want exactly the action with no owner", needs)
	}
	if mismatches := meeting.AuditEvidence(in, got); len(mismatches) != 0 {
		t.Errorf("model quotes do not match the transcript: %v", mismatches)
	}
}

// S2: evidence that points outside the transcript is rejected, not trimmed.
func TestParseSummaryRejectsInventedEvidence(t *testing.T) {
	in := transcriptOf(t, "transcript_ok.json")
	raw := strings.Replace(modelAnswer, `"segmentIndex": 2`, `"segmentIndex": 99`, 1)

	_, err := summary.ParseSummary(in, []byte(raw))
	if !errors.Is(err, meeting.ErrEvidenceIndexOutOfRange) {
		t.Fatalf("error = %v, want %v", err, meeting.ErrEvidenceIndexOutOfRange)
	}
}

func TestParseSummaryRejectsANonJSONAnswer(t *testing.T) {
	in := transcriptOf(t, "transcript_ok.json")

	_, err := summary.ParseSummary(in, []byte("I cannot help with that."))
	if err == nil {
		t.Fatal("a non-JSON answer must fail")
	}
	if !strings.Contains(err.Error(), "decode model answer") {
		t.Errorf("error = %v, want it to say the answer could not be decoded", err)
	}
}

// S7: without credentials the agent is never built, so nothing can call a model.
func TestNewAgentRequiresCredentials(t *testing.T) {
	t.Setenv(summary.APIKeyEnv, "")

	agent, err := summary.NewAgent(context.Background(), summary.Config{APIKey: "   "})
	if !errors.Is(err, summary.ErrMissingAPIKey) {
		t.Fatalf("error = %v, want %v", err, summary.ErrMissingAPIKey)
	}
	if agent != nil {
		t.Error("no agent may be built without credentials")
	}
}

// The ADK round trip is the only part that needs credentials and a network. With
// no key it is skipped instead of faked; the wiring is covered by the build and
// the pure tests above.
func TestSummarizeLive(t *testing.T) {
	if strings.TrimSpace(os.Getenv(summary.APIKeyEnv)) == "" {
		t.Skipf("%s is not set: the live ADK round trip is not exercised", summary.APIKeyEnv)
	}

	ctx := context.Background()
	in := transcriptOf(t, "transcript_ok.json")

	agent, err := summary.NewAgent(ctx, summary.Config{})
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}
	got, err := agent.Summarize(ctx, in)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if err := meeting.ValidateSummary(in, got); err != nil {
		t.Errorf("live summary is invalid: %v", err)
	}
	if mismatches := meeting.AuditEvidence(in, got); len(mismatches) != 0 {
		t.Errorf("live summary quotes do not match the transcript: %v", mismatches)
	}
}
