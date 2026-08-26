package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{"bybit":{"base_url":"https://api.bybit.com","category":"linear"},"filters":{"max_price":2,"min_turnover_24h":5000000,"top_candidates":20},"analysis":{"concurrency":3},"output":{"file":"screening.json"}}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bybit.Category != "linear" || cfg.Filters.TopCandidates != 20 || cfg.Analysis.Concurrency != 3 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
