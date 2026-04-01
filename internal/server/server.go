package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/togashi/wowschat-translator/internal/translator"
)

var nonTranslatablePattern = regexp.MustCompile(`^[\p{P}\p{S}\p{N}\s]+$`)

type Server struct {
	tr           translator.Translator
	targetLang   string
	listenAddr   string
	endpointPath string
	http         *http.Server
}

func New(tr translator.Translator, targetLang string, listenPort int, endpointPath string) *Server {
	listenAddr := fmt.Sprintf("127.0.0.1:%d", listenPort)
	s := &Server{
		tr:           tr,
		targetLang:   targetLang,
		listenAddr:   listenAddr,
		endpointPath: endpointPath,
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
