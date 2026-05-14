package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/togashi/wowschat-translator/internal/translator"
)

var nonTranslatablePattern = regexp.MustCompile(`^[\p{P}\p{S}\p{N}\s]+$`)

type Server struct {
	tr                           translator.Translator
	targetLang                   string
	listenAddr                   string
	endpointPath                 string
	http                         *http.Server
	duplicateBurstSkipEnabled    bool
	duplicateBurstWindow         time.Duration
	duplicateNormalizeWhitespace bool
	dupMu                        sync.Mutex
	duplicateLastSeen            map[string]time.Time
}

func New(
	tr translator.Translator,
	targetLang string,
	listenPort int,
	endpointPath string,
	duplicateBurstSkipEnabled bool,
	duplicateBurstWindowMs int,
	duplicateNormalizeWhitespace bool,
) *Server {
	listenAddr := fmt.Sprintf("127.0.0.1:%d", listenPort)
	if duplicateBurstWindowMs < 0 {
		duplicateBurstWindowMs = 0
	}
	s := &Server{
		tr:                           tr,
		targetLang:                   targetLang,
		listenAddr:                   listenAddr,
		endpointPath:                 endpointPath,
		duplicateBurstSkipEnabled:    duplicateBurstSkipEnabled,
		duplicateBurstWindow:         time.Duration(duplicateBurstWindowMs) * time.Millisecond,
		duplicateNormalizeWhitespace: duplicateNormalizeWhitespace,
		duplicateLastSeen:            make(map[string]time.Time),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(endpointPath, s.handle)
	s.http = &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	return s
}

// Start begins serving. Blocks until the server is closed.
func (s *Server) Start() error {
	log.Printf("listening on http://%s%s", s.listenAddr, s.endpointPath)
	if err := s.http.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	if text == "" {
		return
	}

	log.Printf("received: %q", text)

	if nonTranslatablePattern.MatchString(text) {
		log.Printf("skip: non-translatable text (symbols/numbers only)")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		return
	}

	if s.shouldSkipDuplicateBurst(text) {
		log.Printf("skip: duplicate burst message")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		return
	}

	result, err := s.tr.Translate(text, s.targetLang)
	if err != nil {
		log.Printf("translation error: %v", err)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		return
	}

	if result == "" {
		log.Printf("skip: no translation needed")
	} else {
		log.Printf("translated: %q", result)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(result))
}

func (s *Server) shouldSkipDuplicateBurst(text string) bool {
	if !s.duplicateBurstSkipEnabled || s.duplicateBurstWindow <= 0 {
		return false
	}

	key := text
	if s.duplicateNormalizeWhitespace {
		key = normalizeWhitespace(text)
	}
	if key == "" {
		return false
	}

	now := time.Now()
	s.dupMu.Lock()
	defer s.dupMu.Unlock()

	if last, ok := s.duplicateLastSeen[key]; ok {
		if now.Sub(last) <= s.duplicateBurstWindow {
			s.duplicateLastSeen[key] = now
			return true
		}
	}
	s.duplicateLastSeen[key] = now

	if len(s.duplicateLastSeen) > 2048 {
		expireBefore := now.Add(-2 * s.duplicateBurstWindow)
		for k, t := range s.duplicateLastSeen {
			if t.Before(expireBefore) {
				delete(s.duplicateLastSeen, k)
			}
		}
	}

	return false
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
