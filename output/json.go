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

// fileHashes хранит последнюю посчитанную MD5-сумму полезной нагрузки для каждого выходного файла.
// Используется для предотвращения холостых перезаписей на диск, если состав кандидатов не изменился.
var (
	hashMu     sync.Mutex
	fileHashes = make(map[string]string)
)

// WriteJSON выполняет гарантированную и атомарную запись результатов скрининга на диск.
// Метод пишет данные во временный файл (.tmp) и переименовывает его в целевой.
// Это предотвращает чтение поврежденных или частично записанных JSON внешними процессами (ИИ/Execution Bot).
func WriteJSON(path string, v models.ScreeningResult) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации JSON: %w", err)
	}

	return atomicWrite(path, data)
}

// WriteJSONIfChanged проверяет MD5-хэш сгенерированных данных и перезаписывает файл на диске
// ТОЛЬКО в том случае, если содержимое изменилось по сравнению с предыдущей итерацией демона.
// Это существенно снижает I/O нагрузку на накопитель при высокой частоте скрининга.
func WriteJSONIfChanged(path string, v models.ScreeningResult) (bool, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return false, fmt.Errorf("ошибка сериализации JSON: %w", err)
	}

	// Расчет MD5-контрольной суммы текущего снимка данных
	hasher := md5.New()
	hasher.Write(data)
	currentHash := hex.EncodeToString(hasher.Sum(nil))

	hashMu.Lock()
	lastHash, exists := fileHashes[path]
	if exists && lastHash == currentHash {
		hashMu.Unlock()
		// Данные идентичны предыдущему тику, запись на диск не требуется
		return false, nil
	}
	fileHashes[path] = currentHash
	hashMu.Unlock()

	// Запись выполняется только при обнаружении дельты в данных
	if err := atomicWrite(path, data); err != nil {
		return false, err
	}

	return true, nil
}

// atomicWrite реализует паттерн безопасной атомарной записи на уровне файловой системы.
func atomicWrite(path string, data []byte) error {
	// Создание целевой директории, если она отсутствует
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("ошибка создания директории %s: %w", dir, err)
		}
	}

	// Запись во временный буферный файл
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("ошибка записи во временный файл %s: %w", tmp, err)
	}

	// Мгновенная замена оригинального файла атомарной операцией переименования ОС
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // Очистка временного файла при сбое
		return fmt.Errorf("ошибка атомарной замены файла %s -> %s: %w", tmp, path, err)
	}

	return nil
}
