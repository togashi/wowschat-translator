package translator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ClaudeTranslator struct {
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

type claudeMessagesRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	System      string          `json:"system"`
	Messages    []claudeMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeMessagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// NewClaudeTranslator creates a ClaudeTranslator.
func NewClaudeTranslator(
	apiKey,
	model string,
	promptFile string,
	temperature float64,
	outputFormat string,
	passthrough []string,
	glossary map[string]string,
	expand map[string]string,
	debug bool,
) *ClaudeTranslator {
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	if temperature == 0 {
		temperature = 0.2
	}
	return &ClaudeTranslator{
		apiKey:       apiKey,
		baseURL:      "https://api.anthropic.com/v1",
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
func (t *ClaudeTranslator) SetTraceSink(sink func(TranslatorTraceEvent)) {
	t.traceSink = sink
}

// Translate translates text with the Anthropic Messages API.
func (t *ClaudeTranslator) Translate(text, targetLang string) (string, error) {
	totalStart := time.Now()
	var requestElapsed time.Duration
	var statusCode int
	defer func() {
		t.debugf("timing request_ms=%d total_ms=%d status=%d", requestElapsed.Milliseconds(), time.Since(totalStart).Milliseconds(), statusCode)
		t.trace("claude", "timing", "request finished", map[string]any{
			"request_ms": requestElapsed.Milliseconds(),
			"total_ms":   time.Since(totalStart).Milliseconds(),
			"status":     statusCode,
		})
	}()

	t.debugf("translate start model=%s temp=%.3f target=%s text_len=%d", t.model, t.temperature, targetLang, len(text))
	t.trace("claude", "input", text, nil)

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
		t.trace("claude", "skip", "passthrough-only text", map[string]any{
			"masked_segments": len(segments),
		})
		return "", nil
	}

	reqBody := claudeMessagesRequest{
		Model:     t.model,
		MaxTokens: 256,
		System:    t.buildSystemPrompt(),
		Messages: []claudeMessage{
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

	req, err := http.NewRequest(http.MethodPost, t.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", t.apiKey)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	requestStart := time.Now()
	resp, err := t.httpClient.Do(req)
	requestElapsed = time.Since(requestStart)
	if err != nil {
		return "", fmt.Errorf("Anthropic request: %w", err)
	}
	defer resp.Body.Close()
	statusCode = resp.StatusCode
	t.debugf("messages status=%d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Anthropic API status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var apiResp claudeMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("Anthropic response: %w", err)
	}

	responseText := strings.TrimSpace(extractClaudeOutputText(apiResp))
	if responseText == "" {
		t.debugf("messages output is empty")
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
		if translationResult.TranslationNote != "" {
			t.debugf("translation note: %s", translationResult.TranslationNote)
		}
		return "", nil
	}
	t.debugf("translated source=%s translated_len=%d", translationResult.SourceLang, len(translationResult.Text))
	if translationResult.TranslationNote != "" {
		t.debugf("translation note: %s", translationResult.TranslationNote)
	}
	t.trace("claude", "output", translationResult.Text, map[string]any{
		"source_lang": translationResult.SourceLang,
	})

	return formatOutput(t.outputFormat, translationResult.SourceLang, targetLang, text, translationResult.Text), nil
}

func extractClaudeOutputText(apiResp claudeMessagesResponse) string {
	for _, block := range apiResp.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return block.Text
		}
	}
	return ""
}

func (t *ClaudeTranslator) buildSystemPrompt() string {
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

func (t *ClaudeTranslator) hasActiveExternalPrompt() bool {
	t.promptMu.RLock()
	active := t.promptPath != ""
	t.promptMu.RUnlock()
	return active
}

func (t *ClaudeTranslator) getSystemPrompt() string {
	return getSystemPromptFromFileOrDefault(
		t.promptFile,
		embeddedSystemPrompt,
		&t.promptMu,
		&t.promptCached,
		&t.promptValue,
		&t.promptPath,
		&t.promptModTime,
		func(format string, args ...any) { t.debugf(format, args...) },
	)
}

func (t *ClaudeTranslator) getPassthroughRules() []passthroughRule {
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

func (t *ClaudeTranslator) debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	t.trace("claude", "debug", msg, nil)

	if !t.debug {
		return
	}
	log.Printf("[DEBUG][Claude] %s", msg)
}

func (t *ClaudeTranslator) trace(engine, stage, message string, fields map[string]any) {
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
