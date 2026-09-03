package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/clofour/trellis/internal/lifecycle"
)

func TestJobsEventsCommandRegistered(t *testing.T) {
	config = CLIConfig{}
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"jobs", "events"})
	if err != nil {
		t.Fatalf("find jobs events: %v", err)
	}
	if cmd.Name() != "events" {
		t.Fatalf("found command %q, want events", cmd.Name())
	}
	if cmd.Flags().Lookup("allocation") == nil {
		t.Fatal("jobs events is missing --allocation")
	}
	if cmd.Flags().Lookup("output") == nil {
		t.Fatal("jobs events is missing structured --output")
	}
}

func TestPrintJobEvents(t *testing.T) {
	events := []jobAllocationEvent{
		{
			Allocation: "abcdef12-rest",
			Group:      "web",
			Phase:      lifecycle.PhaseRunning,
			At:         time.Date(2026, 9, 3, 12, 0, 2, 0, time.UTC),
		},
		{
			Allocation: "abcdef12-rest",
			Group:      "web",
			Phase:      lifecycle.PhaseFailed,
			Reason:     "image_pull_failed",
			Message:    "first line\nsecond line",
			At:         time.Date(2026, 9, 3, 12, 0, 1, 0, time.UTC),
		},
	}

	var output bytes.Buffer
	if err := printJobEvents(&output, events); err != nil {
		t.Fatalf("print events: %v", err)
	}
	text := output.String()
	for _, want := range []string{
		"Allocation",
		"Task group",
		"Lifecycle",
		"abcdef12",
		"failed",
		"image_pull_failed",
		"first line second line",
		"running",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output %q does not contain %q", text, want)
		}
	}
}
