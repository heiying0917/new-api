package official_pricing

import (
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// SourceModelsDev is the canonical source label for the book.
const SourceModelsDev = "models.dev"

// modelsDev JSON shape: top-level is provider_id -> provider.
type modelsDevProvider struct {
	Models map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	Cost modelsDevCost `json:"cost"`
}

// modelsDevCost holds USD-per-1M-token prices. All optional.
type modelsDevCost struct {
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
}

// firstPartyProviders maps a model-name substring to the models.dev provider id
// that is the model's first-party (official) vendor. Order matters: the first
// matching rule wins. Provider ids verified against the live models.dev dataset.
var firstPartyRules = []struct {
	needle   string
	provider string
}{
	{"claude", "anthropic"},
	{"chatgpt", "openai"},
	{"gpt-", "openai"},
	{"gpt", "openai"},
	{"o1", "openai"},
	{"o3", "openai"},
	{"o4", "openai"},
	{"gemini", "google"},
	{"gemma", "google"},
	{"deepseek", "deepseek"},
	{"grok", "xai"},
	{"qwen", "alibaba"},
	{"kimi", "moonshotai"},
	{"glm", "zhipuai"},
	{"mistral", "mistral"},
	{"mixtral", "mistral"},
	{"codestral", "mistral"},
	{"ministral", "mistral"},
	{"magistral", "mistral"},
	{"pixtral", "mistral"},
}

// firstPartyProvider returns the models.dev provider id that is the first-party
// vendor for the given model name, or "" if unknown.
func firstPartyProvider(model string) string {
	m := strings.ToLower(model)
	for _, r := range firstPartyRules {
		if strings.Contains(m, r.needle) {
			return r.provider
		}
	}
	return ""
}

// ParseModelsDev decodes the raw models.dev /api.json payload.
func ParseModelsDev(data []byte) (map[string]modelsDevProvider, error) {
	var providers map[string]modelsDevProvider
	if err := common.Unmarshal(data, &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

func validCost(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type candidate struct {
	provider string
	cost     modelsDevCost
}

// pickCandidate chooses the candidate to represent a model name: the first-party
// provider if present, otherwise the cheapest valid non-zero input price (with a
// deterministic provider-name tiebreak). Returns the chosen candidate and
// whether it is first-party.
func pickCandidate(cands []candidate, firstParty string) (candidate, bool) {
	var best candidate
	bestSet := false
	bestIsFP := false
	for _, c := range cands {
		isFP := firstParty != "" && c.provider == firstParty
		if !bestSet {
			best, bestSet, bestIsFP = c, true, isFP
			continue
		}
		if isFP && !bestIsFP {
			best, bestIsFP = c, true
			continue
		}
		if isFP == bestIsFP {
			// Prefer cheaper non-zero input; fall back to provider-name order.
			ci, bi := *c.cost.Input, *best.cost.Input
			cNonZero, bNonZero := ci > 0, bi > 0
			switch {
			case cNonZero != bNonZero:
				if cNonZero {
					best = c
				}
			case cNonZero && !nearlyEqual(ci, bi):
				if ci < bi {
					best = c
				}
			default:
				if c.provider < best.provider {
					best = c
				}
			}
		}
	}
	return best, bestIsFP
}

// toEntry converts a chosen candidate's USD pricing into a BookEntry in ratio
// units. A zero input baseline yields a zero model_ratio and no derived ratios.
func toEntry(model, provider string, firstParty bool, c modelsDevCost) BookEntry {
	input := *c.Input
	e := BookEntry{Model: model, Provider: provider, FirstParty: firstParty}
	if input == 0 {
		e.ModelRatio = 0
		return e
	}
	e.ModelRatio = round6(input * float64(ratio_setting.USD) / 1000.0)
	if c.Output != nil && validCost(*c.Output) {
		v := round6(*c.Output / input)
		e.CompletionRatio = &v
	}
	if c.CacheRead != nil && validCost(*c.CacheRead) {
		v := round6(*c.CacheRead / input)
		e.CacheRatio = &v
	}
	if c.CacheWrite != nil && validCost(*c.CacheWrite) {
		v := round6(*c.CacheWrite / input)
		e.CreateCacheRatio = &v
	}
	return e
}

// BuildBook converts parsed models.dev providers into an official price book.
// Each distinct raw model name becomes one entry, priced from its first-party
// provider when available. fetchedAt is unix seconds (injected by the caller so
// this stays deterministic / testable).
func BuildBook(providers map[string]modelsDevProvider, fetchedAt int64) *Book {
	grouped := make(map[string][]candidate)
	for _, prov := range sortedKeys(providers) {
		models := providers[prov].Models
		for _, name := range sortedKeys(models) {
			cost := models[name].Cost
			if cost.Input == nil || !validCost(*cost.Input) {
				continue
			}
			grouped[name] = append(grouped[name], candidate{provider: prov, cost: cost})
		}
	}

	entries := make(map[string]BookEntry, len(grouped))
	for name, cands := range grouped {
		chosen, isFP := pickCandidate(cands, firstPartyProvider(name))
		entries[name] = toEntry(name, chosen.provider, isFP, chosen.cost)
	}

	b := &Book{Source: SourceModelsDev, FetchedAt: fetchedAt, Entries: entries}
	b.EnsureIndex()
	return b
}
