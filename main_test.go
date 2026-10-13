package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const overlapFixture = "testdata/transcript_overlap.json"

func runCLI(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

// S7: the dry run exercises the whole path from transcript to payload with no
// credential and no model call.
func TestDryRunPrintsThePayloadWithoutCredentials(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	code, stdout, stderr := runCLI(t, "", "-dry-run", "-in", overlapFixture)
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr = %s)", code, exitOK, stderr)
	}

	want := "[00:01:00] Ana: The deploy window moves to Thursday.\n" +
		"[00:01:10] Bruno: I can prepare the rollback plan.\n" +
		"[unknown] Ana: Nobody volunteered for the customer note.\n" +
		"[unknown] unknown: Second recorder picked up an unidentified voice.\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestDryRunReadsStandardInput(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	code, stdout, stderr := runCLI(t,
		`{"meetingID":"mtg-stdin","transcriptSegments":[{"speaker":"Ana","text":"Only line."}]}`,
		"-dry-run")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr = %s)", code, exitOK, stderr)
	}
	if want := "[unknown] Ana: Only line.\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// S4: an empty transcript fails with the named contract error and prints no summary.
func TestEmptyTranscriptFailsWithExitOne(t *testing.T) {
	code, stdout, stderr := runCLI(t, "", "-dry-run", "-in", "testdata/transcript_empty.json")
	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "transcriptSegments must contain at least one segment") {
		t.Errorf("stderr = %q, want the contract error", stderr)
	}
}

func TestMissingTranscriptFileFailsWithExitOne(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent.json")

	code, stdout, stderr := runCLI(t, "", "-dry-run", "-in", absent)
	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "transcript:") {
		t.Errorf("stderr = %q, want a transcript error", stderr)
	}
}

// S7: a model run without credentials fails before any network call and prints nothing.
func TestSummarizeWithoutCredentialsFailsWithExitOne(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	code, stdout, stderr := runCLI(t, "", "-in", overlapFixture)
	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "GOOGLE_API_KEY is required") {
		t.Errorf("stderr = %q, want the missing credential error", stderr)
	}
}

func TestUnknownFlagFailsWithExitTwo(t *testing.T) {
	code, stdout, stderr := runCLI(t, "", "-nope")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "flag provided but not defined") {
		t.Errorf("stderr = %q, want the flag error", stderr)
	}
}

func TestHelpPrintsUsageAndExitsZero(t *testing.T) {
	code, stdout, stderr := runCLI(t, "", "-h")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	for _, want := range []string{"-dry-run", "-in string", "-out string", "GOOGLE_API_KEY", "out of scope"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("usage is missing %q", want)
		}
	}
}

func TestWriteOutputWritesAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.json")

	if err := writeOutput(path, []byte(`{"ok":true}`), io.Discard); err != nil {
		t.Fatalf("writeOutput: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if want := "{\"ok\":true}\n"; string(raw) != want {
		t.Errorf("file = %q, want %q", raw, want)
	}
}
