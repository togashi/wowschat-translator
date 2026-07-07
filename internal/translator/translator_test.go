package translator

import "testing"

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
