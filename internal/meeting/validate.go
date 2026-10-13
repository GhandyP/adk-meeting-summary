package meeting

import (
	"fmt"
	"strings"
)

// ValidateInput rejects a transcript that cannot be summarized at all. Missing
// timestamps and overlapping speakers are legitimate and pass.
func ValidateInput(in MeetingInput) error {
	if strings.TrimSpace(in.MeetingID) == "" {
		return ErrEmptyMeetingID
	}
	if len(in.Segments) == 0 {
		return ErrNoSegments
	}
	for i, seg := range in.Segments {
		if strings.TrimSpace(seg.Text) == "" {
			return fmt.Errorf("%w: segment %d", ErrEmptySegmentText, i)
		}
	}
	return nil
}

// ValidateSummary enforces the summary contract against the transcript it claims
// to describe. It rejects invented evidence, unbacked items, a summary of another
// meeting, and participants the transcript never had.
func ValidateSummary(in MeetingInput, s MeetingSummary) error {
	if strings.TrimSpace(s.MeetingID) != strings.TrimSpace(in.MeetingID) {
		return fmt.Errorf("%w: got %q, want %q", ErrSummaryMeetingIDMismatch, s.MeetingID, in.MeetingID)
	}

	known := make(map[string]struct{}, len(s.Evidence))
	for _, e := range s.Evidence {
		id := strings.TrimSpace(e.ID)
		if id == "" {
			return fmt.Errorf("%w: evidence %d", ErrEvidenceIDRequired, len(known))
		}
		if _, duplicate := known[id]; duplicate {
			return fmt.Errorf("%w: %q", ErrDuplicateEvidenceID, id)
		}
		known[id] = struct{}{}

		if e.SegmentIndex < 0 || e.SegmentIndex >= len(in.Segments) {
			return fmt.Errorf("%w: evidence %q points at segment %d of %d",
				ErrEvidenceIndexOutOfRange, id, e.SegmentIndex, len(in.Segments))
		}
		if strings.TrimSpace(e.Quote) == "" {
			return fmt.Errorf("%w: evidence %q", ErrEmptyEvidenceQuote, id)
		}
	}

	for _, d := range s.Decisions {
		if err := validateItem("decision", d.Text, d.EvidenceIDs, known); err != nil {
			return err
		}
	}
	for _, a := range s.Actions {
		if err := validateItem("action", a.Text, a.EvidenceIDs, known); err != nil {
			return err
		}
	}
	for _, q := range s.OpenQuestions {
		if err := validateItem("open question", q.Text, q.EvidenceIDs, known); err != nil {
			return err
		}
	}

	speakers := make(map[string]struct{}, len(in.Segments))
	for _, seg := range in.Segments {
		if speaker := strings.TrimSpace(seg.Speaker); speaker != "" && !IsUnknown(speaker) {
			speakers[strings.ToLower(speaker)] = struct{}{}
		}
	}
	for _, p := range s.Participants {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return ErrEmptyParticipantName
		}
		if p.Unknown || IsUnknown(name) {
			continue
		}
		if _, ok := speakers[strings.ToLower(name)]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownParticipant, name)
		}
	}

	return nil
}

// RequiresConfirmation lists what a human must confirm before these minutes are
// published or any task is created: every action whose owner or due date the
// transcript never stated.
func RequiresConfirmation(s MeetingSummary) []string {
	var out []string
	for _, a := range s.Actions {
		if IsUnknown(a.Owner) || IsUnknown(a.DueDate) {
			out = append(out, fmt.Sprintf(
				"confirm owner and due date for action %q (owner=%s, dueDate=%s)",
				NormalizeText(a.Text), orUnknown(a.Owner), orUnknown(a.DueDate)))
		}
	}
	return out
}

// AuditEvidence reports quotes that do not appear in the transcript segment they
// point at. It is advisory: ValidateSummary covers the structural rules.
func AuditEvidence(in MeetingInput, s MeetingSummary) []string {
	var out []string
	for _, e := range s.Evidence {
		if e.SegmentIndex < 0 || e.SegmentIndex >= len(in.Segments) {
			out = append(out, fmt.Sprintf("evidence %s does not point at a transcript segment (index %d)",
				e.ID, e.SegmentIndex))
			continue
		}
		segment := NormalizeText(in.Segments[e.SegmentIndex].Text)
		quote := NormalizeText(e.Quote)
		if !strings.Contains(strings.ToLower(segment), strings.ToLower(quote)) {
			out = append(out, fmt.Sprintf("evidence %s does not quote transcript segment %d",
				e.ID, e.SegmentIndex))
		}
	}
	return out
}

func validateItem(kind, text string, evidenceIDs []string, known map[string]struct{}) error {
	normalized := NormalizeText(text)
	if normalized == "" {
		return fmt.Errorf("%w: %s", ErrItemWithoutText, kind)
	}
	ids := normalizeIDs(evidenceIDs)
	if len(ids) == 0 {
		return fmt.Errorf("%w: %s %q", ErrItemWithoutEvidence, kind, normalized)
	}
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("%w: %s %q references %q", ErrDanglingEvidenceID, kind, normalized, id)
		}
	}
	return nil
}
