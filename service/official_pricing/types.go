package official_pricing

// Package official_pricing builds an "official price book" from models.dev and
// matches local model names against it so administrators can fill official
// prices into the global ratio settings with one click.
//
// This package contains only pure logic (parsing, conversion, normalization,
// matching, fill diffing). All database / option / ratio_setting access lives
// in controller/official_pricing.go so this package stays trivially testable.

// BookEntry is one model's official pricing, already converted into new-api
// ratio units. Optional fields use pointers: nil means "models.dev did not
// provide it" and the fill step must not write it.
type BookEntry struct {
	Model            string   `json:"model"`
	Provider         string   `json:"provider"`     // the models.dev provider this price came from
	FirstParty       bool     `json:"first_party"`  // true if Provider is the model's first-party vendor
	ModelRatio       float64  `json:"model_ratio"`  // always present for token-priced models
	CompletionRatio  *float64 `json:"completion_ratio,omitempty"`
	CacheRatio       *float64 `json:"cache_ratio,omitempty"`
	CreateCacheRatio *float64 `json:"create_cache_ratio,omitempty"`
}

// Book is the cached official price book. Entries is keyed by the raw model
// name as it appears in models.dev. normIndex is derived (not serialized) and
// must be rebuilt via EnsureIndex after loading from persistence.
type Book struct {
	Source    string               `json:"source"`     // e.g. "models.dev"
	FetchedAt int64                `json:"fetched_at"` // unix seconds
	Entries   map[string]BookEntry `json:"entries"`

	normIndex map[string]normHit `json:"-"`
}

// normHit is a precomputed normalized-key lookup result. ambiguous=true means
// several distinct prices collide on the same normalized key with no
// first-party tiebreaker, so matching must refuse to guess.
type normHit struct {
	entry     BookEntry
	ambiguous bool
}

// CurrentRatios is a snapshot of the live global ratio maps, passed into the
// pure fill functions so they need no ratio_setting/model dependency.
type CurrentRatios struct {
	ModelRatio       map[string]float64
	CompletionRatio  map[string]float64
	CacheRatio       map[string]float64
	CreateCacheRatio map[string]float64
}

// Meta is the lightweight summary returned to the frontend (never the full
// entries map, which is large).
type Meta struct {
	Source          string `json:"source"`
	FetchedAt       int64  `json:"fetched_at"`
	ModelCount      int    `json:"model_count"`
	FirstPartyCount int    `json:"first_party_count"`
}

// Meta returns a summary of the book.
func (b *Book) Meta() Meta {
	m := Meta{Source: b.Source, FetchedAt: b.FetchedAt, ModelCount: len(b.Entries)}
	for _, e := range b.Entries {
		if e.FirstParty {
			m.FirstPartyCount++
		}
	}
	return m
}
