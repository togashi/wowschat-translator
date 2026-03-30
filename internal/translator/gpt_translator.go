package translator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type GPTTranslator struct {
	apiKey       string
	baseURL      string
	model        string
	temperature  float64
	httpClient   *http.Client
	outputFormat string
	passthrough  []string
	glossary     map[string]string
	debug        bool
	traceSink    func(TranslatorTraceEvent)
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

// NewGPTTranslator creates a GPTTranslator.
func NewGPTTranslator(
	apiKey,
	model string,
	temperature float64,
	outputFormat string,
	passthrough []string,
	glossary map[string]string,
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
		temperature:  temperature,
		outputFormat: outputFormat,
		passthrough:  passthrough,
		glossary:     glossary,
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

const gptTranslationSystemPrompt = `You are a translation helper for casual in-game chat.

Translate into the target language when the meaning is clear and useful.

Rules:
- Assume the text is from a multiplayer game context
- Keep common gaming slang unchanged (gg, wp, lol, afk, brb)
- If the meaning is clear, translate even short phrases
- Treat short commands and gameplay instructions as meaningful and translate them (e.g., "cap A", "push mid", "go B immediately")
- If the text is ambiguous or unclear outside a game context, keep it unchanged

Also detect the source language.

Output JSON with:
- text: final output text
- source_lang: language code like "zh", "en", "ko", "id", "ja"
- translation_note: optional note explaining translation choices (for debugging, not required)`

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

	rules := buildPassthroughRules(t.passthrough)
	maskedText, segments := applyPassthroughRules(text, rules)
	t.debugf("passthrough rules=%d masked_segments=%d", len(rules), len(segments))

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
	prompt := gptTranslationSystemPrompt

	if len(t.passthrough) > 0 {
		prompt += "\n\nPassthrough words/phrases (keep as-is):\n"
		for _, s := range t.passthrough {
			if strings.TrimSpace(s) == "" {
				continue
			}
			prompt += "- " + s + "\n"
		}
	}

	if len(t.glossary) > 0 {
		prompt += "\nGlossary (source -> preferred target):\n"
		for src, dst := range t.glossary {
			if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
				continue
			}
			prompt += "- " + src + " -> " + dst + "\n"
		}
	}

	return prompt
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

func buildPassthroughRules(items []string) []passthroughRule {
	rules := make([]passthroughRule, 0, len(items))
	for _, raw := range items {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		switch {
		case strings.HasPrefix(raw, "regex:"):
			pattern := strings.TrimSpace(strings.TrimPrefix(raw, "regex:"))
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			rules = append(rules, passthroughRule{kind: "regex", value: pattern, pattern: re})
		case strings.HasPrefix(raw, "prefix:"):
			value := strings.TrimSpace(strings.TrimPrefix(raw, "prefix:"))
			if value == "" {
				continue
			}
			rules = append(rules, passthroughRule{kind: "prefix", value: value})
		case strings.HasPrefix(raw, "contains:"):
			value := strings.TrimSpace(strings.TrimPrefix(raw, "contains:"))
			if value == "" {
				continue
			}
			rules = append(rules, passthroughRule{kind: "contains", value: value})
		default:
			rules = append(rules, passthroughRule{kind: "exact", value: raw})
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
		case "exact", "contains":
			if rule.value == "" || !strings.Contains(maskedText, rule.value) {
				continue
			}
			placeholder := mask(rule.value)
			maskedText = strings.ReplaceAll(maskedText, rule.value, placeholder)
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
