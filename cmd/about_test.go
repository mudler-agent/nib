package cmd

import (
	"strings"
	"testing"

	"github.com/mudler/nib/types"
)

func TestAboutTextContainsVersion(t *testing.T) {
	cfg := types.Config{ProgramName: "nib"}
	got := AboutText(cfg)

	if !strings.Contains(got, "nib") {
		t.Errorf("AboutText missing program name: %q", got)
	}
	// A local build without ldflags reports "dev (local build)".
	if !strings.Contains(got, "dev (local build)") {
		// Released builds report "Version (Commit)".
		if !strings.Contains(got, "(") {
			t.Errorf("AboutText missing version: %q", got)
		}
	}
}

func TestAboutTextContainsConfigPaths(t *testing.T) {
	cfg := types.Config{ProgramName: "nib"}
	got := AboutText(cfg)

	if !strings.Contains(got, "Configuration paths") {
		t.Errorf("AboutText missing config paths header: %q", got)
	}
	if !strings.Contains(got, "Writable config:") {
		t.Errorf("AboutText missing writable config: %q", got)
	}
}

func TestAboutTextContainsSelfReadMention(t *testing.T) {
	cfg := types.Config{ProgramName: "nib"}
	got := AboutText(cfg)

	if !strings.Contains(got, "about-nib") {
		t.Errorf("AboutText missing about-nib mention: %q", got)
	}
}

func TestAboutTextCustomProgramName(t *testing.T) {
	cfg := types.Config{ProgramName: "my-agent"}
	got := AboutText(cfg)

	if !strings.Contains(got, "my-agent") {
		t.Errorf("AboutText missing custom program name: %q", got)
	}
}
