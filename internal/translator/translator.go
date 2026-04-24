package translator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type Translator interface {
	Translate(text, targetLang string) (string, error)
}

type translationResult struct {
	Text            string `json:"text"`
	SourceLang      string `json:"source_lang"`
	TranslationNote string `json:"translation_note,omitempty"`
}

func formatOutput(outputFormat, detectedSourceLanguage, targetLanguage, sourceText, translatedText string) string {
	formatted := strings.ReplaceAll(outputFormat, "{DetectedSourceLanguage}", strings.ToUpper(detectedSourceLanguage))
	formatted = strings.ReplaceAll(formatted, "{TargetLanguage}", targetLanguage)
	formatted = strings.ReplaceAll(formatted, "{SourceText}", sourceText)
	formatted = strings.ReplaceAll(formatted, "{TranslatedText}", translatedText)

	return formatted
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

func parseTranslationResult(content string) (*translationResult, error) {
	var out translationResult
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

	return nil, fmt.Errorf("translation response is not valid JSON: %q", content)
}
