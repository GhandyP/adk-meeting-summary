// Command adk-meeting-summary turns a supplied text meeting transcript into a
// validated, evidence-bearing summary.
//
// It never transcribes audio, and it never writes tasks or calendar events.
//
// Exit codes:
//
//	0  the summary was produced (or help was printed)
//	1  the transcript, the model, or the summary failed validation
//	2  the command line was wrong
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/GhandyP/adk-meeting-summary/internal/meeting"
	"github.com/GhandyP/adk-meeting-summary/internal/summary"
	"github.com/GhandyP/adk-meeting-summary/internal/transcript"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

const usage = `adk-meeting-summary turns a supplied text meeting transcript into a validated summary.

Usage:
  adk-meeting-summary [flags]

Flags:
  -in string     transcript JSON file to read, or "-" for stdin (default "-")
  -out string    file for the summary JSON, or "-" for stdout (default "-")
  -model string  Gemini model name (default: ADK_MODEL, then ` + summary.DefaultModel + `)
  -dry-run       validate the transcript and print the model payload without calling a model
  -pretty        indent the summary JSON (default true)

Environment:
  ` + summary.APIKeyEnv + `  required to call the model; not needed with -dry-run
  ` + summary.ModelEnv + `       overrides the default model name

The summary is validated against the transcript before it is printed: an answer that
invents evidence, participants, owners or due dates is rejected. Actions whose owner or
due date the transcript never stated are reported on stderr for human confirmation.

Audio transcription and task or calendar writes are out of scope.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("adk-meeting-summary", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }

	inPath := flags.String("in", "-", "transcript JSON file, or \"-\" for stdin")
	outPath := flags.String("out", "-", "summary JSON file, or \"-\" for stdout")
	modelName := flags.String("model", "", "Gemini model name")
	dryRun := flags.Bool("dry-run", false, "validate the transcript and print the model payload")
	pretty := flags.Bool("pretty", true, "indent the summary JSON")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n\n", flags.Arg(0))
		flags.Usage()
		return exitUsage
	}

	input, err := transcript.Load(*inPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "transcript: %v\n", err)
		return exitFailure
	}

	if *dryRun {
		fmt.Fprint(stdout, transcript.RenderPlainText(input))
		return exitOK
	}

	ctx := context.Background()
	agent, err := summary.NewAgent(ctx, summary.Config{ModelName: *modelName})
	if err != nil {
		fmt.Fprintf(stderr, "agent: %v\n", err)
		return exitFailure
	}

	result, err := agent.Summarize(ctx, input)
	if err != nil {
		fmt.Fprintf(stderr, "summary: %v\n", err)
		return exitFailure
	}

	for _, warning := range meeting.AuditEvidence(input, result) {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}
	for _, pending := range meeting.RequiresConfirmation(result) {
		fmt.Fprintf(stderr, "needs human confirmation: %s\n", pending)
	}

	encoded, err := marshalSummary(result, *pretty)
	if err != nil {
		fmt.Fprintf(stderr, "encode summary: %v\n", err)
		return exitFailure
	}
	if err := writeOutput(*outPath, encoded, stdout); err != nil {
		fmt.Fprintf(stderr, "write summary: %v\n", err)
		return exitFailure
	}
	return exitOK
}

func marshalSummary(result meeting.MeetingSummary, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(result, "", "  ")
	}
	return json.Marshal(result)
}

func writeOutput(path string, content []byte, stdout io.Writer) error {
	if path == "-" {
		_, err := stdout.Write(append(content, '\n'))
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0o644)
}
