package official_pricing

import (
	"testing"
)

func fp(v float64) *float64 { return &v }

func deref(p *float64) float64 {
	if p == nil {
		return -999
	}
	return *p
}

// fixtureProviders builds a small models.dev-like dataset exercising:
//   - first-party preference (anthropic over venice; openai over a cheaper reseller)
//   - a non-first-party-only name variant (anthropic/claude-opus-4.8 via openrouter)
//   - a zero-priced (free) model
//   - two distinct raw names colliding on one normalized key with no first-party
func fixtureProviders() map[string]modelsDevProvider {
	return map[string]modelsDevProvider{
		"anthropic": {Models: map[string]modelsDevModel{
			"claude-opus-4-6": {Cost: modelsDevCost{Input: fp(5), Output: fp(25), CacheRead: fp(0.5), CacheWrite: fp(6.25)}},
		}},
		"venice": {Models: map[string]modelsDevModel{
			"claude-opus-4-6": {Cost: modelsDevCost{Input: fp(6), Output: fp(30)}},
		}},
		"openrouter": {Models: map[string]modelsDevModel{
			"anthropic/claude-opus-4.8": {Cost: modelsDevCost{Input: fp(5), Output: fp(25)}},
		}},
		"openai": {Models: map[string]modelsDevModel{
			"gpt-4o": {Cost: modelsDevCost{Input: fp(2.5), Output: fp(10), CacheRead: fp(1.25)}},
		}},
		"cheapreseller": {Models: map[string]modelsDevModel{
			"gpt-4o": {Cost: modelsDevCost{Input: fp(2), Output: fp(9)}},
		}},
		"freeprov": {Models: map[string]modelsDevModel{
			"free-model": {Cost: modelsDevCost{Input: fp(0), Output: fp(0)}},
		}},
		"ambigA": {Models: map[string]modelsDevModel{
			"weird-model": {Cost: modelsDevCost{Input: fp(1), Output: fp(2)}},
		}},
		"ambigB": {Models: map[string]modelsDevModel{
			"weirdmodel": {Cost: modelsDevCost{Input: fp(9), Output: fp(18)}},
		}},
	}
}

