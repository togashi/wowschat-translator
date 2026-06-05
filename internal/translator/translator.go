package translator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var ptTokenRe = regexp.MustCompile(`__PT\d+__`)

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

func sanitizePTTokens(text string) string {
	return ptTokenRe.ReplaceAllString(text, "")
}

func buildUserMessage(targetLang, maskedText string) string {
	return fmt.Sprintf("Target language: %s\n<chat_message>\n%s\n</chat_message>", targetLang, maskedText)
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

func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	newline := strings.IndexByte(s, '\n')
	if newline < 0 {
		return s
	}
	s = s[newline+1:]
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}

func parseTranslationResult(content string) (*translationResult, error) {
	content = stripMarkdownFence(content)
	var out translationResult
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("translation response is not valid JSON: %q", content)
	}
	return &out, nil
}
