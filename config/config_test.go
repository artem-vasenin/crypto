package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "config.json")
	data := []byte(`{"bybit":{"base_url":"https://api.bybit.com"},"filters":{"max_price":2,"min_turnover_24h":5000000,"preselect_candidates":60,"top_candidates":20},"analysis":{"kline_limit_15m":300,"kline_limit_1h":300,"kline_limit_4h":300,"open_interest_limit":50,"funding_limit":20},"concurrency":8,"http_timeout":"10s","run_timeout":"3m","max_retries":3,"retry_delay":"500ms","output":{"file":"screening.json"}}`)
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Filters.PreselectCandidates != 60 || cfg.Filters.TopCandidates != 20 {
		t.Fatalf("unexpected filters: %+v", cfg.Filters)
	}
}
