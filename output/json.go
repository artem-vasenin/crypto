package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"universal-bybit-screener/models"
)

// WriteJSON пишет во временный файл и затем атомарно заменяет целевой.
func WriteJSON(path string, v models.ScreeningResult) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
