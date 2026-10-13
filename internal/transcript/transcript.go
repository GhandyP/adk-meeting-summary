// Package transcript loads a meeting transcript document from a file or standard
// input, validates it against the meeting contract, and renders the plain text
// payload handed to the model.
//
// v1 is text-only: no audio, no STT adapter, no speaker diarization.
package transcript

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GhandyP/adk-meeting-summary/internal/meeting"
)

// ErrNoStdin reports a "-" path without a reader to read from.
var ErrNoStdin = errors.New("a stdin reader is required when the path is \"-\"")

// Decode reads one transcript document, validates it and returns the normalized
// input. A document that cannot be summarized fails with a meeting contract
// error, so callers can match it with errors.Is.
func Decode(r io.Reader) (meeting.MeetingInput, error) {
	var in meeting.MeetingInput
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return meeting.MeetingInput{}, fmt.Errorf("decode transcript: %w", err)
	}
	if err := meeting.ValidateInput(in); err != nil {
		return meeting.MeetingInput{}, err
	}
	return meeting.NormalizeInput(in), nil
}

// Load reads a transcript from a file path, or from stdin when path is "-".
func Load(path string, stdin io.Reader) (meeting.MeetingInput, error) {
	if path == "-" {
		if stdin == nil {
			return meeting.MeetingInput{}, ErrNoStdin
		}
		return Decode(stdin)
	}

	file, err := os.Open(path)
	if err != nil {
		return meeting.MeetingInput{}, fmt.Errorf("open transcript: %w", err)
	}
	defer file.Close()

	return Decode(file)
}

// RenderPlainText renders the transcript as the text payload sent to the model.
// A timestamp the transcript never stated is labelled unknown instead of being
// invented, and the segment order is preserved.
func RenderPlainText(in meeting.MeetingInput) string {
	var b strings.Builder
	for _, seg := range in.Segments {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[%s] %s: %s", orUnknown(seg.Start), orUnknown(seg.Speaker), meeting.NormalizeText(seg.Text))
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String() + "\n"
}

func orUnknown(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return meeting.Unknown
	}
	return s
}
