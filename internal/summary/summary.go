// Package summary turns a validated transcript into a validated meeting summary
// by running one Google ADK agent.
//
// The model boundary is deliberately small: everything that can be checked
// without credentials - the instruction, the output schema, the parser and the
// validation - is a pure function. Only Agent talks to a model.
package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/GhandyP/adk-meeting-summary/internal/meeting"
	"github.com/GhandyP/adk-meeting-summary/internal/transcript"
)

const (
	// DefaultModel is the Gemini model the README documents.
	DefaultModel = "gemini-flash-latest"
	// ModelEnv overrides DefaultModel.
	ModelEnv = "ADK_MODEL"
	// APIKeyEnv is the environment variable the model client reads.
	APIKeyEnv = "GOOGLE_API_KEY"
	// AppName identifies the ADK application that owns the sessions.
	AppName   = "meeting-summary"
	agentName = "meeting_summarizer"
	userID    = "meeting-summary-cli"
)

// ErrMissingAPIKey reports a model call attempted without credentials.
var ErrMissingAPIKey = fmt.Errorf("%s is required to call the model", APIKeyEnv)

// ErrNoAgentOutput reports an agent turn that produced no summary at all.
var ErrNoAgentOutput = errors.New("the agent finished without a summary")

const instruction = `You summarize ONE supplied meeting transcript.

Rules:
- Work only from the transcript you are given. Never use outside knowledge.
- decisions: what the meeting settled. actions: agreed work. openQuestions: what the meeting left undecided.
- For every action, copy the owner and the due date exactly as the transcript states them.
  When the transcript does not state an owner or a due date, write "unknown".
  Never guess or infer an owner or a due date, and never invent a person's name.
- Every decision, action and open question needs at least one evidence id.
  evidence.segmentIndex is the 0-based position of a provided transcript segment, and
  evidence.quote must copy text from that segment verbatim.
- participants: only the speakers that appear in the transcript. When a segment has no
  identifiable speaker, use "unknown" as the participant name.
- Write "unknown" for anything the transcript does not state.
- The input is already text. Never claim to have transcribed audio, and never mention
  audio, recordings, Whisper or speakers that are not in the transcript.
`

// Instruction returns the agent instruction. It keeps the summary inside the
// supplied transcript (S3) and inside the text-only boundary (S6, S7).
func Instruction() string {
	return instruction
}

// OutputSchema describes the JSON object the model must return. The meeting id
// is deliberately absent: the caller stamps it, so the model cannot get it wrong.
func OutputSchema() *genai.Schema {
	evidenceIDs := func(description string) *genai.Schema {
		return &genai.Schema{
			Type:        genai.TypeArray,
			Description: description,
			Items:       &genai.Schema{Type: genai.TypeString},
		}
	}

	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"decisions": {
				Type:        genai.TypeArray,
				Description: "What the meeting settled.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":        {Type: genai.TypeString, Description: "One sentence per decision."},
						"evidenceIDs": evidenceIDs("Evidence ids that support this decision."),
					},
					Required: []string{"text", "evidenceIDs"},
				},
			},
			"actions": {
				Type:        genai.TypeArray,
				Description: "Work the meeting agreed to do.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":        {Type: genai.TypeString, Description: "One sentence per action."},
						"owner":       {Type: genai.TypeString, Description: "The owner exactly as stated, or \"unknown\"."},
						"dueDate":     {Type: genai.TypeString, Description: "The due date exactly as stated, or \"unknown\"."},
						"evidenceIDs": evidenceIDs("Evidence ids that support this action."),
					},
					Required: []string{"text", "owner", "dueDate", "evidenceIDs"},
				},
			},
			"openQuestions": {
				Type:        genai.TypeArray,
				Description: "What the meeting left undecided.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":        {Type: genai.TypeString, Description: "One sentence per open question."},
						"evidenceIDs": evidenceIDs("Evidence ids that support this question."),
					},
					Required: []string{"text", "evidenceIDs"},
				},
			},
			"participants": {
				Type:        genai.TypeArray,
				Description: "Speakers that appear in the transcript.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"name": {Type: genai.TypeString, Description: "The speaker name from the transcript."},
						"role": {Type: genai.TypeString, Description: "The role, only when the transcript states it."},
					},
					Required: []string{"name"},
				},
			},
			"evidence": {
				Type:        genai.TypeArray,
				Description: "Transcript quotations backing the items above.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id":           {Type: genai.TypeString, Description: "Short id referenced by the items above."},
						"segmentIndex": {Type: genai.TypeInteger, Description: "0-based position of the transcript segment."},
						"quote":        {Type: genai.TypeString, Description: "Text copied verbatim from that segment."},
					},
					Required: []string{"id", "segmentIndex", "quote"},
				},
			},
		},
		Required: []string{"decisions", "actions", "openQuestions", "participants", "evidence"},
	}
}

