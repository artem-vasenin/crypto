package structure

import (
	"candidate-screener/internal/domain"
	"testing"
	"time"
)

func TestInsufficientStructure(t *testing.T) {
	c := make([]domain.Candle, 3)
	for i := range c {
		c[i].OpenTime = time.Now()
	}
	if got := Classify(c, 2); got != domain.StructureUnknown {
		t.Fatalf("got %s", got)
	}
}
