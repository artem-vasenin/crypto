package config

import (
	"os"
	"testing"
)

func TestLoadValidation(t *testing.T) {
	f := t.TempDir() + "/c.json"
	os.WriteFile(f, []byte(`{"min_history_bars":50}`), 0644)
	if _, e := Load(f); e == nil {
		t.Fatal("ожидалась ошибка валидации")
	}
}
