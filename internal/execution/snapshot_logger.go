// internal/execution/snapshot_logger.go
package execution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"universal-bybit-screener/models"
)

type TradeSnapshot struct {
	Timestamp  time.Time        `json:"timestamp"`
	Symbol     string           `json:"symbol"`
	Side       string           `json:"side"`
	Price      float64          `json:"price"`
	Qty        float64          `json:"qty"`
	Leverage   int              `json:"leverage"`
	OrderID    string           `json:"order_id"`
	Candidate  models.Candidate `json:"candidate_metrics"`
	BTCContext interface{}      `json:"btc_context,omitempty"`
}

func SaveTradeSnapshot(symbol, side string, price, qty float64, leverage int, orderID string, candidate models.Candidate) error {
	exePath, err := os.Executable()
	baseDir := "."
	if err == nil {
		baseDir = filepath.Dir(exePath)
	}

	snapshotDir := filepath.Join(baseDir, "snapshots")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot dir: %w", err)
	}

	snap := TradeSnapshot{
		Timestamp: time.Now().UTC(),
		Symbol:    symbol,
		Side:      side,
		Price:     price,
		Qty:       qty,
		Leverage:  leverage,
		OrderID:   orderID,
		Candidate: candidate,
	}

	fileName := fmt.Sprintf("%s_%s_%s_%s.json",
		snap.Timestamp.Format("20060102_150405"),
		symbol,
		side,
		orderID[:8],
	)
	filePath := filepath.Join(snapshotDir, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snap); err != nil {
		return fmt.Errorf("failed to write snapshot json: %w", err)
	}

	return nil
}
