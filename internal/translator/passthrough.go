package translator

import (
	"fmt"
	"regexp"
	"strings"
)

type passthroughRule struct {
	kind    string
	value   string
	pattern *regexp.Regexp
}

type maskedSegment struct {
	placeholder string
	original    string
}

var passthroughPlaceholderPattern = regexp.MustCompile(`__PT\d+__`)

func buildPassthroughRules(items []string) []passthroughRule {
	rules := make([]passthroughRule, 0, len(items))
	for _, raw := range items {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		switch {
		case len(raw) >= 2 && raw[0] == '/' && strings.ContainsRune(raw[1:], '/'):
			// regex literal: /pattern/ or /pattern/flags
			lastSlash := strings.LastIndex(raw[1:], "/") + 1
			pattern := raw[1:lastSlash]
			flags := raw[lastSlash+1:]
			if pattern == "" {
				continue
			}
			if flags != "" {
				pattern = "(?" + flags + ")" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			rules = append(rules, passthroughRule{kind: "regex", value: pattern, pattern: re})
		case strings.HasSuffix(raw, "*"):
			value := raw[:len(raw)-1]
			if value == "" {
				continue
			}
			rules = append(rules, passthroughRule{kind: "prefix", value: value})
		default:
			re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(raw) + `\b`)
			if err != nil {
				continue
			}
			rules = append(rules, passthroughRule{kind: "exact", value: raw, pattern: re})
		}
	}
	return rules
}

func applyPassthroughRules(text string, rules []passthroughRule) (string, []maskedSegment) {
	maskedText := text
	segments := make([]maskedSegment, 0)

	mask := func(value string) string {
		placeholder := fmt.Sprintf("__PT%d__", len(segments))
		segments = append(segments, maskedSegment{placeholder: placeholder, original: value})
		return placeholder
	}

	for _, rule := range rules {
		switch rule.kind {
		case "exact":
			if rule.pattern == nil {
				continue
			}
			indexes := rule.pattern.FindAllStringIndex(maskedText, -1)
			for i := len(indexes) - 1; i >= 0; i-- {
				start := indexes[i][0]
				end := indexes[i][1]
				if start < 0 || end <= start || end > len(maskedText) {
					continue
				}
				original := maskedText[start:end]
				placeholder := mask(original)
				maskedText = maskedText[:start] + placeholder + maskedText[end:]
			}
		case "prefix":
			if rule.value == "" || !strings.HasPrefix(maskedText, rule.value) {
				continue
			}
			placeholder := mask(rule.value)
			maskedText = placeholder + strings.TrimPrefix(maskedText, rule.value)
		case "regex":
			if rule.pattern == nil {
				continue
			}
			indexes := rule.pattern.FindAllStringIndex(maskedText, -1)
			for i := len(indexes) - 1; i >= 0; i-- {
				start := indexes[i][0]
				end := indexes[i][1]
				if start < 0 || end <= start || end > len(maskedText) {
					continue
				}
				original := maskedText[start:end]
				placeholder := mask(original)
				maskedText = maskedText[:start] + placeholder + maskedText[end:]
			}
		}
	}

	return maskedText, segments
}

func restoreMaskedSegments(text string, segments []maskedSegment) string {
	if len(segments) == 0 {
		return text
	}

	restored := text
	for _, segment := range segments {
		restored = strings.ReplaceAll(restored, segment.placeholder, segment.original)
	}
	return restored
}

func isPassthroughOnlyMaskedText(maskedText string) bool {
	if !strings.Contains(maskedText, "__PT") {
		return false
	}
	remaining := passthroughPlaceholderPattern.ReplaceAllString(maskedText, "")
	return strings.TrimSpace(remaining) == ""
}