// ParseSummary decodes the model's JSON answer for one transcript, stamps the
// meeting id and validates the result against the transcript. A model answer
// that invents evidence or participants is rejected, not repaired.
func ParseSummary(in meeting.MeetingInput, raw []byte) (meeting.MeetingSummary, error) {
	var parsed meeting.MeetingSummary
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return meeting.MeetingSummary{}, fmt.Errorf("decode model answer: %w", err)
	}

	parsed.MeetingID = in.MeetingID
	parsed = meeting.NormalizeSummary(parsed)

	if err := meeting.ValidateSummary(in, parsed); err != nil {
		return meeting.MeetingSummary{}, fmt.Errorf("model answer rejected: %w", err)
	}
	return parsed, nil
}

// Config selects the model and the credentials. Both default to the environment.
type Config struct {
	// ModelName defaults to ADK_MODEL and then to DefaultModel.
	ModelName string
	// APIKey defaults to APIKeyEnv.
	APIKey string
}

// Agent runs one ADK agent over a transcript.
type Agent struct {
	runner *runner.Runner
	runs   atomic.Uint64
}

// NewAgent builds the single-agent summarizer. It fails before any network call
// when no credentials are available.
func NewAgent(ctx context.Context, cfg Config) (*Agent, error) {
	modelName := strings.TrimSpace(cfg.ModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(os.Getenv(ModelEnv))
	}
	if modelName == "" {
		modelName = DefaultModel
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv(APIKeyEnv))
	}
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	model, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("create model %q: %w", modelName, err)
	}

	// No tools: drafting minutes must not create tasks, write calendars or publish.
	summarizer, err := llmagent.New(llmagent.Config{
		Name:         agentName,
		Model:        model,
		Description:  "Summarizes one supplied meeting transcript into decisions, actions, open questions, participants and evidence.",
		Instruction:  Instruction(),
		OutputSchema: OutputSchema(),
	})
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	run, err := runner.NewInMemory(AppName, summarizer)
	if err != nil {
		return nil, fmt.Errorf("create runner: %w", err)
	}

	return &Agent{runner: run}, nil
}

// Summarize runs one turn over the transcript and returns the validated summary.
func (a *Agent) Summarize(ctx context.Context, in meeting.MeetingInput) (meeting.MeetingSummary, error) {
	if err := meeting.ValidateInput(in); err != nil {
		return meeting.MeetingSummary{}, err
	}
	normalized := meeting.NormalizeInput(in)

	// A fresh session per run keeps one meeting's answer out of the next prompt.
	sessionID := fmt.Sprintf("%s-%d", normalized.MeetingID, a.runs.Add(1))
	prompt := genai.NewContentFromText(transcript.RenderPlainText(normalized), genai.RoleUser)

	var raw, text string
	for event, err := range a.runner.Run(ctx, userID, sessionID, prompt, agent.RunConfig{}) {
		if err != nil {
			return meeting.MeetingSummary{}, fmt.Errorf("run agent: %w", err)
		}
		if event == nil {
			continue
		}
		// ADK stores the value parsed against OutputSchema on the terminal event.
		// Read it wherever it appears and keep the last one, so a differently
		// marked final event cannot make the summary disappear.
		if event.Output != nil {
			encoded, err := json.Marshal(event.Output)
			if err != nil {
				return meeting.MeetingSummary{}, fmt.Errorf("encode agent output: %w", err)
			}
			raw = string(encoded)
		}
		if event.IsFinalResponse() {
			if answer := eventText(event); answer != "" {
				text = answer
			}
		}
	}

	if raw == "" {
		if text == "" {
			return meeting.MeetingSummary{}, ErrNoAgentOutput
		}
		raw = text
	}
	return ParseSummary(normalized, []byte(raw))
}

func eventText(event *session.Event) string {
	if event == nil || event.Content == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range event.Content.Parts {
		if part != nil {
			b.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(b.String())
}
