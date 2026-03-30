package translator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type DeepLTranslator struct {
	apiKey       string
	baseURL      string
	httpClient   *http.Client
	outputFormat string
	debug        bool
	traceSink    func(TranslatorTraceEvent)
}

type translateRequest struct {
	Text       []string `json:"text"`
	TargetLang string   `json:"target_lang"`
}

type translateResponse struct {
	Translations []struct {
		DetectedSourceLanguage string `json:"detected_source_language"`
		Text                   string `json:"text"`
	} `json:"translations"`
}

// NewDeepLTranslator creates a DeepLTranslator. API keys ending in ":fx" use the free-tier endpoint.
func NewDeepLTranslator(apiKey, outputFormat string, debug bool) *DeepLTranslator {
	baseURL := "https://api.deepl.com"
	if strings.HasSuffix(apiKey, ":fx") {
		baseURL = "https://api-free.deepl.com"
	}
	return &DeepLTranslator{
		apiKey:       apiKey,
		baseURL:      baseURL,
		outputFormat: outputFormat,
		debug:        debug,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetTraceSink sets an optional hook for collecting translator trace events.
func (t *DeepLTranslator) SetTraceSink(sink func(TranslatorTraceEvent)) {
	t.traceSink = sink
}

// Translate translates text to targetLang via DeepL.
// Returns empty string if no translation is needed (same language or unchanged text).
func (t *DeepLTranslator) Translate(text, targetLang string) (string, error) {
	totalStart := time.Now()
	var requestElapsed time.Duration
	var statusCode int
	defer func() {
		t.debugf("timing request_ms=%d total_ms=%d status=%d", requestElapsed.Milliseconds(), time.Since(totalStart).Milliseconds(), statusCode)
		t.trace("deepl", "timing", "request finished", map[string]any{
			"request_ms": requestElapsed.Milliseconds(),
			"total_ms":   time.Since(totalStart).Milliseconds(),
			"status":     statusCode,
		})
	}()

	t.debugf("translate start target=%s text_len=%d", targetLang, len(text))

	body, err := json.Marshal(translateRequest{
		Text:       []string{text},
		TargetLang: targetLang,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, t.baseURL+"/v2/translate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "DeepL-Auth-Key "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")

	requestStart := time.Now()
	resp, err := t.httpClient.Do(req)
	requestElapsed = time.Since(requestStart)
	if err != nil {
		return "", fmt.Errorf("DeepL request: %w", err)
	}
	defer resp.Body.Close()
	statusCode = resp.StatusCode
	t.debugf("deepl status=%d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepL API status %d", resp.StatusCode)
	}

	var result translateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("DeepL response: %w", err)
	}

	if len(result.Translations) == 0 {
		t.debugf("deepl no translations")
		return "", nil
	}

	tr := result.Translations[0]

	// Skip if source language matches target or text is unchanged (same as original)
	if strings.EqualFold(tr.DetectedSourceLanguage, targetLang) || tr.Text == text {
		t.debugf("skip translation source=%s target=%s unchanged=%t", tr.DetectedSourceLanguage, targetLang, tr.Text == text)
		return "", nil
	}
	t.debugf("translated source=%s translated_len=%d", tr.DetectedSourceLanguage, len(tr.Text))

	return formatOutput(t.outputFormat, tr.DetectedSourceLanguage, targetLang, text, tr.Text), nil
}

func (t *DeepLTranslator) debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	t.trace("deepl", "debug", msg, nil)

	if !t.debug {
		return
	}
	log.Printf("[DEBUG][DeepL] %s", msg)
}

func (t *DeepLTranslator) trace(engine, stage, message string, fields map[string]any) {
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
