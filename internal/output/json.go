package output

import (
	"encoding/json"
	"io"
)

// WriteJSON отвечает только за форматирование результата. Отдельный пакет
// позволяет без изменения аналитики позже добавить запись в файл/HTTP API.
func WriteJSON(w io.Writer, value any, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	enc.SetEscapeHTML(false)
	return enc.Encode(value)
}
