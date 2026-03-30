package translator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Translator struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	outputFormat string
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

// New creates a Translator. API keys ending in ":fx" use the free-tier endpoint.
func New(apiKey, outputFormat string) *Translator {
	baseURL := "https://api.deepl.com"
	if strings.HasSuffix(apiKey, ":fx") {
		baseURL = "https://api-free.deepl.com"
	}
	return &Translator{
		apiKey:  apiKey,
		baseURL: baseURL,
		outputFormat: outputFormat,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Translate translates text to targetLang via DeepL.
// Returns empty string if no translation is needed (same language or unchanged text).
func (t *Translator) Translate(text, targetLang string) (string, error) {
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

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("DeepL request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepL API status %d", resp.StatusCode)
	}

	var result translateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("DeepL response: %w", err)
	}

	if len(result.Translations) == 0 {
		return "", nil
	}

	tr := result.Translations[0]

	// Skip if source language matches target or text is unchanged (same as original)
	if strings.EqualFold(tr.DetectedSourceLanguage, targetLang) || tr.Text == text {
		return "", nil
	}

	formatted := strings.ReplaceAll(t.outputFormat, "{DetectedSourceLanguage}", strings.ToUpper(tr.DetectedSourceLanguage))
	formatted = strings.ReplaceAll(formatted, "{TargetLanguage}", targetLang)
	formatted = strings.ReplaceAll(formatted, "{SourceText}", text)
	formatted = strings.ReplaceAll(formatted, "{TranslatedText}", tr.Text)

	return formatted, nil
}
