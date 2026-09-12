// internal/execution/snapshot_logger.go
package execution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"universal-bybit-screener/models"
)

type TradeSnapshot struct {
	Timestamp       time.Time        `json:"timestamp"`
	Symbol          string           `json:"symbol"`
	Side            string           `json:"side"`
	Price           float64          `json:"price"`
	Qty             float64          `json:"qty"`
	Leverage        int              `json:"leverage"`
	OrderID         string           `json:"order_id"`
	ExecutionReason string           `json:"execution_reason"`
	ExecutionFee    float64          `json:"execution_fee"`
	ExecutionID     string           `json:"execution_id"`
	ExecutionTime   time.Time        `json:"execution_time"`
	Candidate       models.Candidate `json:"candidate_metrics"`
	BTC15mTrendPct  float64          `json:"btc_15m_trend_pct"`
}

func SaveTradeSnapshot(symbol, side string, price, qty float64, leverage int, orderID string, candidate models.Candidate, btcTrendPct, executionFee float64, executionID string, executionTime time.Time) error {
	exePath, err := os.Executable()
	baseDir := "."
	if err == nil {
		baseDir = filepath.Dir(exePath)
	}

	snapshotDir := filepath.Join(baseDir, "snapshots")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot dir: %w", err)
	}

	stratKey := "long"
	if strings.EqualFold(side, "Sell") || strings.EqualFold(side, "short") {
		stratKey = "short"
	}

	execReason := ""
	if st, ok := candidate.Strategies[stratKey]; ok {
		execReason = st.Reason
	}

	snap := TradeSnapshot{
		Timestamp:       time.Now().UTC(),
		Symbol:          symbol,
		Side:            side,
		Price:           price,
		Qty:             qty,
		Leverage:        leverage,
		OrderID:         orderID,
		ExecutionReason: execReason,
		ExecutionFee:    executionFee,
		ExecutionID:     executionID,
		ExecutionTime:   executionTime,
		Candidate:       candidate,
		BTC15mTrendPct:  btcTrendPct,
	}

	orderPart := orderID
	if len(orderPart) > 8 {
		orderPart = orderPart[:8]
	}
	fileName := fmt.Sprintf("%s_%s_%s_%s.json",
		snap.Timestamp.Format("20060102_150405"),
		symbol,
		side,
		orderPart,
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
