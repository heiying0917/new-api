package reasoning

import "strings"

// ClaudeCapabilities describes which thinking / effort controls a Claude model
// accepts. It mirrors upstream relaykit/relayconvert/reasoning/claude.go
// (claudeCapabilitiesFor) so the relay stops hard-coding version prefixes.
type ClaudeCapabilities struct {
	// Adaptive: accepts thinking.type="adaptive" + output_config.effort.
	Adaptive bool
	// ManualThinking: accepts thinking.type="enabled" + budget_tokens.
	// Opus 4.7+ and every Claude 5 model reject it with 400.
	ManualThinking bool
	// StrictSampling: rejects non-default temperature/top_p/top_k with 400 when
	// thinking is on, and defaults thinking.display to "omitted".
	StrictSampling bool
	SupportsXHigh  bool
	SupportsMax    bool
}

// ClaudeCapabilitiesFor returns the capability table for a Claude model name.
// Effort / -thinking suffixes may still be attached; matching is by prefix.
func ClaudeCapabilitiesFor(model string) ClaudeCapabilities {
	model = strings.ToLower(model)
	caps := ClaudeCapabilities{ManualThinking: true}
	switch {
	case strings.HasPrefix(model, "claude-fable-5"),
		strings.HasPrefix(model, "claude-mythos-5"):
		caps.Adaptive, caps.ManualThinking, caps.StrictSampling = true, false, true
		caps.SupportsXHigh, caps.SupportsMax = true, false
	case strings.HasPrefix(model, "claude-mythos-preview"):
		caps.Adaptive, caps.StrictSampling, caps.SupportsMax = true, true, true
	case strings.HasPrefix(model, "claude-opus-5"),
		strings.HasPrefix(model, "claude-sonnet-5"),
		strings.HasPrefix(model, "claude-opus-4-8"),
		strings.HasPrefix(model, "claude-opus-4-7"):
		caps.Adaptive, caps.ManualThinking, caps.StrictSampling = true, false, true
		caps.SupportsXHigh, caps.SupportsMax = true, true
	case strings.HasPrefix(model, "claude-opus-4-6"),
		strings.HasPrefix(model, "claude-sonnet-4-6"):
		caps.Adaptive, caps.SupportsMax = true, true
	}
	return caps
}

// NormalizeClaudeEffort maps an effort level onto one the model accepts.
// "xhigh" and "max" are documented as equivalent, so swap when only one of
// them is supported; every other level passes through unchanged.
func NormalizeClaudeEffort(model, effort string) string {
	caps := ClaudeCapabilitiesFor(model)
	switch effort {
	case "max":
		if !caps.SupportsMax && caps.SupportsXHigh {
			return "xhigh"
		}
	case "xhigh":
		if !caps.SupportsXHigh && caps.SupportsMax {
			return "max"
		}
	}
	return effort
}
