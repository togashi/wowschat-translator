package translator

import (
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	promptPlaceholderPassthrough = "{{PASSTHROUGH}}"
	promptPlaceholderGlossary    = "{{GLOSSARY}}"
)

func buildPassthroughPromptBlock(items []string) string {
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\nPassthrough words/phrases (keep as-is):\n")
	count := 0
	for _, s := range items {
		if strings.TrimSpace(s) == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteString("\n")
		count++
	}

	if count == 0 {
		return ""
	}
	return b.String()
}

func buildGlossaryPromptBlock(glossary map[string]string) string {
	if len(glossary) == 0 {
		return ""
	}

	keys := make([]string, 0, len(glossary))
	for src := range glossary {
		keys = append(keys, src)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("\nGlossary (source -> preferred target):\n")
	count := 0
	for _, src := range keys {
		dst := glossary[src]
		if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(src)
		b.WriteString(" -> ")
		b.WriteString(dst)
		b.WriteString("\n")
		count++
	}

	if count == 0 {
		return ""
	}
	return b.String()
}

// getSystemPromptFromFileOrDefault loads a system prompt from an external file
// (with modification-time caching) or falls back to the embedded default.
// The caller passes pointers to its own cache fields so the logic is shared
// between GPTTranslator and ClaudeTranslator without duplicating code.
func getSystemPromptFromFileOrDefault(
	promptFile string,
	embeddedPrompt string,
	mu *sync.RWMutex,
	cached *bool,
	value *string,
	path *string,
	modTime *time.Time,
	debugf func(string, ...any),
) string {
	defaultPrompt := strings.TrimSpace(embeddedPrompt)
	if defaultPrompt == "" {
		defaultPrompt = "You are a translation helper for casual in-game chat."
	}

	promptFile = strings.TrimSpace(promptFile)
	if promptFile == "" {
		mu.RLock()
		if *cached {
			v := *value
			mu.RUnlock()
			return v
		}
		mu.RUnlock()

		mu.Lock()
		if !*cached {
			*value = defaultPrompt
			*cached = true
			*path = ""
			*modTime = time.Time{}
		}
		v := *value
		mu.Unlock()
		return v
	}

	info, err := os.Stat(promptFile)
	if err == nil && !info.IsDir() {
		mt := info.ModTime()

		mu.RLock()
		if *cached && *path == promptFile && (*modTime).Equal(mt) {
			v := *value
			mu.RUnlock()
			return v
		}
		mu.RUnlock()

		if data, readErr := os.ReadFile(promptFile); readErr == nil {
			if loaded := strings.TrimSpace(string(data)); loaded != "" {
				mu.Lock()
				*value = loaded
				*cached = true
				*path = promptFile
				*modTime = mt
				mu.Unlock()
				debugf("loaded prompt file: %s", promptFile)
				return loaded
			}
			debugf("prompt file is empty, using default: %s", promptFile)
		} else {
			debugf("prompt file load failed, using default: %s (%v)", promptFile, readErr)
		}
	} else if err != nil {
		debugf("prompt file load failed, using default: %s (%v)", promptFile, err)
	}

	mu.RLock()
	if *cached {
		v := *value
		mu.RUnlock()
		return v
	}
	mu.RUnlock()

	mu.Lock()
	if !*cached {
		*value = defaultPrompt
		*cached = true
		*path = ""
		*modTime = time.Time{}
	}
	v := *value
	mu.Unlock()

	return v
}
