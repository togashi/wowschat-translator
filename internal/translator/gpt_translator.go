package translator

import (
	"fmt"
	"net/http"
	"time"
)

type GPTTranslator struct {
	apiKey       string
	baseURL      string
	model        string
	httpClient   *http.Client
	outputFormat string
}

// NewGPTTranslator creates a GPTTranslator.
func NewGPTTranslator(apiKey, outputFormat string) *GPTTranslator {
	return &GPTTranslator{
		apiKey:       apiKey,
		baseURL:      "https://api.openai.com/v1",
		model:        "gpt-4o-mini",
		outputFormat: outputFormat,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Translate is a placeholder for a future GPT-based translation implementation.
func (t *GPTTranslator) Translate(text, targetLang string) (string, error) {
	return "", fmt.Errorf("GPT translator is not implemented yet")
}
