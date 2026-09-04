package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunWritesHelpWithAllDefaultsToProvidedStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(context.Background(), []string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	output := stderr.String()
	for _, fragment := range []string{
		"Usage: quorum-ingest [flags]",
		"--site  default=\"\"",
		"--archive  default=\"\"",
		"--watermark-patterns  default=\"\"",
		"--dry-run  default=false",
		"--reject-threshold  default=0.5",
		"--max-record-bytes  default=8388608",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("help missing %q: %s", fragment, output)
		}
	}
}

func TestRunWritesLocatedRequestErrorsToProvidedStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(context.Background(), []string{"--site", "stackoverflow.com", "--tables", "posts,widgets", "--dry-run"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), "error table=widgets offset=0 cause=unsupported corpus table \"widgets\"\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunReturnsSyntaxExitCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := run(context.Background(), []string{"--reject-threshold", "nope"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `invalid value "nope" for flag -reject-threshold`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
