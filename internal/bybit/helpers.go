package bybit

import (
	"fmt"
	"strconv"
	"time"
)

func f(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
func ts(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(n)
}
func fmtInt(n int) string {
	if n < 1 {
		return "1"
	}
	return strconv.Itoa(n)
}
func requirePositive(v float64, name string) error {
	if v <= 0 {
		return fmt.Errorf("%s must be positive", name)
	}
	return nil
}
