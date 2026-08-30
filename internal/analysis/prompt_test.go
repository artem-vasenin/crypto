package analysis

import (
	"strings"
	"testing"
)

func TestBuildAIPrompt(t *testing.T) {
	p := BuildAIPrompt("neutral-grid")
	for _, want := range []string{"neutral-grid", "support/resistance", "RSI", "ATR", "Open Interest", "стакан", "Score"} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt does not contain %q", want)
		}
	}
}
