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

type GeminiTranslator struct {
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

type geminiGenerateRequest struct {
	SystemInstruction *geminiContent        `json:"systemInstruction,omitempty"`
	Contents          []geminiContent       `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature      float64               `json:"temperature"`
	MaxOutputTokens  int                   `json:"maxOutputTokens"`
	ResponseMimeType string                `json:"responseMimeType,omitempty"`
	ThinkingConfig   *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

type geminiThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// NewGeminiTranslator creates a GeminiTranslator.
func NewGeminiTranslator(
	apiKey,
	model string,
	promptFile string,
	temperature float64,
	outputFormat string,
	passthrough []string,
	glossary map[string]string,
	expand map[string]string,
	debug bool,
) *GeminiTranslator {
	if model == "" {
		model = "gemini-2.5-flash"
	}
	if temperature == 0 {
		temperature = 0.2
	}
	return &GeminiTranslator{
		apiKey:       apiKey,
		baseURL:      "https://generativelanguage.googleapis.com/v1beta",
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
func (t *GeminiTranslator) SetTraceSink(sink func(TranslatorTraceEvent)) {
	t.traceSink = sink
}

// Translate translates text with the Gemini generateContent API.
func (t *GeminiTranslator) Translate(text, targetLang string) (string, error) {
	totalStart := time.Now()
	var requestElapsed time.Duration
	var statusCode int
	defer func() {
		t.debugf("timing request_ms=%d total_ms=%d status=%d", requestElapsed.Milliseconds(), time.Since(totalStart).Milliseconds(), statusCode)
		t.trace("gemini", "timing", "request finished", map[string]any{
			"request_ms": requestElapsed.Milliseconds(),
			"total_ms":   time.Since(totalStart).Milliseconds(),
			"status":     statusCode,
		})
	}()

	t.debugf("translate start model=%s temp=%.3f target=%s text_len=%d", t.model, t.temperature, targetLang, len(text))
	t.trace("gemini", "input", text, nil)

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
		t.trace("gemini", "skip", "passthrough-only text", map[string]any{
			"masked_segments": len(segments),
		})
		return "", nil
	}

	systemPrompt := t.buildSystemPrompt()
	reqBody := geminiGenerateRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{{
					Text: fmt.Sprintf(
						"Target language: %s\nText: %s",
						targetLang,
						maskedText,
					),
				}},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:      t.temperature,
			MaxOutputTokens:  512,
			ResponseMimeType: "application/json",
			ThinkingConfig:   &geminiThinkingConfig{ThinkingBudget: 0},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent", t.baseURL, t.model)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-goog-api-key", t.apiKey)
	req.Header.Set("Content-Type", "application/json")

	requestStart := time.Now()
	resp, err := t.httpClient.Do(req)
	requestElapsed = time.Since(requestStart)
	if err != nil {
		return "", fmt.Errorf("Gemini request: %w", err)
	}
	defer resp.Body.Close()
	statusCode = resp.StatusCode
	t.debugf("generateContent status=%d", resp.StatusCode)

	rawBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", fmt.Errorf("Gemini response read: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini API status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	var apiResp geminiGenerateResponse
	if err := json.Unmarshal(rawBody, &apiResp); err != nil {
		t.debugf("raw response body: %s", string(rawBody))
		return "", fmt.Errorf("Gemini response: %w", err)
	}

	finishReason := ""
	if len(apiResp.Candidates) > 0 {
		finishReason = apiResp.Candidates[0].FinishReason
	}
	t.debugf("generateContent finish=%s prompt_tokens=%d output_tokens=%d thoughts_tokens=%d total_tokens=%d",
		finishReason,
		apiResp.UsageMetadata.PromptTokenCount,
		apiResp.UsageMetadata.CandidatesTokenCount,
		apiResp.UsageMetadata.ThoughtsTokenCount,
		apiResp.UsageMetadata.TotalTokenCount,
	)

	responseText := strings.TrimSpace(extractGeminiOutputText(apiResp))
	if responseText == "" {
		t.debugf("generateContent output is empty; raw response body: %s", string(rawBody))
		return "", nil
	}

	translationResult, err := parseTranslationResult(responseText)
	if err != nil {
		t.debugf("parse translation result failed: %v", err)
		t.debugf("raw response body: %s", string(rawBody))
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
	t.trace("gemini", "output", translationResult.Text, map[string]any{
		"source_lang": translationResult.SourceLang,
	})

	return formatOutput(t.outputFormat, translationResult.SourceLang, targetLang, text, translationResult.Text), nil
}

func extractGeminiOutputText(apiResp geminiGenerateResponse) string {
	for _, candidate := range apiResp.Candidates {
		var b strings.Builder
		for _, part := range candidate.Content.Parts {
			b.WriteString(part.Text)
		}
		if text := b.String(); strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func (t *GeminiTranslator) buildSystemPrompt() string {
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

func (t *GeminiTranslator) hasActiveExternalPrompt() bool {
	t.promptMu.RLock()
	active := t.promptPath != ""
	t.promptMu.RUnlock()
	return active
}

func (t *GeminiTranslator) getSystemPrompt() string {
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

func (t *GeminiTranslator) getPassthroughRules() []passthroughRule {
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

func (t *GeminiTranslator) debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	t.trace("gemini", "debug", msg, nil)

	if !t.debug {
		return
	}
	log.Printf("[DEBUG][Gemini] %s", msg)
}

func (t *GeminiTranslator) trace(engine, stage, message string, fields map[string]any) {
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
