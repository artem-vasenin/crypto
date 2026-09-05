// output/json.go
package output

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"universal-bybit-screener/models"
)

var (
	hashMu     sync.Mutex
	fileHashes = make(map[string]string)
)

func WriteJSON(path string, v models.ScreeningResult) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	return atomicWrite(path, data)
}

func WriteJSONIfChanged(path string, v models.ScreeningResult) (bool, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return false, fmt.Errorf("json marshal error: %w", err)
	}

	hasher := md5.New()
	hasher.Write(data)
	currentHash := hex.EncodeToString(hasher.Sum(nil))

	hashMu.Lock()
	lastHash, exists := fileHashes[path]
	if exists && lastHash == currentHash {
		hashMu.Unlock()
		return false, nil
	}
	fileHashes[path] = currentHash
	hashMu.Unlock()

	if err := atomicWrite(path, data); err != nil {
		return false, err
	}

	return true, nil
}

func atomicWrite(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s error: %w", dir, err)
		}
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp file %s error: %w", tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic rename %s -> %s error: %w", tmp, path, err)
	}

	return nil
}
