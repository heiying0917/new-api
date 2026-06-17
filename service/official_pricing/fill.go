package official_pricing

import "sort"

// Fill modes.
const (
	ModeMissingOnly   = "missing_only"   // only fill models that currently have no model_ratio
	ModeRefreshLatest = "refresh_latest" // update matched models whose price differs from official
)

// Fill actions (per row).
const (
	ActionAdd    = "add"
	ActionUpdate = "update"
	ActionSkip   = "skip"
)

// RatioSet is a partial set of ratio values for one model. Pointer fields are
// omitted when absent.
type RatioSet struct {
	ModelRatio       *float64 `json:"model_ratio,omitempty"`
	CompletionRatio  *float64 `json:"completion_ratio,omitempty"`
	CacheRatio       *float64 `json:"cache_ratio,omitempty"`
	CreateCacheRatio *float64 `json:"create_cache_ratio,omitempty"`
}

// PreviewRow describes one matched target model and the proposed change.
type PreviewRow struct {
	Model      string    `json:"model"`
	MatchType  string    `json:"match_type"`
	Provider   string    `json:"provider"`
	FirstParty bool      `json:"first_party"`
	Action     string    `json:"action"`
	Official   *RatioSet `json:"official"`
	Current    *RatioSet `json:"current"`
}

// PreviewResult is the full preview payload: matched rows plus the names that
// could not be matched against the book.
type PreviewResult struct {
	Rows      []PreviewRow `json:"rows"`
	Unmatched []string     `json:"unmatched"`
}

func dedupSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func officialRatioSet(e BookEntry) *RatioSet {
	mr := e.ModelRatio
	return &RatioSet{
		ModelRatio:       &mr,
		CompletionRatio:  e.CompletionRatio,
		CacheRatio:       e.CacheRatio,
		CreateCacheRatio: e.CreateCacheRatio,
	}
}

// currentRatioSet collects whatever the live maps already hold for a model.
func currentRatioSet(cur CurrentRatios, model string) *RatioSet {
	rs := &RatioSet{}
	if v, ok := cur.ModelRatio[model]; ok {
		vv := v
		rs.ModelRatio = &vv
	}
	if v, ok := cur.CompletionRatio[model]; ok {
		vv := v
		rs.CompletionRatio = &vv
	}
	if v, ok := cur.CacheRatio[model]; ok {
		vv := v
		rs.CacheRatio = &vv
	}
	if v, ok := cur.CreateCacheRatio[model]; ok {
		vv := v
		rs.CreateCacheRatio = &vv
	}
	return rs
}

func ptrDiffers(official, current *float64) bool {
	if official == nil {
		return false // nothing official to write -> not a difference we care about
	}
	if current == nil {
		return true
	}
	return !nearlyEqual(*official, *current)
}

// officialDiffersFromCurrent reports whether applying the official set would
// change any currently-stored value.
func officialDiffersFromCurrent(official, current *RatioSet) bool {
	return ptrDiffers(official.ModelRatio, current.ModelRatio) ||
		ptrDiffers(official.CompletionRatio, current.CompletionRatio) ||
		ptrDiffers(official.CacheRatio, current.CacheRatio) ||
		ptrDiffers(official.CreateCacheRatio, current.CreateCacheRatio)
}

// BuildPreview matches each target model against the book and computes the
// proposed action under the given mode. Unmatched targets are collected
// separately so the caller can surface them for manual handling.
func BuildPreview(book *Book, targets []string, cur CurrentRatios, mode string) PreviewResult {
	res := PreviewResult{Rows: []PreviewRow{}, Unmatched: []string{}}
	for _, t := range dedupSorted(targets) {
		entry, matchType, ok := Match(book, t)
		if !ok {
			res.Unmatched = append(res.Unmatched, t)
			continue
		}
		official := officialRatioSet(entry)
		current := currentRatioSet(cur, t)

		action := ActionSkip
		_, hasRatio := cur.ModelRatio[t]
		switch mode {
		case ModeRefreshLatest:
			if officialDiffersFromCurrent(official, current) {
				if hasRatio {
					action = ActionUpdate
				} else {
					action = ActionAdd
				}
			}
		default: // ModeMissingOnly
			if !hasRatio {
				action = ActionAdd
			}
		}

		res.Rows = append(res.Rows, PreviewRow{
			Model:      t,
			MatchType:  matchType,
			Provider:   entry.Provider,
			FirstParty: entry.FirstParty,
			Action:     action,
			Official:   official,
			Current:    current,
		})
	}
	return res
}

func cloneFloatMap(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func setIfChanged(m map[string]float64, key string, val float64, changed *bool) {
	if old, ok := m[key]; !ok || !nearlyEqual(old, val) {
		m[key] = val
		*changed = true
	}
}

// Apply option-map keys (match the global option keys / ratio_setting maps).
const (
	KeyModelRatio       = "ModelRatio"
	KeyCompletionRatio  = "CompletionRatio"
	KeyCacheRatio       = "CacheRatio"
	KeyCreateCacheRatio = "CreateCacheRatio"
)

// ApplyToMaps merges official prices for the given models into copies of the
// current ratio maps. It writes a model's optional ratios only when the book
// provides them, and reports which option keys actually changed (so the caller
// persists only those). Existing keys for other models are preserved.
func ApplyToMaps(book *Book, models []string, cur CurrentRatios) (next CurrentRatios, changed map[string]bool, applied []string) {
	next = CurrentRatios{
		ModelRatio:       cloneFloatMap(cur.ModelRatio),
		CompletionRatio:  cloneFloatMap(cur.CompletionRatio),
		CacheRatio:       cloneFloatMap(cur.CacheRatio),
		CreateCacheRatio: cloneFloatMap(cur.CreateCacheRatio),
	}
	changed = map[string]bool{}
	applied = []string{}

	for _, m := range dedupSorted(models) {
		entry, _, ok := Match(book, m)
		if !ok {
			continue
		}
		var mrChanged, crChanged, caChanged, ccChanged bool
		setIfChanged(next.ModelRatio, m, entry.ModelRatio, &mrChanged)
		if entry.CompletionRatio != nil {
			setIfChanged(next.CompletionRatio, m, *entry.CompletionRatio, &crChanged)
		}
		if entry.CacheRatio != nil {
			setIfChanged(next.CacheRatio, m, *entry.CacheRatio, &caChanged)
		}
		if entry.CreateCacheRatio != nil {
			setIfChanged(next.CreateCacheRatio, m, *entry.CreateCacheRatio, &ccChanged)
		}
		if mrChanged {
			changed[KeyModelRatio] = true
		}
		if crChanged {
			changed[KeyCompletionRatio] = true
		}
		if caChanged {
			changed[KeyCacheRatio] = true
		}
		if ccChanged {
			changed[KeyCreateCacheRatio] = true
		}
		applied = append(applied, m)
	}
	return next, changed, applied
}
