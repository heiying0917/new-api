package types

import (
	"fmt"
	"math"
)

type GroupRatioInfo struct {
	GroupRatio        float64
	GroupSpecialRatio float64
	HasSpecialRatio   bool
}

type PriceData struct {
	FreeModel            bool
	ModelPrice           float64
	ModelRatio           float64
	CompletionRatio      float64
	CacheRatio           float64
	CacheCreationRatio   float64
	CacheCreation5mRatio float64
	CacheCreation1hRatio float64
	ImageRatio           float64
	AudioRatio           float64
	AudioCompletionRatio float64
	OtherRatios          map[string]float64
	UsePrice             bool
	Quota                int // 按次计费的最终额度（MJ / Task）
	QuotaToPreConsume    int // 按量计费的预消耗额度
	GroupRatioInfo       GroupRatioInfo
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if p.OtherRatios == nil {
		p.OtherRatios = make(map[string]float64)
	}
	// NaN/Inf would poison every downstream quota multiplication
	// (int(NaN * quota) wraps to a negative charge).
	if !(ratio > 0) || math.IsInf(ratio, 1) {
		return
	}
	p.OtherRatios[key] = ratio
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio)
}

// HasOtherRatio reports whether a valid (positive, finite) multiplier is
// registered under key. Ported from upstream fc1259f58 without privatizing
// OtherRatios, so existing callers keep compiling.
func (p *PriceData) HasOtherRatio(key string) bool {
	ratio, ok := p.OtherRatios[key]
	return ok && isValidOtherRatio(ratio)
}

// OtherRatioMultiplier returns the product of all valid other ratios (1.0 when none).
func (p *PriceData) OtherRatioMultiplier() float64 {
	multiplier := 1.0
	for _, ratio := range p.OtherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			multiplier *= ratio
		}
	}
	return multiplier
}

// ApplyOtherRatiosToFloat multiplies value by every valid other ratio.
func (p *PriceData) ApplyOtherRatiosToFloat(value float64) float64 {
	return value * p.OtherRatioMultiplier()
}

func isValidOtherRatio(ratio float64) bool {
	// NaN/Inf would poison every downstream quota multiplication
	// (int(NaN * quota) wraps to a negative charge).
	return ratio > 0 && !math.IsInf(ratio, 1)
}
