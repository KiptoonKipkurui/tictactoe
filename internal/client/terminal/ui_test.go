package terminal

import (
	"bytes"
	"strings"
	"testing"
)

func TestShowErrorReprintsPromptWhenMoveAllowed(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	ui := NewUI(strings.NewReader(""), &stdout, &stderr)
	ui.canMove = true

	ui.ShowError("cell is already occupied")

	if got := stderr.String(); got != "Server error: cell is already occupied\n" {
		t.Fatalf("stderr = %q", got)
	}
	if got := stdout.String(); got != "Your move: " {
		t.Fatalf("stdout = %q", got)
	}
}

func TestShowErrorDoesNotPrintPromptWhenMoveBlocked(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	ui := NewUI(strings.NewReader(""), &stdout, &stderr)
	ui.canMove = false

	ui.ShowError("it is not your turn")

	if got := stderr.String(); got != "Server error: it is not your turn\n" {
		t.Fatalf("stderr = %q", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q", got)
	}
}