func TestNorm(t *testing.T) {
	cases := map[string]string{
		"claude-opus-4-6":           "claudeopus46",
		"claude-opus-4.8":           "claudeopus48",
		"claude-opus4-8":            "claudeopus48",
		"anthropic/claude-opus-4.8": "claudeopus48",
		"~anthropic/claude-opus-4.8": "claudeopus48",
		"claude-opus-4-6@default":   "claudeopus46",
		"claude-opus-4-6:thinking":  "claudeopus46",
		"GPT-4o":                    "gpt4o",
		"  gpt-4o  ":                "gpt4o",
	}
	for in, want := range cases {
		if got := Norm(in); got != want {
			t.Errorf("Norm(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildBookFirstPartyAndConversion(t *testing.T) {
	b := BuildBook(fixtureProviders(), 1700000000)

	// claude-opus-4-6: anthropic is first-party, beats venice even though venice
	// is more expensive (first-party always wins regardless of price).
	e, ok := b.Entries["claude-opus-4-6"]
	if !ok {
		t.Fatal("missing claude-opus-4-6")
	}
	if e.Provider != "anthropic" || !e.FirstParty {
		t.Errorf("claude-opus-4-6 provider=%q firstParty=%v, want anthropic/true", e.Provider, e.FirstParty)
	}
	if e.ModelRatio != 2.5 {
		t.Errorf("claude-opus-4-6 model_ratio=%v, want 2.5", e.ModelRatio)
	}
	if deref(e.CompletionRatio) != 5 {
		t.Errorf("completion=%v, want 5", deref(e.CompletionRatio))
	}
	if deref(e.CacheRatio) != 0.1 {
		t.Errorf("cache=%v, want 0.1", deref(e.CacheRatio))
	}
	if deref(e.CreateCacheRatio) != 1.25 {
		t.Errorf("create_cache=%v, want 1.25", deref(e.CreateCacheRatio))
	}

	// gpt-4o: openai is first-party, must be chosen over the cheaper reseller.
	g := b.Entries["gpt-4o"]
	if g.Provider != "openai" || !g.FirstParty {
		t.Errorf("gpt-4o provider=%q firstParty=%v, want openai/true", g.Provider, g.FirstParty)
	}
	if g.ModelRatio != 1.25 {
		t.Errorf("gpt-4o model_ratio=%v, want 1.25", g.ModelRatio)
	}

	// non-first-party-only name: falls back, flagged non-first-party.
	v := b.Entries["anthropic/claude-opus-4.8"]
	if v.FirstParty || v.Provider != "openrouter" {
		t.Errorf("anthropic/claude-opus-4.8 provider=%q firstParty=%v, want openrouter/false", v.Provider, v.FirstParty)
	}

	// free model: zero ratio, no derived optionals.
	f := b.Entries["free-model"]
	if f.ModelRatio != 0 || f.CompletionRatio != nil {
		t.Errorf("free-model = %+v, want zero ratio and nil optionals", f)
	}
}

func TestMatch(t *testing.T) {
	b := BuildBook(fixtureProviders(), 1)

	if e, mt, ok := Match(b, "claude-opus-4-6"); !ok || mt != "exact" || e.Provider != "anthropic" {
		t.Errorf("exact match failed: ok=%v mt=%q provider=%q", ok, mt, e.Provider)
	}
	// normalized: "claude-opus-4.8" -> the openrouter variant
	if e, mt, ok := Match(b, "claude-opus-4.8"); !ok || mt != "normalized" || e.ModelRatio != 2.5 {
		t.Errorf("normalized match failed: ok=%v mt=%q ratio=%v", ok, mt, e.ModelRatio)
	}
	// ambiguous normalized key -> refuse to guess
	if _, _, ok := Match(b, "weird_model"); ok {
		t.Error("expected ambiguous 'weird_model' to be unmatched")
	}
	// unknown
	if _, _, ok := Match(b, "totally-unknown-zzz"); ok {
		t.Error("expected unknown model to be unmatched")
	}
}

func TestBuildPreviewMissingOnly(t *testing.T) {
	b := BuildBook(fixtureProviders(), 1)
	cur := CurrentRatios{
		ModelRatio:       map[string]float64{"gpt-4o": 1.25},
		CompletionRatio:  map[string]float64{},
		CacheRatio:       map[string]float64{},
		CreateCacheRatio: map[string]float64{},
	}
	res := BuildPreview(b, []string{"claude-opus-4-6", "gpt-4o", "weird_model", "totally-unknown"}, cur, ModeMissingOnly)

	actions := map[string]string{}
	for _, r := range res.Rows {
		actions[r.Model] = r.Action
	}
	if actions["claude-opus-4-6"] != ActionAdd {
		t.Errorf("claude-opus-4-6 action=%q, want add", actions["claude-opus-4-6"])
	}
	if actions["gpt-4o"] != ActionSkip {
		t.Errorf("gpt-4o action=%q, want skip (already has ratio)", actions["gpt-4o"])
	}
	if len(res.Unmatched) != 2 {
		t.Errorf("unmatched=%v, want 2 (weird_model, totally-unknown)", res.Unmatched)
	}
}

func TestBuildPreviewRefreshLatest(t *testing.T) {
	b := BuildBook(fixtureProviders(), 1)
	// gpt-4o ratio already matches official but completion/cache are missing,
	// so refresh should still mark it for update.
	cur := CurrentRatios{
		ModelRatio:       map[string]float64{"gpt-4o": 1.25},
		CompletionRatio:  map[string]float64{},
		CacheRatio:       map[string]float64{},
		CreateCacheRatio: map[string]float64{},
	}
	res := BuildPreview(b, []string{"gpt-4o"}, cur, ModeRefreshLatest)
	if len(res.Rows) != 1 || res.Rows[0].Action != ActionUpdate {
		t.Errorf("refresh gpt-4o rows=%+v, want single update", res.Rows)
	}
}

func TestApplyToMapsMergePreservesExisting(t *testing.T) {
	b := BuildBook(fixtureProviders(), 1)
	cur := CurrentRatios{
		ModelRatio:       map[string]float64{"existing-model": 9, "gpt-4o": 1.25},
		CompletionRatio:  map[string]float64{},
		CacheRatio:       map[string]float64{},
		CreateCacheRatio: map[string]float64{},
	}
	next, changed, applied := ApplyToMaps(b, []string{"claude-opus-4-6", "gpt-4o", "totally-unknown"}, cur)

	// The pre-existing unrelated key MUST survive (the production bug we fixed).
	if next.ModelRatio["existing-model"] != 9 {
		t.Errorf("existing-model lost: got %v, want 9", next.ModelRatio["existing-model"])
	}
	if next.ModelRatio["claude-opus-4-6"] != 2.5 {
		t.Errorf("claude-opus-4-6 ratio=%v, want 2.5", next.ModelRatio["claude-opus-4-6"])
	}
	if next.CompletionRatio["claude-opus-4-6"] != 5 || next.CacheRatio["claude-opus-4-6"] != 0.1 || next.CreateCacheRatio["claude-opus-4-6"] != 1.25 {
		t.Errorf("claude-opus-4-6 optionals not written: %+v", next)
	}
	// gpt-4o has no cache_write in fixture -> create_cache must stay unset.
	if _, ok := next.CreateCacheRatio["gpt-4o"]; ok {
		t.Error("gpt-4o create_cache should be absent (no cache_write in source)")
	}
	if !changed[KeyModelRatio] || !changed[KeyCompletionRatio] || !changed[KeyCacheRatio] || !changed[KeyCreateCacheRatio] {
		t.Errorf("changed flags incomplete: %+v", changed)
	}
	// totally-unknown is skipped; applied is sorted.
	if len(applied) != 2 || applied[0] != "claude-opus-4-6" || applied[1] != "gpt-4o" {
		t.Errorf("applied=%v, want [claude-opus-4-6 gpt-4o]", applied)
	}
}

func TestApplyToMapsNoChangeWhenIdentical(t *testing.T) {
	b := BuildBook(fixtureProviders(), 1)
	// Pre-seed exactly the official values; applying again must report no change.
	cur := CurrentRatios{
		ModelRatio:       map[string]float64{"gpt-4o": 1.25},
		CompletionRatio:  map[string]float64{"gpt-4o": 4},
		CacheRatio:       map[string]float64{"gpt-4o": 0.5},
		CreateCacheRatio: map[string]float64{},
	}
	_, changed, _ := ApplyToMaps(b, []string{"gpt-4o"}, cur)
	if len(changed) != 0 {
		t.Errorf("expected no changes, got %+v", changed)
	}
}

func TestParseModelsDev(t *testing.T) {
	raw := []byte(`{"anthropic":{"models":{"claude-opus-4-8":{"cost":{"input":5,"output":25,"cache_read":0.5,"cache_write":6.25}}}}}`)
	providers, err := ParseModelsDev(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	cost := providers["anthropic"].Models["claude-opus-4-8"].Cost
	if cost.Input == nil || *cost.Input != 5 || cost.CacheWrite == nil || *cost.CacheWrite != 6.25 {
		t.Errorf("parsed cost wrong: %+v", cost)
	}
}
