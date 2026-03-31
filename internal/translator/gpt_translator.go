package translator

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type GPTTranslator struct {
	apiKey        string
	baseURL       string
	model         string
	promptFile    string
	temperature   float64
	httpClient    *http.Client
	outputFormat  string
	passthrough   []string
	glossary      map[string]string
	expand        map[string]string
	debug         bool
	traceSink     func(TranslatorTraceEvent)
	promptMu      sync.RWMutex
	promptCached  bool
	promptValue   string
	promptPath    string
	promptModTime time.Time
	rulesMu       sync.RWMutex
	rulesCached   bool
	rulesValue    []passthroughRule
}

type passthroughRule struct {
	kind    string
	value   string
	pattern *regexp.Regexp
}

type maskedSegment struct {
	placeholder string
	original    string
}

type openAIResponsesRequest struct {
	Model       string       `json:"model"`
	Input       []gptMessage `json:"input"`
	Temperature float64      `json:"temperature"`
}

type gptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponsesResponse struct {
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

const (
	promptPlaceholderPassthrough = "{{PASSTHROUGH}}"
	promptPlaceholderGlossary    = "{{GLOSSARY}}"
)

// NewGPTTranslator creates a GPTTranslator.
func NewGPTTranslator(
	apiKey,
	model string,
	promptFile string,
	temperature float64,
	outputFormat string,
	passthrough []string,
	glossary map[string]string,
	expand map[string]string,
	debug bool,
) *GPTTranslator {
	if model == "" {
		model = "gpt-5.4-mini"
	}
	if temperature == 0 {
		temperature = 0.2
	}
	return &GPTTranslator{
		apiKey:       apiKey,
		baseURL:      "https://api.openai.com/v1",
		model:        model,
		promptFile:   promptFile,
		temperature:  temperature,
		outputFormat: outputFormat,
		passthrough:  passthrough,
		glossary:     glossary,
		expand:       expand,
		debug:        debug,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetTraceSink sets an optional hook for collecting translator trace events.
func (t *GPTTranslator) SetTraceSink(sink func(TranslatorTraceEvent)) {
	t.traceSink = sink
}

//go:embed prompts/gpt_system_prompt.txt
var embeddedGPTSystemPrompt string

type gptTranslationResult struct {
	Text            string `json:"text"`
	SourceLang      string `json:"source_lang"`
	TranslationNote string `json:"translation_note,omitempty"`
}

// Translate translates text with OpenAI Responses API.
func (t *GPTTranslator) Translate(text, targetLang string) (string, error) {
	totalStart := time.Now()
	var requestElapsed time.Duration
	var statusCode int
	defer func() {
		t.debugf("timing request_ms=%d total_ms=%d status=%d", requestElapsed.Milliseconds(), time.Since(totalStart).Milliseconds(), statusCode)
		t.trace("gpt", "timing", "request finished", map[string]any{
			"request_ms": requestElapsed.Milliseconds(),
			"total_ms":   time.Since(totalStart).Milliseconds(),
			"status":     statusCode,
		})
	}()

	t.debugf("translate start model=%s temp=%.3f target=%s text_len=%d", t.model, t.temperature, targetLang, len(text))

	expanded := applyExpand(text, t.expand)
	if expanded != text {
		t.debugf("expand applied: %q -> %q", text, expanded)
	}

	rules := t.getPassthroughRules()
	maskedText, segments := applyPassthroughRules(expanded, rules)
	t.debugf("passthrough rules=%d masked_segments=%d", len(rules), len(segments))
	t.debugf("llm input: %q", maskedText)
	if isPassthroughOnlyMaskedText(maskedText) {
		t.debugf("skip translation passthrough-only text")
		t.trace("gpt", "skip", "passthrough-only text", map[string]any{
			"masked_segments": len(segments),
		})
		return "", nil
	}

	reqBody := openAIResponsesRequest{
		Model: t.model,
		Input: []gptMessage{
			{
				Role:    "system",
				Content: t.buildSystemPrompt(),
			},
			{
				Role: "user",
				Content: fmt.Sprintf(
					"Target language: %s\nText: %s",
					targetLang,
					maskedText,
				),
			},
		},
		Temperature: t.temperature,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, t.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")

	requestStart := time.Now()
	resp, err := t.httpClient.Do(req)
	requestElapsed = time.Since(requestStart)
	if err != nil {
		return "", fmt.Errorf("OpenAI request: %w", err)
	}
	defer resp.Body.Close()
	statusCode = resp.StatusCode
	t.debugf("responses status=%d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var apiResp openAIResponsesResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("OpenAI response: %w", err)
	}

	responseText := strings.TrimSpace(extractResponsesOutputText(apiResp))
	if responseText == "" {
		t.debugf("responses output is empty")
		return "", nil
	}

	translationResult, err := parseTranslationResult(responseText)
	if err != nil {
		t.debugf("parse translation result failed: %v", err)
		return "", err
	}

	translationResult.Text = restoreMaskedSegments(translationResult.Text, segments)

	if strings.EqualFold(translationResult.SourceLang, targetLang) || translationResult.Text == text {
		t.debugf("skip translation source=%s target=%s unchanged=%t", translationResult.SourceLang, targetLang, translationResult.Text == text)
		t.debugf("translation note: %s", translationResult.TranslationNote)
		return "", nil
	}
	t.debugf("translated source=%s translated_len=%d", translationResult.SourceLang, len(translationResult.Text))

	return formatOutput(t.outputFormat, translationResult.SourceLang, targetLang, text, translationResult.Text), nil
}

func (t *GPTTranslator) buildSystemPrompt() string {
	prompt := t.getSystemPrompt()

	passthroughBlock := buildPassthroughPromptBlock(t.passthrough)
	glossaryBlock := buildGlossaryPromptBlock(t.glossary)

	hasPassthroughPlaceholder := strings.Contains(prompt, promptPlaceholderPassthrough)
	hasGlossaryPlaceholder := strings.Contains(prompt, promptPlaceholderGlossary)

	prompt = strings.ReplaceAll(prompt, promptPlaceholderPassthrough, passthroughBlock)
	prompt = strings.ReplaceAll(prompt, promptPlaceholderGlossary, glossaryBlock)

	if t.hasActiveExternalPrompt() {
		return prompt
	}

	if passthroughBlock != "" && !hasPassthroughPlaceholder {
		prompt += passthroughBlock
	}

	if glossaryBlock != "" && !hasGlossaryPlaceholder {
		prompt += glossaryBlock
	}

	return prompt
}

func (t *GPTTranslator) hasActiveExternalPrompt() bool {
	t.promptMu.RLock()
	active := t.promptPath != ""
	t.promptMu.RUnlock()
	return active
}

func buildPassthroughPromptBlock(items []string) string {
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\nPassthrough words/phrases (keep as-is):\n")
	count := 0
	for _, s := range items {
		if strings.TrimSpace(s) == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteString("\n")
		count++
	}

	if count == 0 {
		return ""
	}
	return b.String()
}

func buildGlossaryPromptBlock(glossary map[string]string) string {
	if len(glossary) == 0 {
		return ""
	}

	keys := make([]string, 0, len(glossary))
	for src := range glossary {
		keys = append(keys, src)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("\nGlossary (source -> preferred target):\n")
	count := 0
	for _, src := range keys {
		dst := glossary[src]
		if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(src)
		b.WriteString(" -> ")
		b.WriteString(dst)
		b.WriteString("\n")
		count++
	}

	if count == 0 {
		return ""
	}
	return b.String()
}

func (t *GPTTranslator) getSystemPrompt() string {
	return getSystemPromptFromFileOrDefault(
		t.promptFile,
		embeddedGPTSystemPrompt,
		&t.promptMu,
		&t.promptCached,
		&t.promptValue,
		&t.promptPath,
		&t.promptModTime,
		func(format string, args ...any) { t.debugf(format, args...) },
	)
}

func (t *GPTTranslator) getPassthroughRules() []passthroughRule {
	t.rulesMu.RLock()
	if t.rulesCached {
		value := t.rulesValue
		t.rulesMu.RUnlock()
		return value
	}
	t.rulesMu.RUnlock()

	rules := buildPassthroughRules(t.passthrough)

	t.rulesMu.Lock()
	if !t.rulesCached {
		t.rulesValue = rules
		t.rulesCached = true
	}
	value := t.rulesValue
	t.rulesMu.Unlock()

	return value
}

func extractResponsesOutputText(apiResp openAIResponsesResponse) string {
	if strings.TrimSpace(apiResp.OutputText) != "" {
		return apiResp.OutputText
	}

	for _, output := range apiResp.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" || content.Type == "text" {
				if strings.TrimSpace(content.Text) != "" {
					return content.Text
				}
			}
		}
	}

	return ""
}

func parseTranslationResult(content string) (*gptTranslationResult, error) {
	var out gptTranslationResult
	if err := json.Unmarshal([]byte(content), &out); err == nil {
		return &out, nil
	}

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(content[start:end+1]), &out); err == nil {
			return &out, nil
		}
	}

	return nil, fmt.Errorf("OpenAI response content is not valid JSON: %q", content)
}

func (t *GPTTranslator) debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	t.trace("gpt", "debug", msg, nil)

	if !t.debug {
		return
	}
	log.Printf("[DEBUG][GPT] %s", msg)
}

func (t *GPTTranslator) trace(engine, stage, message string, fields map[string]any) {
	if t.traceSink == nil {
		return
	}
	t.traceSink(TranslatorTraceEvent{
		Time:    time.Now(),
		Engine:  engine,
		Stage:   stage,
		Message: message,
		Fields:  fields,
	})
}

func applyExpand(text string, expand map[string]string) string {
	if len(expand) == 0 {
		return text
	}
	for abbr, full := range expand {
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(abbr) + `\b`)
		if err != nil {
			continue
		}
		text = re.ReplaceAllString(text, full)
	}
	return text
}

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

var passthroughPlaceholderPattern = regexp.MustCompile(`__PT\d+__`)

func isPassthroughOnlyMaskedText(maskedText string) bool {
	if !strings.Contains(maskedText, "__PT") {
		return false
	}
	remaining := passthroughPlaceholderPattern.ReplaceAllString(maskedText, "")
	return strings.TrimSpace(remaining) == ""
}
