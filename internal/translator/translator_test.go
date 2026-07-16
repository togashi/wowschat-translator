package translator

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStrictTranslationSchema(t *testing.T) {
	data, err := json.Marshal(strictTranslationSchema())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	want := `{"additionalProperties":false,"properties":{"source_lang":{"type":"string"},"text":{"type":"string"},"translation_note":{"type":"string"}},"required":["text","source_lang","translation_note"],"type":"object"}`
	if got != want {
		t.Errorf("strict schema = %s, want %s", got, want)
	}
}

func TestGeminiTranslationSchema(t *testing.T) {
	data, err := json.Marshal(geminiTranslationSchema())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	want := `{"properties":{"source_lang":{"type":"string"},"text":{"type":"string"},"translation_note":{"type":"string"}},"propertyOrdering":["text","source_lang","translation_note"],"required":["text","source_lang","translation_note"],"type":"object"}`
	if got != want {
		t.Errorf("gemini schema = %s, want %s", got, want)
	}
}

func TestTemperatureField(t *testing.T) {
	if got := temperatureField(-1); got != nil {
		t.Errorf("negative temperature should be nil, got %v", *got)
	}
	if got := temperatureField(-0.001); got != nil {
		t.Errorf("any negative temperature should be nil, got %v", *got)
	}
	if got := temperatureField(0); got == nil || *got != 0 {
		t.Errorf("zero temperature should serialize as 0, got %v", got)
	}
	if got := temperatureField(0.2); got == nil || *got != 0.2 {
		t.Errorf("positive temperature should serialize as 0.2, got %v", got)
	}
}

func TestGPTRequestOmitsNegativeTemperature(t *testing.T) {
	withTemp, err := json.Marshal(openAIResponsesRequest{Model: "m", Temperature: temperatureField(0.2)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !containsJSONKey(string(withTemp), "temperature") {
		t.Errorf("expected temperature present, got %s", withTemp)
	}

	omitted, err := json.Marshal(openAIResponsesRequest{Model: "m", Temperature: temperatureField(-1)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if containsJSONKey(string(omitted), "temperature") {
		t.Errorf("expected temperature omitted, got %s", omitted)
	}
}

func containsJSONKey(payload, key string) bool {
	return json.Valid([]byte(payload)) && strings.Contains(payload, `"`+key+`":`)
}

func TestParseTranslationResult_Valid(t *testing.T) {
	res, err := parseTranslationResult(`{"text":"キャップAへ","source_lang":"en","translation_note":"cmd"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "キャップAへ" || res.SourceLang != "en" || res.TranslationNote != "cmd" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestParseTranslationResult_Fenced(t *testing.T) {
	res, err := parseTranslationResult("```json\n{\"text\":\"go B\",\"source_lang\":\"ja\"}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "go B" || res.SourceLang != "ja" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestParseTranslationResult_Truncated(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantText   string
		wantSource string
	}{
		{
			name:       "cut before closing brace",
			content:    `{"text":"キャップAへ","source_lang":"en"`,
			wantText:   "キャップAへ",
			wantSource: "en",
		},
		{
			name:       "cut inside second value",
			content:    `{"text":"push mid","source_lang":"e`,
			wantText:   "push mid",
			wantSource: "e",
		},
		{
			name:       "cut inside second key",
			content:    `{"text":"push mid","source_l`,
			wantText:   "push mid",
			wantSource: "",
		},
		{
			name:       "trailing comma then cut",
			content:    `{"text":"go B immediately",`,
			wantText:   "go B immediately",
			wantSource: "",
		},
		{
			name:       "cut inside text value keeps partial",
			content:    `{"text":"キャップAに突`,
			wantText:   "キャップAに突",
			wantSource: "",
		},
		{
			name:       "unfenced truncation",
			content:    "```json\n{\"text\":\"go B\",\"source_lang\":\"j",
			wantText:   "go B",
			wantSource: "j",
		},
		{
			name:       "escaped quote in text",
			content:    `{"text":"say \"hi\" now","source_lang":`,
			wantText:   `say "hi" now`,
			wantSource: "",
		},
		{
			name:       "literal cjk value then cut",
			content:    `{"text":"キャップ`,
			wantText:   "キャップ",
			wantSource: "",
		},
		{
			name:       "unicode escapes then cut",
			content:    `{"text":"キャ","source_lang":"e`,
			wantText:   "キャ",
			wantSource: "e",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := parseTranslationResult(tc.content)
			if err != nil {
				t.Fatalf("expected recovery, got error: %v", err)
			}
			if res.Text != tc.wantText {
				t.Errorf("text = %q, want %q", res.Text, tc.wantText)
			}
			if res.SourceLang != tc.wantSource {
				t.Errorf("source_lang = %q, want %q", res.SourceLang, tc.wantSource)
			}
		})
	}
}

func TestParseTranslationResult_Unrecoverable(t *testing.T) {
	cases := []string{
		``,
		`not json at all`,
		`{"source_lang":"en"`, // truncated, no text field
		`{"text":`,            // text key but no value
	}
	for _, content := range cases {
		if _, err := parseTranslationResult(content); err == nil {
			t.Errorf("expected error for %q, got nil", content)
		}
	}
}
