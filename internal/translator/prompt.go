package translator

import (
	"os"
	"strings"
	"sync"
	"time"
)

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
