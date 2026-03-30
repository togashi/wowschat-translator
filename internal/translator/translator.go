package translator

import (
	"strings"
)

type Translator interface {
	Translate(text, targetLang string) (string, error)
}

func formatOutput(outputFormat, detectedSourceLanguage, targetLanguage, sourceText, translatedText string) string {
	formatted := strings.ReplaceAll(outputFormat, "{DetectedSourceLanguage}", strings.ToUpper(detectedSourceLanguage))
	formatted = strings.ReplaceAll(formatted, "{TargetLanguage}", targetLanguage)
	formatted = strings.ReplaceAll(formatted, "{SourceText}", sourceText)
	formatted = strings.ReplaceAll(formatted, "{TranslatedText}", translatedText)

	return formatted
}
