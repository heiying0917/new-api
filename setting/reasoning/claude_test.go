package reasoning

import "testing"

func TestClaudeCapabilitiesFor(t *testing.T) {
	cases := []struct {
		model                      string
		adaptive, manual           bool
		strictSampling             bool
		supportsXHigh, supportsMax bool
	}{
		{"claude-fable-5-1", true, false, true, true, false},
		{"claude-fable-5", true, false, true, true, false},
		{"claude-mythos-5-1", true, false, true, true, false},
		{"claude-mythos-preview", true, true, true, false, true},
		{"claude-opus-5", true, false, true, true, true},
		{"claude-sonnet-5", true, false, true, true, true},
		{"claude-opus-4-8", true, false, true, true, true},
		{"claude-opus-4-7", true, false, true, true, true},
		{"claude-opus-4-6", true, true, false, false, true},
		{"claude-sonnet-4-6", true, true, false, false, true},
		{"claude-sonnet-4-5-20250929", false, true, false, false, false},
		{"claude-haiku-4-5", false, true, false, false, false},
		{"CLAUDE-OPUS-5-high", true, false, true, true, true}, // case-insensitive, suffix tolerated
	}
	for _, c := range cases {
		got := ClaudeCapabilitiesFor(c.model)
		if got.Adaptive != c.adaptive || got.ManualThinking != c.manual || got.StrictSampling != c.strictSampling ||
			got.SupportsXHigh != c.supportsXHigh || got.SupportsMax != c.supportsMax {
			t.Errorf("%s: got %+v", c.model, got)
		}
	}
}

func TestNormalizeClaudeEffort(t *testing.T) {
	cases := []struct{ model, in, want string }{
		{"claude-fable-5-1", "max", "xhigh"}, // Fable has no max level
		{"claude-opus-4-6", "xhigh", "max"},  // 4.6 has max but no xhigh
		{"claude-opus-5", "max", "max"},
		{"claude-opus-5", "xhigh", "xhigh"},
		{"claude-sonnet-5", "low", "low"},
	}
	for _, c := range cases {
		if got := NormalizeClaudeEffort(c.model, c.in); got != c.want {
			t.Errorf("%s %s -> %q, want %q", c.model, c.in, got, c.want)
		}
	}
}
