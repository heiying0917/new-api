package ratio_setting

import "testing"

// Official list prices (USD per 1M input tokens ÷ 2 = model ratio; output ÷ input = completion ratio):
//
//	OpenAI  gpt-5.5 $5, gpt-5.6-sol $5, gpt-5.6-terra $2.5, gpt-5.6-luna $1, gpt-6-astra $10/$50
//	Claude  opus-5 $5/$25, sonnet-5 $2/$10, sonnet-4.6 $3/$15, fable-5(.1) $10/$50, haiku-4.5 $1/$5
//	Cache   read 0.1x (fable-5.1 / mythos-5.1: 0.025x), write 1.25x
func TestLatestModelsHaveDefaultPricing(t *testing.T) {
	InitRatioSettings() // the RW maps are only seeded from the default tables at startup
	cases := []struct {
		model      string
		ratio      float64
		completion float64
		cacheRead  float64
	}{
		{"gpt-5.5", 2.5, 6, 0.1},
		{"gpt-5.6-sol", 2.5, 6, 0.1},
		{"gpt-5.6-terra", 1.25, 6, 0.1},
		{"gpt-5.6-luna", 0.5, 6, 0.1},
		{"gpt-6-astra", 5, 5, 0.1},
		{"claude-opus-5", 2.5, 5, 0.1},
		{"claude-opus-5-high", 2.5, 5, 0.1},
		{"claude-opus-5-thinking", 2.5, 5, 0.1},
		{"claude-sonnet-5", 1, 5, 0.1},
		{"claude-sonnet-5-xhigh", 1, 5, 0.1},
		{"claude-sonnet-4-6", 1.5, 5, 0.1},
		{"claude-sonnet-4-6-high", 1.5, 5, 0.1},
		{"claude-fable-5-1", 5, 5, 0.025},
		{"claude-fable-5-1-xhigh", 5, 5, 0.025},
		{"claude-fable-5", 5, 5, 0.1},
		{"claude-mythos-5-1", 5, 5, 0.025},
		{"claude-mythos-5", 5, 5, 0.1},
		{"claude-haiku-4-5", 0.5, 5, 0.1},
	}
	for _, c := range cases {
		ratio, ok, _ := GetModelRatio(c.model)
		if !ok || ratio != c.ratio {
			t.Errorf("%s: model ratio = %v (ok=%v), want %v", c.model, ratio, ok, c.ratio)
		}
		if got := GetCompletionRatio(c.model); got != c.completion {
			t.Errorf("%s: completion ratio = %v, want %v", c.model, got, c.completion)
		}
		if got, _ := GetCacheRatio(c.model); got != c.cacheRead {
			t.Errorf("%s: cache read ratio = %v, want %v", c.model, got, c.cacheRead)
		}
		if got, _ := GetCreateCacheRatio(c.model); got != 1.25 {
			t.Errorf("%s: create cache ratio = %v, want 1.25", c.model, got)
		}
	}
}
