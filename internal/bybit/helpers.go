// internal/bybit/helpers.go
package bybit

import (
	"strconv"
	"time"
)

// f безопасно конвертирует строку в float64. При ошибке возвращает 0.0
func f(s string) float64 {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return val
}

// ts парсит миллисекунды в формате строки в time.Time UTC
func ts(s string) time.Time {
	msec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(msec).UTC()
}

// fmtInt конвертирует int в строку для URL query
func fmtInt(n int) string {
	if n < 1 {
		return "1"
	}
	return strconv.Itoa(n)
}
