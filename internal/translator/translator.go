package translator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
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

// temperatureField converts a configured temperature into a pointer suitable
// for JSON serialization. A negative value is a sentinel meaning "do not send
// temperature" (returns nil so the field is omitted), which lets callers target
// models that reject an explicit temperature.
func temperatureField(temperature float64) *float64 {
	if temperature < 0 {
		return nil
	}
	value := temperature
	return &value
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

// translationSchemaProperties describes the fields of the translation result
// object. It is shared by every engine's native structured-output request so
// the model is constrained to emit valid, parseable JSON.
func translationSchemaProperties() map[string]any {
	return map[string]any{
		"text":             map[string]any{"type": "string"},
		"source_lang":      map[string]any{"type": "string"},
		"translation_note": map[string]any{"type": "string"},
	}
}

// translationSchemaFields is the ordered list of result fields, used for both
// the "required" set and Gemini's property ordering.
var translationSchemaFields = []string{"text", "source_lang", "translation_note"}

// strictTranslationSchema builds a JSON Schema for OpenAI and Anthropic
// structured outputs: every field required, no extra properties.
func strictTranslationSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           translationSchemaProperties(),
		"required":             translationSchemaFields,
		"additionalProperties": false,
	}
}

// geminiTranslationSchema builds the response schema for Gemini, whose OpenAPI
// subset uses propertyOrdering and does not accept additionalProperties.
func geminiTranslationSchema() map[string]any {
	return map[string]any{
		"type":             "object",
		"properties":       translationSchemaProperties(),
		"required":         translationSchemaFields,
		"propertyOrdering": translationSchemaFields,
	}
}

func parseTranslationResult(content string) (*translationResult, error) {
	content = stripMarkdownFence(content)
	var out translationResult
	if err := json.Unmarshal([]byte(content), &out); err == nil {
		return &out, nil
	}

	// The response may be a well-formed JSON object that was cut off partway
	// through (e.g. the model hit its output-token limit). Recover whatever
	// complete fields precede the truncation point, plus a partially-received
	// final string value, so the translation is not thrown away entirely.
	if res, ok := recoverTruncatedTranslation(content); ok {
		return res, nil
	}

	return nil, fmt.Errorf("translation response is not valid JSON: %q", content)
}

// recoverTruncatedTranslation salvages fields from a translation JSON object
// that was truncated mid-stream. It reads the flat "key": "value" pairs of the
// leading object, tolerating an unterminated final key or value and missing
// closing braces. It succeeds only if a non-empty text field is recovered.
func recoverTruncatedTranslation(content string) (*translationResult, bool) {
	start := strings.IndexByte(content, '{')
	if start < 0 {
		return nil, false
	}

	s := &jsonScanner{src: content, pos: start + 1}
	fields := map[string]string{}

	for {
		s.skipSpaceAndCommas()
		if s.done() || s.peek() == '}' {
			break
		}
		if s.peek() != '"' {
			break // unexpected token; stop reading further pairs
		}

		key, complete := s.readString()
		if !complete {
			break // key itself was truncated; nothing usable follows
		}

		s.skipSpace()
		if s.done() || s.peek() != ':' {
			break // key without a colon (truncated); drop it
		}
		s.pos++ // consume ':'

		s.skipSpace()
		if s.done() {
			break // colon without a value (truncated); drop the key
		}
		if s.peek() != '"' {
			s.skipPrimitive() // non-string value; ignore for our schema
			continue
		}

		value, complete := s.readString()
		if _, seen := fields[key]; !seen {
			fields[key] = value
		}
		if !complete {
			break // value was cut off; keep the partial text we have
		}
	}

	if strings.TrimSpace(fields["text"]) == "" {
		return nil, false
	}
	return &translationResult{
		Text:            fields["text"],
		SourceLang:      fields["source_lang"],
		TranslationNote: fields["translation_note"],
	}, true
}

// jsonScanner is a minimal, fault-tolerant reader over a JSON fragment used to
// recover data from truncated responses. Unlike encoding/json it never fails on
// an unexpected EOF; it simply reports the value read so far as incomplete.
type jsonScanner struct {
	src string
	pos int
}

func (s *jsonScanner) done() bool { return s.pos >= len(s.src) }

func (s *jsonScanner) peek() byte { return s.src[s.pos] }

func (s *jsonScanner) skipSpace() {
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return
		}
	}
}

func (s *jsonScanner) skipSpaceAndCommas() {
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case ' ', '\t', '\n', '\r', ',':
			s.pos++
		default:
			return
		}
	}
}

// skipPrimitive advances past a non-string value (number, bool, null) until the
// next comma, object close, or EOF.
func (s *jsonScanner) skipPrimitive() {
	for s.pos < len(s.src) {
		if c := s.src[s.pos]; c == ',' || c == '}' {
			return
		}
		s.pos++
	}
}

// readString consumes a double-quoted string starting at the current position
// (which must be '"') and returns the decoded value along with whether the
// closing quote was found. A false result means the string was truncated.
func (s *jsonScanner) readString() (string, bool) {
	s.pos++ // consume opening quote
	var b strings.Builder
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch c {
		case '"':
			s.pos++
			return b.String(), true
		case '\\':
			r, ok := s.readEscape()
			if !ok {
				return b.String(), false // truncated escape sequence
			}
			b.WriteRune(r)
		default:
			b.WriteByte(c)
			s.pos++
		}
	}
	return b.String(), false // reached EOF before the closing quote
}

// readEscape decodes a backslash escape at the current position, including
// \uXXXX (with surrogate-pair combining). It returns false if the sequence is
// truncated by EOF.
func (s *jsonScanner) readEscape() (rune, bool) {
	if s.pos+1 >= len(s.src) {
		return 0, false
	}
	esc := s.src[s.pos+1]
	switch esc {
	case '"':
		s.pos += 2
		return '"', true
	case '\\':
		s.pos += 2
		return '\\', true
	case '/':
		s.pos += 2
		return '/', true
	case 'n':
		s.pos += 2
		return '\n', true
	case 't':
		s.pos += 2
		return '\t', true
	case 'r':
		s.pos += 2
		return '\r', true
	case 'b':
		s.pos += 2
		return '\b', true
	case 'f':
		s.pos += 2
		return '\f', true
	case 'u':
		r, ok := s.readUnicodeEscape()
		if !ok {
			return 0, false
		}
		return r, true
	default:
		s.pos += 2
		return rune(esc), true
	}
}

func (s *jsonScanner) readUnicodeEscape() (rune, bool) {
	if s.pos+6 > len(s.src) {
		s.pos = len(s.src)
		return 0, false // truncated \uXXXX
	}
	hi, err := strconv.ParseUint(s.src[s.pos+2:s.pos+6], 16, 32)
	if err != nil {
		s.pos += 2 // skip "\u" and treat the rest as literal text
		return 'u', true
	}
	s.pos += 6
	r := rune(hi)
	if utf16.IsSurrogate(r) && s.pos+6 <= len(s.src) && s.src[s.pos] == '\\' && s.src[s.pos+1] == 'u' {
		if lo, err := strconv.ParseUint(s.src[s.pos+2:s.pos+6], 16, 32); err == nil {
			if combined := utf16.DecodeRune(r, rune(lo)); combined != '�' {
				s.pos += 6
				return combined, true
			}
		}
	}
	return r, true
}
