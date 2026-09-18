// Package output отвечает только за сериализацию результата.
package output

import (
	"encoding/json"
	"io"
)

// WriteJSON пишет UTF-8 JSON без HTML-экранирования и с опциональными отступами.
func WriteJSON(w io.Writer, value any, pretty bool) error {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	if pretty {
		e.SetIndent("", "  ")
	}
	return e.Encode(value)
}
