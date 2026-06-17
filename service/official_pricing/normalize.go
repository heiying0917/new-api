package official_pricing

import (
	"sort"
	"strings"
)

const floatEpsilon = 1e-9

func nearlyEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < floatEpsilon
}

// Norm canonicalizes a model name for fuzzy matching: it drops a leading
// "provider/" prefix, any "@..." or ":..." suffix, and all non-alphanumeric
// characters. So "anthropic/claude-opus-4.8", "claude-opus-4-8" and
// "claude-opus4-8" all normalize to "claudeopus48".
func Norm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.IndexAny(s, "@:"); i >= 0 {
		s = s[:i]
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// EnsureIndex builds the normalized lookup index from Entries. Safe to call
// repeatedly; required after loading a Book from persistence.
func (b *Book) EnsureIndex() {
	groups := make(map[string][]BookEntry)
	for _, e := range b.Entries {
		nk := Norm(e.Model)
		if nk == "" {
			continue
		}
		groups[nk] = append(groups[nk], e)
	}
	idx := make(map[string]normHit, len(groups))
	for nk, es := range groups {
		idx[nk] = chooseNormHit(es)
	}
	b.normIndex = idx
}

// chooseNormHit decides the representative entry for a normalized-key group.
// A first-party entry is authoritative (never ambiguous). Without a first-party
// entry, conflicting prices make the key ambiguous so matching refuses to guess.
func chooseNormHit(es []BookEntry) normHit {
	// Deterministic order.
	sort.Slice(es, func(i, j int) bool { return es[i].Model < es[j].Model })

	if len(es) == 1 {
		return normHit{entry: es[0]}
	}

	var firstParty []BookEntry
	for _, e := range es {
		if e.FirstParty {
			firstParty = append(firstParty, e)
		}
	}
	if len(firstParty) > 0 {
		// Pick cheapest first-party deterministically; first-party is trusted.
		best := firstParty[0]
		for _, e := range firstParty[1:] {
			if e.ModelRatio < best.ModelRatio {
				best = e
			}
		}
		return normHit{entry: best}
	}

	// No first-party: ambiguous if model ratios disagree.
	best := es[0]
	ambiguous := false
	for _, e := range es[1:] {
		if !nearlyEqual(e.ModelRatio, best.ModelRatio) {
			ambiguous = true
		}
		if e.ModelRatio < best.ModelRatio {
			best = e
		}
	}
	return normHit{entry: best, ambiguous: ambiguous}
}

// Match resolves a local model name against the book, always preferring a
// first-party (official) price. Tiers, in order:
//
//	1. exact name match that is first-party
//	2. normalized match that is first-party (so a reseller-styled input like
//	   "anthropic/claude-opus-4.8" still resolves to anthropic's official price)
//	3. exact name match (non-first-party reseller)
//	4. non-ambiguous normalized match (non-first-party)
//
// Returns the entry, the match type ("exact"/"normalized") and whether found.
func Match(b *Book, model string) (BookEntry, string, bool) {
	if b.normIndex == nil {
		b.EnsureIndex()
	}

	exact, hasExact := b.Entries[model]
	if hasExact && exact.FirstParty {
		return exact, "exact", true
	}

	var norm *normHit
	if nk := Norm(model); nk != "" {
		if h, ok := b.normIndex[nk]; ok {
			norm = &h
		}
	}
	if norm != nil && !norm.ambiguous && norm.entry.FirstParty {
		return norm.entry, "normalized", true
	}
	if hasExact {
		return exact, "exact", true
	}
	if norm != nil && !norm.ambiguous {
		return norm.entry, "normalized", true
	}
	return BookEntry{}, "", false
}
