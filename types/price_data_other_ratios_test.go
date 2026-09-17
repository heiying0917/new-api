package types

import (
	"math"
	"testing"
)

func TestPriceDataOtherRatioHelpers(t *testing.T) {
	var p PriceData
	if p.HasOtherRatio("n") {
		t.Fatal("empty PriceData should not report ratio n")
	}
	if got := p.OtherRatioMultiplier(); got != 1.0 {
		t.Fatalf("empty multiplier = %v, want 1", got)
	}
	p.AddOtherRatio("n", 3)
	p.AddOtherRatio("seconds", 2.5)
	p.AddOtherRatio("bad", math.NaN()) // rejected by AddOtherRatio
	if !p.HasOtherRatio("n") || p.HasOtherRatio("bad") {
		t.Fatalf("HasOtherRatio wrong: n=%v bad=%v", p.HasOtherRatio("n"), p.HasOtherRatio("bad"))
	}
	if got := p.OtherRatioMultiplier(); got != 7.5 {
		t.Fatalf("multiplier = %v, want 7.5", got)
	}
	if got := p.ApplyOtherRatiosToFloat(10); got != 75 {
		t.Fatalf("ApplyOtherRatiosToFloat(10) = %v, want 75", got)
	}
}
