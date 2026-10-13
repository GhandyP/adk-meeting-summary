// Package meeting defines the typed contract between a supplied meeting
// transcript and its summary, together with the validation rules that keep the
// summary faithful to the transcript.
//
// It performs no I/O and calls no model: every rule here is deterministic and
// testable without credentials.
package meeting

import (
	"errors"
	"strings"
)

// Unknown marks a value the transcript never stated. It is recorded, never guessed.
const Unknown = "unknown"

// Contract errors. Callers match them with errors.Is.
var (
	ErrEmptyMeetingID           = errors.New("meetingID is required")
	ErrNoSegments               = errors.New("transcriptSegments must contain at least one segment")
	ErrEmptySegmentText         = errors.New("transcript segment text must not be empty")
	ErrSummaryMeetingIDMismatch = errors.New("summary meetingID must match the input meetingID")
	ErrItemWithoutText          = errors.New("summary items require non-empty text")
	ErrItemWithoutEvidence      = errors.New("summary items require at least one evidence id")
	ErrDanglingEvidenceID       = errors.New("summary references an evidence id that is not defined in evidence")
	ErrDuplicateEvidenceID      = errors.New("evidence ids must be unique")
	ErrEvidenceIDRequired       = errors.New("evidence id must not be empty")
	ErrEvidenceIndexOutOfRange  = errors.New("evidence segmentIndex is out of range")
	ErrEmptyEvidenceQuote       = errors.New("evidence quote must not be empty")
	ErrUnknownParticipant       = errors.New("participant is not present in the transcript")
	ErrEmptyParticipantName     = errors.New("participant name must not be empty")
)

// TranscriptSegment is one speaker turn of a supplied transcript. Start and End
// stay empty when the source never stated them; a missing speaker is recorded as
// Unknown instead of being dropped.
type TranscriptSegment struct {
	Speaker string `json:"speaker"`
	Start   string `json:"start,omitempty"`
	End     string `json:"end,omitempty"`
	Text    string `json:"text"`
}

// MeetingInput is the transcript handed to the summarizer.
type MeetingInput struct {
	MeetingID string              `json:"meetingID"`
	Title     string              `json:"title,omitempty"`
	Segments  []TranscriptSegment `json:"transcriptSegments"`
}

// Evidence ties a summary item back to the transcript it came from.
type Evidence struct {
	ID           string `json:"id"`
	SegmentIndex int    `json:"segmentIndex"`
	Quote        string `json:"quote"`
}

// Decision is something the meeting settled.
type Decision struct {
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidenceIDs"`
}

// Action is agreed work. Owner and DueDate are Unknown when the transcript never
// stated them.
type Action struct {
	Text        string   `json:"text"`
	Owner       string   `json:"owner"`
	DueDate     string   `json:"dueDate"`
	EvidenceIDs []string `json:"evidenceIDs"`
}

// OpenQuestion is something the meeting left undecided.
type OpenQuestion struct {
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidenceIDs"`
}

// Participant is someone who spoke. Unknown marks a speaker the transcript
// identifies only as unidentified.
type Participant struct {
	Name    string `json:"name"`
	Role    string `json:"role,omitempty"`
	Unknown bool   `json:"unknown,omitempty"`
}

// MeetingSummary is the validated result. Every slice is non-nil so the JSON
// contract always renders arrays.
type MeetingSummary struct {
	MeetingID     string         `json:"meetingID"`
	Decisions     []Decision     `json:"decisions"`
	Actions       []Action       `json:"actions"`
	OpenQuestions []OpenQuestion `json:"openQuestions"`
	Participants  []Participant  `json:"participants"`
	Evidence      []Evidence     `json:"evidence"`
}

// NormalizeInput trims the transcript, records a missing speaker as Unknown and
// leaves missing timestamps empty. Segment order is preserved because it is the
// order the meeting happened in.
func NormalizeInput(in MeetingInput) MeetingInput {
	out := in
	out.MeetingID = strings.TrimSpace(in.MeetingID)
	out.Title = strings.TrimSpace(in.Title)

	out.Segments = make([]TranscriptSegment, 0, len(in.Segments))
	for _, seg := range in.Segments {
		seg.Speaker = strings.TrimSpace(seg.Speaker)
		if seg.Speaker == "" {
			seg.Speaker = Unknown
		}
		seg.Start = strings.TrimSpace(seg.Start)
		seg.End = strings.TrimSpace(seg.End)
		seg.Text = NormalizeText(seg.Text)
		out.Segments = append(out.Segments, seg)
	}
	return out
}

// NormalizeSummary makes the summary renderable and honest: every slice becomes
// non-nil, and a blank owner or due date becomes Unknown.
func NormalizeSummary(s MeetingSummary) MeetingSummary {
	out := s
	out.MeetingID = strings.TrimSpace(s.MeetingID)

	out.Decisions = make([]Decision, 0, len(s.Decisions))
	for _, d := range s.Decisions {
		d.Text = NormalizeText(d.Text)
		d.EvidenceIDs = normalizeIDs(d.EvidenceIDs)
		out.Decisions = append(out.Decisions, d)
	}

	out.Actions = make([]Action, 0, len(s.Actions))
	for _, a := range s.Actions {
		a.Text = NormalizeText(a.Text)
		a.Owner = orUnknown(a.Owner)
		a.DueDate = orUnknown(a.DueDate)
		a.EvidenceIDs = normalizeIDs(a.EvidenceIDs)
		out.Actions = append(out.Actions, a)
	}

	out.OpenQuestions = make([]OpenQuestion, 0, len(s.OpenQuestions))
	for _, q := range s.OpenQuestions {
		q.Text = NormalizeText(q.Text)
		q.EvidenceIDs = normalizeIDs(q.EvidenceIDs)
		out.OpenQuestions = append(out.OpenQuestions, q)
	}

	out.Participants = make([]Participant, 0, len(s.Participants))
	for _, p := range s.Participants {
		if p.Unknown {
			p.Name = orUnknown(p.Name)
		} else {
			p.Name = strings.TrimSpace(p.Name)
		}
		p.Role = strings.TrimSpace(p.Role)
		out.Participants = append(out.Participants, p)
	}

	out.Evidence = make([]Evidence, 0, len(s.Evidence))
	for _, e := range s.Evidence {
		e.ID = strings.TrimSpace(e.ID)
		e.Quote = NormalizeText(e.Quote)
		out.Evidence = append(out.Evidence, e)
	}

	return out
}

// NormalizeText collapses runs of whitespace so that quotes compare reliably.
func NormalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// IsUnknown reports whether a value is the explicit Unknown marker.
func IsUnknown(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), Unknown)
}

func orUnknown(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return Unknown
	}
	return s
}

func normalizeIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}
