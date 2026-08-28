package output

import (
	"os"
	"path/filepath"
	"testing"
	"universal-bybit-screener/models"
)

func TestWriteJSONAtomic(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "result.json")
	v := models.ScreeningResult{Strategy: "neutral-grid"}
	if err := WriteJSON(p, v); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("JSON is empty")
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
}
